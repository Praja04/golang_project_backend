package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"backend-golang/config"
	"backend-golang/models"
)

// Response structures
type SeparatorStatus struct {
	ID         int       `json:"id"`
	Status     string    `json:"status"`
	Duration   int64     `json:"duration"`
	Count      int64     `json:"count"`
	LastUpdate time.Time `json:"last_update"`
	Online     bool      `json:"online"`
}

type SeparatorResponse struct {
	Success   bool                       `json:"success"`
	Message   string                     `json:"message"`
	Data      *models.SeparatorSensor    `json:"data,omitempty"`
	Timestamp time.Time                  `json:"timestamp"`
}

type SeparatorListResponse struct {
	Success   bool                      `json:"success"`
	Message   string                    `json:"message"`
	Data      []models.SeparatorSensor  `json:"data,omitempty"`
	Timestamp time.Time                 `json:"timestamp"`
	Total     int64                     `json:"total"`
}

type ActivityLogEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Separator  int       `json:"separator"`
	Action     string    `json:"action"`
	Duration   string    `json:"duration"`
	Status     string    `json:"status"`
}

// Helper functions for data consistency
// Separator values: 0 = CLOSED, 1 = OPEN
func getSeparatorStatusText(value int) string {
	if value == 1 {
		return "OPEN"
	}
	return "CLOSED"
}

func getSeparatorActionAndStatus(value int) (string, string) {
	if value == 1 {
		return "open", "success"   // 1 = OPEN = success (green)
	}
	return "close", "danger"       // 0 = CLOSED = danger (red)
}

func isValidSeparatorValue(value int) bool {
	return value == 0 || value == 1
}

// GetLatestSeparatorData - Mendapatkan data sensor separator terbaru
func GetLatestSeparatorData(c *gin.Context) {
	var latest models.SeparatorSensor
	
	// Load timezone Asia/Jakarta
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC // fallback
	}
	
	// Query data terbaru
	result := config.DB.Order("waktu DESC").First(&latest)
	
	if result.Error != nil {
		c.JSON(http.StatusOK, SeparatorResponse{
			Success:   false,
			Message:   "No data available",
			Timestamp: time.Now().In(loc),
		})
		return
	}
	
	// Convert waktu to Asia/Jakarta timezone for response
	latest.Waktu = latest.Waktu.In(loc)
	
	c.JSON(http.StatusOK, SeparatorResponse{
		Success:   true,
		Message:   "Latest separator data retrieved successfully",
		Data:      &latest,
		Timestamp: time.Now().In(loc),
	})
}

