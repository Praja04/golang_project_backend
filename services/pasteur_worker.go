package services

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"backend-golang/config"
	"backend-golang/models"
)

// Ambang Batas (Threshold) Sesuai Spesifikasi:
// 1. Suhu Heating Under (< 1110) & Over (> 1230)
// 2. Suhu Holding Under (< 1110) & Over (> 1230)
// 3. Flowrate Over (> 7500)
const (
	LimitTempMin     = 1110.0
	LimitTempMax     = 1230.0
	LimitFlowrateMax = 7500.0
)

var (
	jakartaLoc *time.Location
	once       sync.Once
)

func initLoc() {
	once.Do(func() {
		var err error
		jakartaLoc, err = time.LoadLocation("Asia/Jakarta")
		if err != nil {
			jakartaLoc = time.FixedZone("WIB", 7*60*60)
		}
	})
}

// CheckDivertTriggers memeriksa apakah suatu data sensor memenuhi kriteria divert
func CheckDivertTriggers(s *models.SensorPasteurisasi) (bool, []string, string) {
	var triggers []string
	var details []string

	// 1. Suhu Heating Under (< 1110) - hanya jika > 0 untuk menghindari sensor offline/unpowered
	if s.SuhuHeating > 0 && s.SuhuHeating < LimitTempMin {
		triggers = append(triggers, "HEATING_UNDER")
		details = append(details, fmt.Sprintf("Suhu Heating under: %.0f (min %.0f)", s.SuhuHeating, LimitTempMin))
	} else if s.SuhuHeating > LimitTempMax {
		triggers = append(triggers, "HEATING_OVER")
		details = append(details, fmt.Sprintf("Suhu Heating over: %.0f (max %.0f)", s.SuhuHeating, LimitTempMax))
	}

	// 2. Suhu Holding Under (< 1110) & Over (> 1230)
	if s.SuhuHolding > 0 && s.SuhuHolding < LimitTempMin {
		triggers = append(triggers, "HOLDING_UNDER")
		details = append(details, fmt.Sprintf("Suhu Holding under: %.0f (min %.0f)", s.SuhuHolding, LimitTempMin))
	} else if s.SuhuHolding > LimitTempMax {
		triggers = append(triggers, "HOLDING_OVER")
		details = append(details, fmt.Sprintf("Suhu Holding over: %.0f (max %.0f)", s.SuhuHolding, LimitTempMax))
	}

	// 3. Flowrate Over (> 7500)
	if s.Flowrate > LimitFlowrateMax {
		triggers = append(triggers, "FLOWRATE_OVER")
		details = append(details, fmt.Sprintf("Flowrate over: %.0f (max %.0f)", s.Flowrate, LimitFlowrateMax))
	}

	isDivert := len(triggers) > 0
	keterangan := strings.Join(details, ", ")
	if !isDivert {
		keterangan = "Normal"
	}

	return isDivert, triggers, keterangan
}

// PasteurWorker mengelola background worker pemantauan divert realtime
type PasteurWorker struct {
	mu                      sync.RWMutex
	running                 bool
	stopChan                chan struct{}
	inDivertState           bool
	activeEventID           uint
	activeEventStart        time.Time
	lastProcessedSensorTime time.Time
}

var GlobalPasteurWorker = &PasteurWorker{
	stopChan: make(chan struct{}),
}

// StartPasteurWorker menjalankan background goroutine untuk memantau data sensor secara terus menerus
func StartPasteurWorker(interval time.Duration) {
	initLoc()
	w := GlobalPasteurWorker

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	// Inisialisasi status jika ada event ACTIVE yang belum ditutup
	w.initActiveState()

	log.Printf("[Pasteur 1 Worker] Background worker started (interval: %v)\n", interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.ProcessCurrentSensor()
			case <-w.stopChan:
				log.Println("[Pasteur 1 Worker] Background worker stopped")
				return
			}
		}
	}()
}

// StopPasteurWorker menghentikan background worker
func StopPasteurWorker() {
	w := GlobalPasteurWorker
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}
	close(w.stopChan)
	w.running = false
}

