package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend-golang/config"
	"backend-golang/models"
	"backend-golang/services"

	"github.com/gin-gonic/gin"
)

// Jakarta timezone
var jakartaLoc *time.Location

func init() {
	var err error
	jakartaLoc, err = time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Fallback ke UTC+7 jika gagal load
		jakartaLoc = time.FixedZone("WIB", 7*60*60)
	}
}

// Response structure untuk average data
type AvgResponse struct {
	Timestamp string  `json:"timestamp"`
	Average   float64 `json:"average"`
}

// GetLatestPasteurData -> ambil data terbaru
func GetLatestPasteurData(c *gin.Context) {
	var data models.SensorPasteurisasi

	if err := config.DB.Order("Waktu desc").First(&data).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Data pasteurisasi tidak ditemukan",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// GetPasteurDataPerHour -> ambil data per jam (tepat menit:detik = 00:00)
func GetPasteurDataPerHour(c *gin.Context) {
	tanggal := c.Query("tanggal") // format: YYYY-MM-DD
	if tanggal == "" {
		tanggal = time.Now().In(jakartaLoc).Format("2006-01-02")
	}

	var data []models.SensorPasteurisasi
	if err := config.DB.
		Where("DATE(Waktu) = ?", tanggal).
		Where("MINUTE(Waktu) = 0 AND SECOND(Waktu) = 0").
		Order("Waktu asc").
		Find(&data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil data",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"tanggal": tanggal,
		"count":   len(data),
		"data":    data,
	})
}

// GetPasteurAbnormal -> ambil periode suhu heating/holding di luar batas
func GetPasteurAbnormal(c *gin.Context) {
	tanggal := c.Query("tanggal") // YYYY-MM-DD
	if tanggal == "" {
		tanggal = time.Now().In(jakartaLoc).Format("2006-01-02")
	}

	// hasil query mentah
	type rawPeriod struct {
		Start      time.Time `json:"start"`
		End        time.Time `json:"end"`
		MinHeating float64   `json:"min_heating"`
		MaxHeating float64   `json:"max_heating"`
		MinHolding float64   `json:"min_holding"`
		MaxHolding float64   `json:"max_holding"`
	}

	// hasil akhir yang dikirim ke frontend
	type AbnormalPeriod struct {
		Start       string `json:"start"`
		End         string `json:"end"`
		SuhuHeating string `json:"suhu_heating,omitempty"`
		SuhuHolding string `json:"suhu_holding,omitempty"`
	}

	var raws []rawPeriod
	var result []AbnormalPeriod

	query := `
	WITH flagged AS (
		SELECT Waktu, SuhuHeating, SuhuHolding
		FROM readsensors_pasteurisasi1
		WHERE DATE(Waktu) = ?
	),
	with_groups AS (
		SELECT *,
			ROW_NUMBER() OVER (ORDER BY Waktu) -
			ROW_NUMBER() OVER (PARTITION BY (SuhuHeating < 105 OR SuhuHeating > 120) ORDER BY Waktu) AS grp_heat,
			ROW_NUMBER() OVER (ORDER BY Waktu) -
			ROW_NUMBER() OVER (PARTITION BY (SuhuHolding < 105 OR SuhuHolding > 120) ORDER BY Waktu) AS grp_hold
		FROM flagged
	)
	SELECT 
		MIN(Waktu) AS start,
		MAX(Waktu) AS end,
		MIN(SuhuHeating) AS min_heating,
		MAX(SuhuHeating) AS max_heating,
		MIN(SuhuHolding) AS min_holding,
		MAX(SuhuHolding) AS max_holding
	FROM with_groups
	GROUP BY grp_heat, grp_hold
	ORDER BY start;
	`

	if err := config.DB.Raw(query, tanggal).Scan(&raws).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil data abnormal",
			"error":   err.Error(),
		})
		return
	}

	// mapping ke hasil akhir
	for _, r := range raws {
		ab := AbnormalPeriod{
			Start: r.Start.Format("2006-01-02 15:04:05"),
			End:   r.End.Format("2006-01-02 15:04:05"),
		}

		if r.MaxHeating > 120 {
			ab.SuhuHeating = ">120"
		} else if r.MinHeating < 105 {
			ab.SuhuHeating = "<105"
		}

		if r.MaxHolding > 120 {
			ab.SuhuHolding = ">120"
		} else if r.MinHolding < 105 {
			ab.SuhuHolding = "<105"
		}

		if ab.SuhuHeating != "" || ab.SuhuHolding != "" {
			result = append(result, ab)
		}
	}

	// encode JSON tanpa escape < >
	c.Writer.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(c.Writer)
	enc.SetEscapeHTML(false)
	enc.Encode(gin.H{
		"success": true,
		"tanggal": tanggal,
		"count":   len(result),
		"data":    result,
	})
}

