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

// Ambang Batas Default (Threshold Fallback):
// 1. Suhu Heating Under (< 110.0) & Over (> 123.0)
// 2. Suhu Holding Under (< 110.0) & Over (> 123.0)
// 3. Flowrate Over (> 7700)
const (
	DefaultLimitTempMin     = 110.0
	DefaultLimitTempMax     = 123.0
	DefaultLimitFlowrateMax = 7700.0
)

var (
	LimitTempMin     = DefaultLimitTempMin
	LimitTempMax     = DefaultLimitTempMax
	LimitFlowrateMax = DefaultLimitFlowrateMax

	thresholdMu        sync.RWMutex
	cachedMinTemp      = DefaultLimitTempMin
	cachedMaxTemp      = DefaultLimitTempMax
	lastThresholdCheck time.Time
	thresholdCacheTTL  = 3 * time.Second
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

// GetLatestTempThresholdsPasteur1 mengambil ambang batas min dan max suhu terbaru dari master_data_minmaxpasteur1.
// Nilai dari tabel dibagi 10 baru menjadi ambang batas suhu (misal: 1230 -> 123.0, 1100 -> 110.0).
func GetLatestTempThresholdsPasteur1() (float64, float64) {
	thresholdMu.RLock()
	if time.Since(lastThresholdCheck) < thresholdCacheTTL && !lastThresholdCheck.IsZero() {
		minT := cachedMinTemp
		maxT := cachedMaxTemp
		thresholdMu.RUnlock()
		return minT, maxT
	}
	thresholdMu.RUnlock()

	return RefreshTempThresholdsPasteur1()
}

// RefreshTempThresholdsPasteur1 memaksa query ke database untuk memperbarui ambang batas suhu terbaru
func RefreshTempThresholdsPasteur1() (float64, float64) {
	thresholdMu.Lock()
	defer thresholdMu.Unlock()

	minTemp := DefaultLimitTempMin
	maxTemp := DefaultLimitTempMax

	if config.DB != nil {
		var rows []models.MasterDataMinMaxPasteur1
		// Ambil data terbaru diurutkan dari id DESC / created_at DESC
		if err := config.DB.Where("parameter_name IN ?", []string{"MinSuhuPasteur", "MaxSuhuPasteur"}).
			Order("id DESC").
			Limit(50).
			Find(&rows).Error; err == nil && len(rows) > 0 {

			foundMin := false
			foundMax := false

			for _, r := range rows {
				name := strings.TrimSpace(r.ParameterName)
				if !foundMax && strings.EqualFold(name, "MaxSuhuPasteur") && r.Value > 0 {
					maxTemp = r.Value / 10.0
					foundMax = true
				}
				if !foundMin && strings.EqualFold(name, "MinSuhuPasteur") && r.Value > 0 {
					minTemp = r.Value / 10.0
					foundMin = true
				}
				if foundMin && foundMax {
					break
				}
			}
		}
	}

	cachedMinTemp = minTemp
	cachedMaxTemp = maxTemp
	LimitTempMin = minTemp
	LimitTempMax = maxTemp
	lastThresholdCheck = time.Now()

	return minTemp, maxTemp
}

// CheckDivertTriggers memeriksa apakah suatu data sensor memenuhi kriteria divert dengan ambang batas terbaru
func CheckDivertTriggers(s *models.SensorPasteurisasi) (bool, []string, string) {
	minTemp, maxTemp := GetLatestTempThresholdsPasteur1()
	return CheckDivertTriggersWithLimits(s, minTemp, maxTemp, LimitFlowrateMax)
}

// CheckDivertTriggersWithLimits memeriksa apakah suatu data sensor memenuhi kriteria divert berdasarkan parameter ambang batas
func CheckDivertTriggersWithLimits(s *models.SensorPasteurisasi, minTemp, maxTemp, maxFlowrate float64) (bool, []string, string) {
	var triggers []string
	var details []string

	// 1. Suhu Heating Under (< minTemp) - hanya jika > 0 untuk menghindari sensor offline/unpowered
	if s.SuhuHeating > 0 && s.SuhuHeating < minTemp {
		triggers = append(triggers, "HEATING_UNDER")
		details = append(details, fmt.Sprintf("Suhu Heating under: %.1f (min %.1f)", s.SuhuHeating, minTemp))
	} else if s.SuhuHeating > maxTemp {
		triggers = append(triggers, "HEATING_OVER")
		details = append(details, fmt.Sprintf("Suhu Heating over: %.1f (max %.1f)", s.SuhuHeating, maxTemp))
	}

	// 2. Suhu Holding Under (< minTemp) & Over (> maxTemp)
	if s.SuhuHolding > 0 && s.SuhuHolding < minTemp {
		triggers = append(triggers, "HOLDING_UNDER")
		details = append(details, fmt.Sprintf("Suhu Holding under: %.1f (min %.1f)", s.SuhuHolding, minTemp))
	} else if s.SuhuHolding > maxTemp {
		triggers = append(triggers, "HOLDING_OVER")
		details = append(details, fmt.Sprintf("Suhu Holding over: %.1f (max %.1f)", s.SuhuHolding, maxTemp))
	}

	// 3. Flowrate Over (> maxFlowrate)
	if s.Flowrate > maxFlowrate {
		triggers = append(triggers, "FLOWRATE_OVER")
		details = append(details, fmt.Sprintf("Flowrate over: %.0f (max %.0f)", s.Flowrate, maxFlowrate))
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
				CreatedAt:   latest.Waktu,
				UpdatedAt:   latest.Waktu,
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
						"updated_at":    endTime,
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
						"updated_at":    endTime,
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