// GetSeparatorHistoryByDate - Mendapatkan data historis separator dengan pembagian shift
func GetSeparatorHistoryByDate(c *gin.Context) {
	// Ambil parameter tanggal
	dateParam := c.DefaultQuery("tanggal", time.Now().Format("2006-01-02"))

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC
	}

	// Parse tanggal dalam timezone lokal
	baseDate, err := time.ParseInLocation("2006-01-02", dateParam, loc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Format tanggal tidak valid. Gunakan YYYY-MM-DD",
			"timestamp": time.Now().In(loc),
		})
		return
	}

	// Debug: log timezone info
	fmt.Printf("Base date: %v\n", baseDate)
	fmt.Printf("Base date timezone: %v\n", baseDate.Location())

	// Rentang waktu operasional: 06:00 hari ini - 05:59:59 besok (dalam WIB)
	startTime := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 6, 0, 0, 0, loc)
	endTime := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day()+1, 5, 59, 59, 0, loc)

	fmt.Printf("Start time: %v (UTC: %v)\n", startTime, startTime.UTC())
	fmt.Printf("End time: %v (UTC: %v)\n", endTime, endTime.UTC())

	// Convert to UTC for database query (assuming DB stores in UTC)
	startTimeUTC := startTime.UTC()
	endTimeUTC := endTime.UTC()

	// Definisi shift dalam WIB
	shifts := map[string][2]time.Time{
		"shift1": {
			time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 6, 0, 0, 0, loc),
			time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 14, 0, 0, 0, loc),
		},
		"shift2": {
			time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 14, 0, 1, 0, loc),
			time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 22, 0, 0, 0, loc),
		},
		"shift3": {
			time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 22, 0, 1, 0, loc),
			time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day()+1, 5, 59, 59, 0, loc),
		},
	}

	// Struct untuk response data
	type EnrichedSeparatorSensor struct {
		Waktu      time.Time `json:"Waktu"`
		Separator1 int       `json:"Separator1"`
		Separator2 int       `json:"Separator2"`
		Separator3 int       `json:"Separator3"`
		Separator4 int       `json:"Separator4"`
		Shift      string    `json:"Shift"`
	}

	var allData []EnrichedSeparatorSensor
	var totalRecords int64

	// Hitung total record dalam 1 hari operasional (query in UTC)
	config.DB.Model(&models.SeparatorSensor{}).
		Where("waktu BETWEEN ? AND ?", startTimeUTC, endTimeUTC).
		Count(&totalRecords)

	fmt.Printf("Total records found: %d\n", totalRecords)

	// Ambil dan gabungkan data per shift
	for shiftName, timeRange := range shifts {
		var shiftHistory []models.SeparatorSensor

		// Convert shift time range to UTC for database query
		shiftStartUTC := timeRange[0].UTC()
		shiftEndUTC := timeRange[1].UTC()

		fmt.Printf("Shift %s: %v - %v (UTC: %v - %v)\n", 
			shiftName, timeRange[0], timeRange[1], shiftStartUTC, shiftEndUTC)

		shiftResult := config.DB.
			Where("waktu BETWEEN ? AND ?", shiftStartUTC, shiftEndUTC).
			Order("waktu ASC").
			Find(&shiftHistory)

		if shiftResult.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":   false,
				"message":   fmt.Sprintf("Database error: %v", shiftResult.Error),
				"timestamp": time.Now().In(loc),
			})
			return
		}

		fmt.Printf("Shift %s: %d records\n", shiftName, len(shiftHistory))

		for _, row := range shiftHistory {
			// Convert waktu from database (assumed UTC) to WIB for response
			waktuWIB := row.Waktu.In(loc)
			allData = append(allData, EnrichedSeparatorSensor{
				Waktu:      waktuWIB,
				Separator1: row.Separator1,
				Separator2: row.Separator2,
				Separator3: row.Separator3,
				Separator4: row.Separator4,
				Shift:      shiftName,
			})
		}
	}

	

	// Response final
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   fmt.Sprintf("Data separator untuk tanggal %s berhasil diambil", dateParam),
		"date":      dateParam,
		"period": gin.H{
			"start": startTime.Format("2006-01-02 15:04:05"),
			"end":   endTime.Format("2006-01-02 15:04:05"),
		},
		"start_time": startTime.Format("2006-01-02 15:04:05"),
		"end_time":   endTime.Format("2006-01-02 15:04:05"),
		"data":       allData,
		"total_records": totalRecords,
		"timestamp":  time.Now().In(loc),
		"debug": gin.H{
			"base_date": baseDate,
			"start_time_utc": startTimeUTC,
			"end_time_utc": endTimeUTC,
			"timezone": loc.String(),
		},
	})
}