// getTimeRange - Helper function untuk mendapatkan start dan end time (Asia/Jakarta)
func getTimeRange(c *gin.Context) (time.Time, time.Time, error) {
	// Default: 8 jam terakhir (waktu Jakarta)
	endTime := time.Now().In(jakartaLoc)
	startTime := endTime.Add(-8 * time.Hour)

	// Parse start_date jika ada (format: 2006-01-02 15:04:05 atau 2006-01-02T15:04:05)
	if startStr := c.Query("start_date"); startStr != "" {
		parsed, err := time.ParseInLocation("2006-01-02 15:04:05", startStr, jakartaLoc)
		if err != nil {
			// Coba format ISO 8601
			parsed, err = time.ParseInLocation("2006-01-02T15:04:05", startStr, jakartaLoc)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
		}
		startTime = parsed
	}

	// Parse end_date jika ada
	if endStr := c.Query("end_date"); endStr != "" {
		parsed, err := time.ParseInLocation("2006-01-02 15:04:05", endStr, jakartaLoc)
		if err != nil {
			// Coba format ISO 8601
			parsed, err = time.ParseInLocation("2006-01-02T15:04:05", endStr, jakartaLoc)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
		}
		endTime = parsed
	}

	return startTime, endTime, nil
}

// GetAverageFlowrate - Menampilkan rata-rata flowrate per menit
func GetAverageFlowrate(c *gin.Context) {
	var results []AvgResponse

	// Dapatkan time range
	startTime, endTime, err := getTimeRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid date format. Use: YYYY-MM-DD HH:MM:SS or YYYY-MM-DDTHH:MM:SS",
			"details": err.Error(),
		})
		return
	}

	// Query untuk menghitung average per menit dengan filter tanggal
	// Data di database sudah dalam timezone WIB, jadi tidak perlu CONVERT_TZ
	// Format waktu ke string untuk memastikan MySQL menerima waktu yang tepat
	startStr := startTime.Format("2006-01-02 15:04:05")
	endStr := endTime.Format("2006-01-02 15:04:05")
	
	query := `
		SELECT 
			DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00') as timestamp,
			AVG(Flowrate) as average
		FROM readsensors_pasteurisasi1
		WHERE Waktu >= ? AND Waktu <= ?
		GROUP BY DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00')
		ORDER BY timestamp ASC
	`

	if err := config.DB.Raw(query, startStr, endStr).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch average flowrate data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"count":  len(results),
		"filter": gin.H{
			"start_date": startTime.Format("2006-01-02 15:04:05"),
			"end_date":   endTime.Format("2006-01-02 15:04:05"),
			"timezone":   "Asia/Jakarta (WIB)",
		},
	})
}

// GetAverageSuhuHeating - Menampilkan rata-rata suhu heating per menit
func GetAverageSuhuHeating(c *gin.Context) {
	var results []AvgResponse

	// Dapatkan time range
	startTime, endTime, err := getTimeRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid date format. Use: YYYY-MM-DD HH:MM:SS or YYYY-MM-DDTHH:MM:SS",
			"details": err.Error(),
		})
		return
	}

	// Format waktu ke string untuk memastikan MySQL menerima waktu yang tepat
	startStr := startTime.Format("2006-01-02 15:04:05")
	endStr := endTime.Format("2006-01-02 15:04:05")
	
	query := `
		SELECT 
			DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00') as timestamp,
			AVG(SuhuHeating) as average
		FROM readsensors_pasteurisasi1
		WHERE Waktu >= ? AND Waktu <= ?
		GROUP BY DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00')
		ORDER BY timestamp ASC
	`

	if err := config.DB.Raw(query, startStr, endStr).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch average suhu heating data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"count":  len(results),
		"filter": gin.H{
			"start_date": startTime.Format("2006-01-02 15:04:05"),
			"end_date":   endTime.Format("2006-01-02 15:04:05"),
			"timezone":   "Asia/Jakarta (WIB)",
		},
	})
}