// initActiveState memulihkan event divert yang masih berstatus ACTIVE dari database jika server restart
func (w *PasteurWorker) initActiveState() {
	if config.DB == nil {
		return
	}

	var active models.DivertLogPasteur1
	if err := config.DB.Where("status = ?", "ACTIVE").Order("waktu_mulai DESC").First(&active).Error; err == nil {
		w.mu.Lock()
		w.inDivertState = true
		w.activeEventID = active.ID
		w.activeEventStart = active.WaktuMulai
		w.mu.Unlock()
		log.Printf("[Pasteur 1 Worker] Menemukan event divert aktif (ID: %d) sejak %v, melanjutkan pemantauan.\n", active.ID, active.WaktuMulai)
	}
}

// ProcessCurrentSensor memindai data sensor terbaru dan mengelola state divert
func (w *PasteurWorker) ProcessCurrentSensor() {
	if config.DB == nil {
		return
	}

	var latest models.SensorPasteurisasi
	if err := config.DB.Order("Waktu DESC").First(&latest).Error; err != nil {
		return // DB error atau tabel kosong
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Lewati jika timestamp sensor belum berubah
	if latest.Waktu.Equal(w.lastProcessedSensorTime) {
		return
	}
	w.lastProcessedSensorTime = latest.Waktu

	isDivert, triggers, keterangan := CheckDivertTriggers(&latest)

	now := time.Now().In(jakartaLoc)

	if isDivert {
		if !w.inDivertState {
			// TRANSISI: NORMAL -> DIVERT (Event baru dimulai)
			w.inDivertState = true
			w.activeEventStart = latest.Waktu

			triggerStr := strings.Join(triggers, ",")
			event := models.DivertLogPasteur1{
				WaktuMulai:  latest.Waktu,
				DurasiDetik: 0,
				SuhuHeating: latest.SuhuHeating,
				SuhuHolding: latest.SuhuHolding,
				Flowrate:    latest.Flowrate,
				Pemicu:      triggerStr,
				Keterangan:  keterangan,
				Status:      "ACTIVE",
				Count:       1,
				CreatedAt:   now,
				UpdatedAt:   now,
			}

			if err := config.DB.Create(&event).Error; err == nil {
				w.activeEventID = event.ID
				log.Printf("[Pasteur 1 Worker] *** DIVERT TERDETEKSI *** Waktu: %v | Pemicu: %s | ID Log: %d\n",
					latest.Waktu.Format("2006-01-02 15:04:05"), keterangan, event.ID)
			}
		} else {
			// SEDANG DALAM DIVERT: Update durasi berjalan
			if w.activeEventID > 0 {
				durasi := int64(latest.Waktu.Sub(w.activeEventStart).Seconds())
				if durasi < 0 {
					durasi = 0
				}
				endTime := latest.Waktu

				config.DB.Model(&models.DivertLogPasteur1{}).
					Where("id = ?", w.activeEventID).
					Updates(map[string]interface{}{
						"waktu_selesai": endTime,
						"durasi_detik":  durasi,
						"updated_at":    now,
					})
			}
		}
	} else {
		if w.inDivertState {
			// TRANSISI: DIVERT -> NORMAL (Event selesai)
			w.inDivertState = false

			if w.activeEventID > 0 {
				durasi := int64(latest.Waktu.Sub(w.activeEventStart).Seconds())
				if durasi < 0 {
					durasi = 0
				}
				endTime := latest.Waktu

				config.DB.Model(&models.DivertLogPasteur1{}).
					Where("id = ?", w.activeEventID).
					Updates(map[string]interface{}{
						"waktu_selesai": endTime,
						"durasi_detik":  durasi,
						"status":        "CLOSED",
						"updated_at":    now,
					})

				log.Printf("[Pasteur 1 Worker] *** DIVERT SELESAI *** Selesai: %v | Total Durasi: %d detik | ID: %d\n",
					latest.Waktu.Format("2006-01-02 15:04:05"), durasi, w.activeEventID)

				w.activeEventID = 0
			}
		}
	}
}

// GetWorkerStatus mengembalikan status in-memory worker saat ini
func (w *PasteurWorker) GetWorkerStatus() (bool, uint, time.Time, time.Time) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.inDivertState, w.activeEventID, w.activeEventStart, w.lastProcessedSensorTime
}