// GetSeparatorLogs - Mendapatkan activity log separator
func GetSeparatorLogs(c *gin.Context) {
	// Parse query parameters
	separatorParam := c.Query("separator") // filter by separator (1,2,3,4)
	actionParam := c.Query("action")       // filter by action (open/close)
	hoursParam := c.DefaultQuery("hours", "24")
	limitParam := c.DefaultQuery("limit", "50")
	
	hours, err := strconv.Atoi(hoursParam)
	if err != nil || hours <= 0 {
		hours = 24
	}
	
	limit, err := strconv.Atoi(limitParam)
	if err != nil || limit <= 0 {
		limit = 50
	}
	
	// Validate separator parameter
	if separatorParam != "" {
		sepID, err := strconv.Atoi(separatorParam)
		if err != nil || sepID < 1 || sepID > 4 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success":   false,
				"message":   "Invalid separator parameter. Must be 1, 2, 3, or 4",
				"timestamp": time.Now(),
			})
			return
		}
	}
	
	// Validate action parameter
	if actionParam != "" && actionParam != "open" && actionParam != "close" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid action parameter. Must be 'open' or 'close'",
			"timestamp": time.Now(),
		})
		return
	}
	
	// Load timezone Asia/Jakarta
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC // fallback
	}
	
	now := time.Now().In(loc)
	startTime := now.Add(-time.Duration(hours) * time.Hour)
	
	// Format time for database query
	startTimeStr := startTime.Format("2006-01-02 15:04:05")
	endTimeStr := now.Format("2006-01-02 15:04:05")
	
	// Build query for detecting status changes
	query := `
		SELECT 
			waktu,
			separator1,
			separator2, 
			separator3,
			separator4
		FROM readsensors_separator 
		WHERE waktu >= ? AND waktu <= ?
		ORDER BY waktu DESC
		LIMIT ?
	`
	
	var rawData []models.SeparatorSensor
	result := config.DB.Raw(query, startTimeStr, endTimeStr, limit*4).Scan(&rawData) // multiply by 4 for all separators
	
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   fmt.Sprintf("Database error: %v", result.Error),
			"timestamp": now,
		})
		return
	}
	
	// Process data to detect status changes
	var activityLog []ActivityLogEntry
	prevData := make(map[int]int) // separator_id -> prev_status
	
	for i := len(rawData) - 1; i >= 0; i-- { // Process chronologically
		data := rawData[i]
		separators := map[int]int{
			1: data.Separator1,
			2: data.Separator2,
			3: data.Separator3,
			4: data.Separator4,
		}
		
		for sepID, currentStatus := range separators {
			// Validate separator value
			if !isValidSeparatorValue(currentStatus) {
				continue
			}
			
			// Apply separator filter
			if separatorParam != "" {
				filterSep, _ := strconv.Atoi(separatorParam)
				if filterSep != sepID {
					continue
				}
			}
			
			prevStatus, exists := prevData[sepID]
			if exists && prevStatus != currentStatus {
				// Status changed, create log entry
				action, status := getSeparatorActionAndStatus(currentStatus)
				
				// Apply action filter
				if actionParam != "" && actionParam != action {
					continue
				}
				
				// Calculate duration (simplified - you might want more complex logic)
				duration := "N/A"
				
				entry := ActivityLogEntry{
					Timestamp: data.Waktu.In(loc),
					Separator: sepID,
					Action:    action,
					Duration:  duration,
					Status:    status,
				}
				
				activityLog = append(activityLog, entry)
				
				if len(activityLog) >= limit {
					break
				}
			}
			
			prevData[sepID] = currentStatus
		}
		
		if len(activityLog) >= limit {
			break
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   fmt.Sprintf("Retrieved %d activity log entries", len(activityLog)),
		"data":      activityLog,
		"timestamp": now,
	})
}

// GetSeparatorStatus - Mendapatkan status realtime semua separator
func GetSeparatorStatus(c *gin.Context) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	// Ambil data separator terbaru
	type LatestRow struct {
		Waktu      time.Time
		Separator1 int
		Separator2 int
		Separator3 int
		Separator4 int
	}
	var latest LatestRow
	err := config.DB.Raw(`
		SELECT waktu, separator1, separator2, separator3, separator4
		FROM readsensors_separator
		ORDER BY waktu DESC
		LIMIT 1
	`).Scan(&latest).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal ambil data terbaru"})
		return
	}

	// Konversi status
	statusText := func(val int) string {
		if val == 1 {
			return "OPEN"
		}
		return "CLOSED"
	}
	status := map[int]string{
		1: statusText(latest.Separator1),
		2: statusText(latest.Separator2),
		3: statusText(latest.Separator3),
		4: statusText(latest.Separator4),
	}

	// Definisi shift
	today := now.Format("2006-01-02")
	tomorrow := now.Add(24 * time.Hour).Format("2006-01-02")
	shifts := map[string][2]string{
		"shift1": {today + " 06:00:00", today + " 14:00:00"},
		"shift2": {today + " 14:00:01", today + " 22:00:00"},
		"shift3": {today + " 22:00:01", tomorrow + " 05:59:59"},
	}

	// Struktur hasil
	type ShiftStat struct {
		Duration int64 `json:"duration"` // jumlah data point bernilai 1
		Count    int64 `json:"count"`    // jumlah blok aktif bernilai 1
	}
	result := make(map[int]map[string]ShiftStat)
	for sepID := 1; sepID <= 4; sepID++ {
		result[sepID] = map[string]ShiftStat{
			"shift1": {},
			"shift2": {},
			"shift3": {},
		}
	}

	// Proses per shift
	for shiftName, timeRange := range shifts {
		query := fmt.Sprintf(`
			SELECT waktu, separator1, separator2, separator3, separator4
			FROM readsensors_separator
			WHERE waktu BETWEEN '%s' AND '%s'
			ORDER BY waktu ASC
		`, timeRange[0], timeRange[1])

		var rows []models.SeparatorSensor
		if err := config.DB.Raw(query).Scan(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Gagal ambil data shift"})
			return
		}

		// Hitung duration dan count berdasarkan blok aktif
		activePeriods := make(map[int]int)     // separator -> jumlah blok aktif
		inActive := make(map[int]bool)         // separator -> sedang dalam blok aktif

		for _, row := range rows {
			separators := map[int]int{
				1: row.Separator1,
				2: row.Separator2,
				3: row.Separator3,
				4: row.Separator4,
			}
			for sepID, val := range separators {
				if val == 1 {
					// Tambah durasi
					result[sepID][shiftName] = ShiftStat{
						Duration: result[sepID][shiftName].Duration + 1,
						Count:    result[sepID][shiftName].Count,
					}
					// Jika baru mulai blok aktif
					if !inActive[sepID] {
						activePeriods[sepID]++
						inActive[sepID] = true
					}
				} else {
					inActive[sepID] = false
				}
			}
		}

		// Finalisasi count
		for sepID := 1; sepID <= 4; sepID++ {
			result[sepID][shiftName] = ShiftStat{
				Duration: result[sepID][shiftName].Duration,
				Count:    int64(activePeriods[sepID]),
			}
		}
	}
	
	// Kirim response
	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Status separator berhasil diambil",
		"last_update": latest.Waktu,
		"status":      status,
		"data":        result,
		"timestamp":   now,
	})
}