// GetAverageSuhuHolding - Menampilkan rata-rata suhu holding per menit
func GetAverageSuhuHolding(c *gin.Context) {
	var results []AvgResponse

	// Dapatkan time range
	startTime, endTime, err := getTimeRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid date format. Use: YYYY-MM-DD HH:MM:SS or YYYY-MM-DDTHH:MM:SS",
			"details": err.Error(),
		})
		return
	}

	// Format waktu ke string untuk memastikan MySQL menerima waktu yang tepat
	startStr := startTime.Format("2006-01-02 15:04:05")
	endStr := endTime.Format("2006-01-02 15:04:05")
	
	query := `
		SELECT 
			DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00') as timestamp,
			AVG(SuhuHolding) as average
		FROM readsensors_pasteurisasi1
		WHERE Waktu >= ? AND Waktu <= ?
		GROUP BY DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00')
		ORDER BY timestamp ASC
	`

	if err := config.DB.Raw(query, startStr, endStr).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch average suhu holding data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"count":  len(results),
		"filter": gin.H{
			"start_date": startTime.Format("2006-01-02 15:04:05"),
			"end_date":   endTime.Format("2006-01-02 15:04:05"),
			"timezone":   "Asia/Jakarta (WIB)",
		},
	})
}

// GetDivertStatusPasteur1 - Mendapatkan status divert realtime Pasteur 1
func GetDivertStatusPasteur1(c *gin.Context) {
	var latest models.SensorPasteurisasi

	if err := config.DB.Order("Waktu DESC").First(&latest).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Data sensor pasteurisasi 1 tidak ditemukan",
		})
		return
	}

	isDivert, triggers, keterangan := services.CheckDivertTriggers(&latest)
	statusText := "NORMAL"
	if isDivert {
		statusText = "DIVERT"
	}

	// Cek status worker & active event
	workerInDivert, activeID, activeStart, _ := services.GlobalPasteurWorker.GetWorkerStatus()
	var activeEventInfo gin.H
	if workerInDivert && activeID > 0 {
		durasi := int64(time.Since(activeStart).Seconds())
		if durasi < 0 {
			durasi = 0
		}
		activeEventInfo = gin.H{
			"id":           activeID,
			"waktu_mulai":  activeStart.In(jakartaLoc).Format("2006-01-02 15:04:05"),
			"durasi_detik": durasi,
		}
	} else if isDivert {
		// Jika worker belum sempat mencatat tapi data terbaru abnormal, cari dari DB
		var activeDB models.DivertLogPasteur1
		if err := config.DB.Where("status = ?", "ACTIVE").Order("waktu_mulai DESC").First(&activeDB).Error; err == nil {
			durasi := int64(time.Since(activeDB.WaktuMulai).Seconds())
			if durasi < 0 {
				durasi = 0
			}
			activeEventInfo = gin.H{
				"id":           activeDB.ID,
				"waktu_mulai":  activeDB.WaktuMulai.In(jakartaLoc).Format("2006-01-02 15:04:05"),
				"durasi_detik": durasi,
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"status":       statusText,
		"is_divert":    isDivert,
		"pemicu":       triggers,
		"keterangan":   keterangan,
		"active_event": activeEventInfo,
		"sensor_latest": gin.H{
			"waktu":        latest.Waktu.In(jakartaLoc).Format("2006-01-02 15:04:05"),
			"suhu_heating": latest.SuhuHeating,
			"suhu_holding": latest.SuhuHolding,
			"flowrate":     latest.Flowrate,
		},
		"limits": gin.H{
			"suhu_heating_min": services.LimitTempMin,
			"suhu_heating_max": services.LimitTempMax,
			"suhu_holding_min": services.LimitTempMin,
			"suhu_holding_max": services.LimitTempMax,
			"flowrate_max":     services.LimitFlowrateMax,
		},
		"timestamp": time.Now().In(jakartaLoc).Format("2006-01-02 15:04:05"),
	})
}

// GetDivertLogsPasteur1 - Mengambil daftar riwayat log kejadian divert Pasteur 1
func GetDivertLogsPasteur1(c *gin.Context) {
	tanggal := c.Query("tanggal") // format YYYY-MM-DD (opsional)
	pemicu := c.Query("pemicu")   // opsional filter (HEATING_UNDER, HEATING_OVER, HOLDING_UNDER, HOLDING_OVER, FLOWRATE_OVER)
	status := c.Query("status")   // opsional filter (ACTIVE, CLOSED)
	limitStr := c.DefaultQuery("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	var logs []models.DivertLogPasteur1
	dbQuery := config.DB.Order("waktu_mulai DESC")

	if tanggal != "" {
		dbQuery = dbQuery.Where("DATE(waktu_mulai) = ?", tanggal)
	}

	if pemicu != "" {
		dbQuery = dbQuery.Where("pemicu LIKE ?", "%"+pemicu+"%")
	}

	if status != "" {
		dbQuery = dbQuery.Where("status = ?", status)
	}

	if err := dbQuery.Limit(limit).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil data log divert pasteur 1",
			"error":   err.Error(),
		})
		return
	}

	var totalDurasiDetik int64
	for _, l := range logs {
		totalDurasiDetik += l.DurasiDetik
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"tanggal":            tanggal,
		"total_count":        len(logs),
		"total_durasi_detik": totalDurasiDetik,
		"total_durasi_menit": fmt.Sprintf("%.1f", float64(totalDurasiDetik)/60.0),
		"data":               logs,
	})
}

// GetDivertSummaryPasteur1 - Menampilkan ringkasan statistik divert per tanggal
func GetDivertSummaryPasteur1(c *gin.Context) {
	tanggal := c.Query("tanggal") // format YYYY-MM-DD (opsional, default: hari ini)
	if tanggal == "" {
		tanggal = time.Now().In(jakartaLoc).Format("2006-01-02")
	}

	var logs []models.DivertLogPasteur1
	if err := config.DB.Where("DATE(waktu_mulai) = ?", tanggal).Order("waktu_mulai ASC").Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil ringkasan divert pasteur 1",
			"error":   err.Error(),
		})
		return
	}

	totalKejadian := len(logs)
	var totalDurasiDetik int64
	var maxDurasiDetik int64
	var maxDurasiID uint

	breakdownPemicu := map[string]int{
		"heating_under": 0,
		"heating_over":  0,
		"holding_under": 0,
		"holding_over":  0,
		"flowrate_over": 0,
	}

	for _, l := range logs {
		totalDurasiDetik += l.DurasiDetik
		if l.DurasiDetik > maxDurasiDetik {
			maxDurasiDetik = l.DurasiDetik
			maxDurasiID = l.ID
		}

		if strings.Contains(l.Pemicu, "HEATING_UNDER") {
			breakdownPemicu["heating_under"]++
		}
		if strings.Contains(l.Pemicu, "HEATING_OVER") {
			breakdownPemicu["heating_over"]++
		}
		if strings.Contains(l.Pemicu, "HOLDING_UNDER") {
			breakdownPemicu["holding_under"]++
		}
		if strings.Contains(l.Pemicu, "HOLDING_OVER") {
			breakdownPemicu["holding_over"]++
		}
		if strings.Contains(l.Pemicu, "FLOWRATE_OVER") {
			breakdownPemicu["flowrate_over"]++
		}
	}

	var rataRataDurasiDetik float64
	if totalKejadian > 0 {
		rataRataDurasiDetik = float64(totalDurasiDetik) / float64(totalKejadian)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                true,
		"tanggal":                tanggal,
		"total_kejadian":         totalKejadian,
		"total_durasi_detik":     totalDurasiDetik,
		"total_durasi_menit":     fmt.Sprintf("%.1f", float64(totalDurasiDetik)/60.0),
		"rata_rata_durasi_detik": fmt.Sprintf("%.1f", rataRataDurasiDetik),
		"kejadian_terlama_detik": maxDurasiDetik,
		"kejadian_terlama_id":    maxDurasiID,
		"breakdown_pemicu":       breakdownPemicu,
	})
}