func GetSeparatorSensorByShift(c *gin.Context) {
	sqlDB, err := config.DB.DB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil koneksi database"})
		return
	}

	// Ambil parameter tanggal dan shift
	dateParam := c.Query("tanggal")
	shiftParam := c.Query("shift")

	var baseDate time.Time
	if dateParam != "" {
		baseDate, err = time.Parse("2006-01-02", dateParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal harus YYYY-MM-DD"})
			return
		}
	} else {
		baseDate = time.Now()
	}

	// Tentukan shift aktif jika tidak ada parameter shift
	var shift int
	if shiftParam != "" {
		shift, err = strconv.Atoi(shiftParam)
		if err != nil || shift < 1 || shift > 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Shift harus bernilai 1, 2, atau 3"})
			return
		}
	} else {
		now := time.Now()
		hour := now.Hour()
		minute := now.Minute()
		currentTime := hour*3600 + minute*60 + now.Second()

		if currentTime >= 6*3600 && currentTime <= 14*3600 {
			shift = 1
		} else if currentTime > 14*3600 && currentTime <= 22*3600 {
			shift = 2
		} else {
			shift = 3
		}
	}

	// Hitung waktu mulai dan akhir berdasarkan shift
	var startTime, endTime string
	switch shift {
	case 1:
		startTime = baseDate.Format("2006-01-02") + " 06:00:00"
		endTime = baseDate.Format("2006-01-02") + " 14:00:00"
	case 2:
		startTime = baseDate.Format("2006-01-02") + " 14:00:01"
		endTime = baseDate.Format("2006-01-02") + " 22:00:00"
	case 3:
		startTime = baseDate.Format("2006-01-02") + " 22:00:01"
		endTime = baseDate.AddDate(0, 0, 1).Format("2006-01-02") + " 05:59:59"
	}

	query := `
		SELECT waktu, separator1, separator2, separator3, separator4
		FROM readsensors_separator
		WHERE waktu BETWEEN ? AND ?
		ORDER BY waktu ASC
	`

	rows, err := sqlDB.Query(query, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menjalankan query"})
		return
	}
	defer rows.Close()

	type SensorData struct {
		Waktu      string `json:"waktu"`
		Separator1 int    `json:"separator1"`
		Separator2 int    `json:"separator2"`
		Separator3 int    `json:"separator3"`
		Separator4 int    `json:"separator4"`
	}

	var results []SensorData
	for rows.Next() {
		var data SensorData
		if err := rows.Scan(&data.Waktu, &data.Separator1, &data.Separator2, &data.Separator3, &data.Separator4); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membaca data"})
			return
		}
		results = append(results, data)
	}

	c.JSON(http.StatusOK, gin.H{
		"tanggal": baseDate.Format("2006-01-02"),
		"shift":   shift,
		"data":    results,
	})
}