// DetectAndRecordDivertPasteur1 - Memindai data sensor historis dan mencatat blok kejadian divert
func DetectAndRecordDivertPasteur1(c *gin.Context) {
	tanggal := c.Query("tanggal") // YYYY-MM-DD (opsional, default: hari ini)
	if tanggal == "" {
		tanggal = time.Now().In(jakartaLoc).Format("2006-01-02")
	}

	var sensors []models.SensorPasteurisasi
	query := config.DB.Where("DATE(Waktu) = ?", tanggal).Order("Waktu ASC")

	if err := query.Find(&sensors).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal membaca sensor pasteur 1",
			"error":   err.Error(),
		})
		return
	}

	type divertBlock struct {
		start       time.Time
		end         time.Time
		triggers    []string
		details     []string
		heating     float64
		holding     float64
		flowrate    float64
		hasRecorded bool
	}

	var blocks []divertBlock
	var currentBlock *divertBlock

	for _, s := range sensors {
		isDivert, triggers, detail := services.CheckDivertTriggers(&s)

		if isDivert {
			if currentBlock == nil {
				// Mulai blok divert baru
				currentBlock = &divertBlock{
					start:    s.Waktu,
					end:      s.Waktu,
					triggers: triggers,
					details:  []string{detail},
					heating:  s.SuhuHeating,
					holding:  s.SuhuHolding,
					flowrate: s.Flowrate,
				}
			} else {
				// Perpanjang blok divert
				currentBlock.end = s.Waktu
				// Gabungkan pemicu baru jika ada
				for _, tr := range triggers {
					exists := false
					for _, existing := range currentBlock.triggers {
						if existing == tr {
							exists = true
							break
						}
					}
					if !exists {
						currentBlock.triggers = append(currentBlock.triggers, tr)
					}
				}
			}
		} else {
			if currentBlock != nil {
				// Selesaikan blok divert
				blocks = append(blocks, *currentBlock)
				currentBlock = nil
			}
		}
	}

	// Tangani jika blok masih terbuka hingga akhir data
	if currentBlock != nil {
		blocks = append(blocks, *currentBlock)
	}

	newDiverts := 0
	var createdLogs []models.DivertLogPasteur1

	now := time.Now().In(jakartaLoc)
	for _, b := range blocks {
		// Cek apakah sudah pernah dicatat di database untuk waktu_mulai ini
		var count int64
		config.DB.Model(&models.DivertLogPasteur1{}).
			Where("waktu_mulai = ?", b.start).
			Count(&count)
		if count > 0 {
			continue
		}

		durasi := int64(b.end.Sub(b.start).Seconds())
		if durasi < 0 {
			durasi = 0
		}
		endTime := b.end

		keterangan := strings.Join(b.details, ", ")
		if len(b.details) == 0 {
			keterangan = fmt.Sprintf("Divert Pasteur 1: %s", strings.Join(b.triggers, ", "))
		}

		logEntry := models.DivertLogPasteur1{
			WaktuMulai:   b.start,
			WaktuSelesai: &endTime,
			DurasiDetik:  durasi,
			SuhuHeating:  b.heating,
			SuhuHolding:  b.holding,
			Flowrate:     b.flowrate,
			Pemicu:       strings.Join(b.triggers, ","),
			Keterangan:   keterangan,
			Status:       "CLOSED",
			Count:        1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := config.DB.Create(&logEntry).Error; err == nil {
			newDiverts++
			createdLogs = append(createdLogs, logEntry)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"tanggal":           tanggal,
		"total_blocks_found": len(blocks),
		"new_diverts_saved": newDiverts,
		"data":              createdLogs,
	})
}

// CreateDivertLogPasteur1 - Endpoint POST manual / pemicu langsung dari sistem PLC
func CreateDivertLogPasteur1(c *gin.Context) {
	type CreateInput struct {
		WaktuMulai   *time.Time `json:"waktu_mulai"`
		WaktuSelesai *time.Time `json:"waktu_selesai"`
		DurasiDetik  int64      `json:"durasi_detik"`
		SuhuHeating  float64    `json:"suhu_heating"`
		SuhuHolding  float64    `json:"suhu_holding"`
		Flowrate     float64    `json:"flowrate"`
		Pemicu       string     `json:"pemicu"`
		Keterangan   string     `json:"keterangan"`
		Status       string     `json:"status"`
	}

	var input CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Format request tidak valid",
			"error":   err.Error(),
		})
		return
	}

	waktuMulai := time.Now().In(jakartaLoc)
	if input.WaktuMulai != nil && !input.WaktuMulai.IsZero() {
		waktuMulai = *input.WaktuMulai
	}

	durasi := input.DurasiDetik
	if durasi == 0 && input.WaktuSelesai != nil && !input.WaktuSelesai.IsZero() {
		durasi = int64(input.WaktuSelesai.Sub(waktuMulai).Seconds())
		if durasi < 0 {
			durasi = 0
		}
	}

	status := input.Status
	if status == "" {
		if input.WaktuSelesai != nil {
			status = "CLOSED"
		} else {
			status = "ACTIVE"
		}
	}

	pemicu := input.Pemicu
	if pemicu == "" {
		pemicu = "MANUAL_ENTRY"
	}

	keterangan := input.Keterangan
	if keterangan == "" {
		keterangan = fmt.Sprintf("Divert Pasteur 1 (Heating: %.0f, Holding: %.0f, Flowrate: %.0f)",
			input.SuhuHeating, input.SuhuHolding, input.Flowrate)
	}

	now := time.Now().In(jakartaLoc)
	divertLog := models.DivertLogPasteur1{
		WaktuMulai:   waktuMulai,
		WaktuSelesai: input.WaktuSelesai,
		DurasiDetik:  durasi,
		SuhuHeating:  input.SuhuHeating,
		SuhuHolding:  input.SuhuHolding,
		Flowrate:     input.Flowrate,
		Pemicu:       pemicu,
		Keterangan:   keterangan,
		Status:       status,
		Count:        1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := config.DB.Create(&divertLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menyimpan log divert pasteur 1",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Log divert Pasteur 1 berhasil dicatat",
		"data":    divertLog,
	})
}