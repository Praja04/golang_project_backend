package controllers

import (
	"fmt"
	"net/http"

	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"backend-golang/config"
	"backend-golang/models"
)

// Helper function untuk mendapatkan model berdasarkan line
func getModelByLine(line string) interface{} {
	switch strings.ToLower(line) {
	case "d1":
		return &models.RetailD1{}
	case "d2":
		return &models.RetailD2{}
	case "d3":
		return &models.RetailD3{}
	case "d4":
		return &models.RetailD4{}
	case "d5":
		return &models.RetailD5{}
	case "d6":
		return &models.RetailD6{}
	case "d7":
		return &models.RetailD7{}
	case "d8":
		return &models.RetailD8{}
	case "d9":
		return &models.RetailD9{}
	case "d10":
		return &models.RetailD10{}
	case "d14":
		return &models.RetailD14{}
	default:
		return nil
	}
}

// Helper function untuk mendapatkan table name berdasarkan line
func getTableByLine(line string) string {
	return fmt.Sprintf("retail_%s", strings.ToLower(line))
}

// Ambil total runtime (start_mesin = 1) dari DB
func getShiftRuntime(line string, start, end, now time.Time) int64 {
	// Jika shift belum dimulai, return 0
	if now.Before(start) {
		return 0
	}
	
	// Konversi ke Asia/Jakarta dulu, lalu format tanpa timezone (seperti di DB)
	loc, _ := time.LoadLocation("Asia/Jakarta")
	startLocal := start.In(loc)
	endLocal := end.In(loc)
	
	// Format ke string tanpa timezone (format yang sama dengan DB)
	startStr := startLocal.Format("2006-01-02 15:04:05")
	endStr := endLocal.Format("2006-01-02 15:04:05")
	
	// Debug: print query parameters
	fmt.Printf("Query DB Runtime %s - Start: %s, End: %s\n", line, startStr, endStr)
	
	model := getModelByLine(line)
	if model == nil {
		fmt.Printf("Invalid line: %s\n", line)
		return 0
	}

	var countSeconds int64
	result := config.DB.Model(model).
		Where("start_mesin = ? AND ts >= ? AND ts <= ?", 1, startStr, endStr).
		Count(&countSeconds)

	if result.Error != nil {
		fmt.Printf("DB Error for %s: %v\n", line, result.Error)
		return 0
	}

	fmt.Printf("DB Result Runtime %s - Count: %d seconds (%d minutes)\n", line, countSeconds, countSeconds/60)
	
	return countSeconds / 60 // convert detik → menit
}

// Ambil total stoptime (start_mesin = 0) dari DB
func getShiftStoptime(line string, start, end, now time.Time) int64 {
	// Jika shift belum dimulai, return 0
	if now.Before(start) {
		return 0
	}
	
	// Konversi ke Asia/Jakarta dulu, lalu format tanpa timezone (seperti di DB)
	loc, _ := time.LoadLocation("Asia/Jakarta")
	startLocal := start.In(loc)
	endLocal := end.In(loc)
	
	// Format ke string tanpa timezone (format yang sama dengan DB)
	startStr := startLocal.Format("2006-01-02 15:04:05")
	endStr := endLocal.Format("2006-01-02 15:04:05")
	
	// Debug: print query parameters
	fmt.Printf("Query DB Stoptime %s - Start: %s, End: %s\n", line, startStr, endStr)
	
	model := getModelByLine(line)
	if model == nil {
		fmt.Printf("Invalid line: %s\n", line)
		return 0
	}

	var countSeconds int64
	result := config.DB.Model(model).
		Where("start_mesin = ? AND ts >= ? AND ts <= ?", 0, startStr, endStr).
		Count(&countSeconds)

	if result.Error != nil {
		fmt.Printf("DB Error for %s: %v\n", line, result.Error)
		return 0
	}

	fmt.Printf("DB Result Stoptime %s - Count: %d seconds (%d minutes)\n", line, countSeconds, countSeconds/60)
	
	return countSeconds / 60 // convert detik → menit
}

func getActualShiftMinutes(start, end, now time.Time) int64 {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	start = start.In(loc)
	end = end.In(loc)
	now = now.In(loc)

	var actualMinutes int64
	if now.Before(start) {
		actualMinutes = 0
	} else if now.After(end) {
		actualMinutes = int64(end.Sub(start).Minutes())
	} else {
		actualMinutes = int64(now.Sub(start).Minutes())
	}

	// Tentukan batas maksimal per hari
	weekday := start.Weekday() // time.Weekday (0=Sunday, 6=Saturday)
	var maxMinutes int64
	if weekday == time.Saturday {
		maxMinutes = 300 // Sabtu = 5 jam kerja
	} else {
		maxMinutes = 420 // default = 7 jam kerja
	}

	// Batasi
	if actualMinutes > maxMinutes {
		actualMinutes = maxMinutes
	}

	return actualMinutes
}

// Tentukan range shift (pakai timezone dari baseDate)
func getShiftRange(baseDate time.Time, shift int) (time.Time, time.Time) {
	loc := baseDate.Location()
	weekday := baseDate.Weekday()
	
	// Cek apakah hari Sabtu
	isSaturday := weekday == time.Saturday
	
	if isSaturday {
		// Shift khusus untuk Sabtu (5 jam per shift)
		switch shift {
		case 1:
			start := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 6, 0, 0, 0, loc)
			end := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 11, 0, 0, 0, loc)
			return start, end
		case 2:
			start := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 11, 0, 1, 0, loc)
			end := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 16, 0, 0, 0, loc)
			return start, end
		case 3:
			start := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 16, 0, 1, 0, loc)
			end := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 21, 0, 0, 0, loc)
			return start, end
		}
	} else {
		// Shift normal untuk hari biasa (7-8 jam per shift)
		switch shift {
		case 1:
			start := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 6, 0, 0, 0, loc)
			end := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 14, 0, 0, 0, loc)
			return start, end
		case 2:
			start := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 14, 1, 0, 0, loc)
			end := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 22, 0, 0, 0, loc)
			return start, end
		case 3:
			start := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 22, 1, 0, 0, loc)
			end := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day()+1, 5, 59, 59, 0, loc)
			return start, end
		}
	}
	
	return baseDate, baseDate
}

// Tentukan shift sekarang (disesuaikan untuk Sabtu)
func getCurrentShift(now time.Time) int {
	hour, min := now.Hour(), now.Minute()
	weekday := now.Weekday()
	
	if weekday == time.Saturday {
		// Shift khusus untuk Sabtu
		if hour >= 6 && (hour < 11 || (hour == 11 && min == 0)) {
			return 1
		} else if (hour > 11 || (hour == 11 && min >= 1)) && (hour < 16 || (hour == 16 && min == 0)) {
			return 2
		} else if (hour > 16 || (hour == 16 && min >= 1)) && hour < 21 {
			return 3
		}
	} else {
		// Shift normal untuk hari biasa
		if hour >= 6 && (hour < 14 || (hour == 14 && min == 0)) {
			return 1
		} else if (hour > 14 || (hour == 14 && min >= 1)) && hour < 22 {
			return 2
		} else {
			return 3
		}
	}
	
	return 3
}

// Controller untuk Uptime (Runtime)
func UptimeStartMesinRealtime(c *gin.Context) {
	line := c.Param("line")
	dateParam := c.Query("date")

	// Validasi line
	if getModelByLine(line) == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Line %s tidak valid. Gunakan d1-d14", line)})
		return
	}

	// Load Asia/Jakarta timezone
	loc, _ := time.LoadLocation("Asia/Jakarta")
	
	// Default pakai tanggal hari ini dalam Asia/Jakarta timezone
	baseDate := time.Now().In(loc)

	if dateParam != "" {
		// Parse date dan set ke Asia/Jakarta timezone
		parsedDate, err := time.Parse("2006-01-02", dateParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal salah. Gunakan YYYY-MM-DD"})
			return
		}
		// Set timezone ke Asia/Jakarta
		baseDate = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, loc)
	}

	// Gunakan waktu sekarang dalam Asia/Jakarta timezone
	now := time.Now().In(loc)
	
	// Debug: print current time
	fmt.Printf("Current time (Asia/Jakarta): %v for line %s\n", now, line)
	
	var shifts []gin.H

	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)
		
		// Debug: print shift times
		fmt.Printf("Shift %d for %s: Start=%v, End=%v\n", i, line, start, end)

		runtimeMinutes := getShiftRuntime(line, start, end, now)
		actualMinutes := getActualShiftMinutes(start, end, now)
		
		// Debug: print calculations
		fmt.Printf("Shift %d for %s: Runtime=%d, Actual=%d\n", i, line, runtimeMinutes, actualMinutes)

		uptime := 0.0
		if actualMinutes > 0 {
			uptime = float64(runtimeMinutes) / float64(actualMinutes) * 100 // dalam persen
		}

		shifts = append(shifts, gin.H{
			"shift":                 i,
			"start_time":            start,
			"end_time":              end,
			"runtime_total_minutes": runtimeMinutes,
			"actual_shift_minutes":  actualMinutes,
			"uptime":                uptime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"line":          line,
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}

// Controller untuk Downtime (Stoptime)
func DowntimeStopMesinRealtime(c *gin.Context) {
	line := c.Param("line")
	dateParam := c.Query("date")

	// Validasi line
	if getModelByLine(line) == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Line %s tidak valid. Gunakan d1-d14", line)})
		return
	}

	// Load Asia/Jakarta timezone
	loc, _ := time.LoadLocation("Asia/Jakarta")
	
	// Default pakai tanggal hari ini dalam Asia/Jakarta timezone
	baseDate := time.Now().In(loc)

	if dateParam != "" {
		// Parse date dan set ke Asia/Jakarta timezone
		parsedDate, err := time.Parse("2006-01-02", dateParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal salah. Gunakan YYYY-MM-DD"})
			return
		}
		// Set timezone ke Asia/Jakarta
		baseDate = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, loc)
	}

	// Gunakan waktu sekarang dalam Asia/Jakarta timezone
	now := time.Now().In(loc)
	
	// Debug: print current time
	fmt.Printf("Current time (Asia/Jakarta): %v for line %s\n", now, line)
	
	var shifts []gin.H

	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)
		
		// Debug: print shift times
		fmt.Printf("Shift %d for %s: Start=%v, End=%v\n", i, line, start, end)

		downtimeMinutes := getShiftStoptime(line, start, end, now)
		actualMinutes := getActualShiftMinutes(start, end, now)
		
		// Debug: print calculations
		fmt.Printf("Shift %d for %s: Downtime=%d, Actual=%d\n", i, line, downtimeMinutes, actualMinutes)

		downtime := 0.0
		if actualMinutes > 0 {
			downtime = float64(downtimeMinutes) / float64(actualMinutes) * 100 // dalam persen
		}

		shifts = append(shifts, gin.H{
			"shift":                  i,
			"start_time":             start,
			"end_time":               end,
			"downtime_total_minutes": downtimeMinutes,
			"actual_shift_minutes":   actualMinutes,
			"downtime":               downtime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"line":          line,
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}

// Optimized getLatestTotalCounter (1 query saja)
// Type definition untuk counter record
type CounterRecord struct {
	Ts           time.Time `json:"ts"`
	TotalCounter int       `json:"total_counter"`
}

// Optimized getLatestTotalCounter dengan logika yang diperbaiki
func getLatestTotalCounter(line string, start, end, now time.Time) int64 {
	if now.Before(start) {
		return 0
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	startStr := start.In(loc).Format("2006-01-02 15:04:05")
	endStr := end.In(loc).Format("2006-01-02 15:04:05")

	model := getModelByLine(line)
	if model == nil {
		return 0
	}

	// Menggunakan raw SQL query
	var records []CounterRecord

	tableName := getTableByLine(line)
	query := fmt.Sprintf("SELECT ts, total_counter FROM %s WHERE ts >= ? AND ts <= ? ORDER BY ts ASC", tableName)
	
	result := config.DB.Raw(query, startStr, endStr).Scan(&records)

	if result.Error != nil || len(records) == 0 {
		fmt.Printf("Error or no records for %s: %v\n", line, result.Error)
		return 0
	}

	// Cek apakah kita mendekati akhir shift (1 jam terakhir)
	// Hitung durasi shift yang sudah berlalu
	shiftDuration := end.Sub(start)
	elapsed := now.Sub(start)
	isNearEndShift := elapsed >= (shiftDuration - time.Hour) // 1 jam sebelum akhir shift
	
	fmt.Printf("Shift analysis for %s: Duration=%v, Elapsed=%v, NearEnd=%v\n", 
		line, shiftDuration, elapsed, isNearEndShift)

	if isNearEndShift {
		// LOGIKA MENDEKATI AKHIR SHIFT: Ambil data sebelum 0
		return getCounterBeforeZero(records, line)
	} else {
		// LOGIKA AWAL SHIFT: Prioritaskan counter yang mulai dari awal shift
		return getCounterForEarlyShift(records, line)
	}
}

// Helper function untuk mendapat counter sebelum nilai 0 (untuk akhir shift)
func getCounterBeforeZero(records []CounterRecord, line string) int64 {
	var lastBeforeZero int64 = 0
	
	for i := 0; i < len(records); i++ {
		current := records[i].TotalCounter
		
		if current > 0 {
			lastBeforeZero = int64(current)
			
			// Cek apakah data berikutnya adalah 0
			if i+1 < len(records) && records[i+1].TotalCounter == 0 {
				// Ini adalah data terakhir sebelum 0, gunakan ini
				fmt.Printf("Near end shift - Found data before zero for %s: %d at %v\n", 
					line, lastBeforeZero, records[i].Ts)
				return lastBeforeZero
			}
		}
	}
	
	// Jika tidak ada pola "sebelum 0", ambil yang terakhir > 0
	fmt.Printf("Near end shift - Using last non-zero for %s: %d\n", line, lastBeforeZero)
	return lastBeforeZero
}

// Helper function untuk mendapat counter di awal shift
func getCounterForEarlyShift(records []CounterRecord, line string) int64 {
	// Cari pola reset: dari nilai besar ke 0, lalu naik lagi
	var resetIndex = -1
	var maxValueBeforeReset int64 = 0
	
	// Cari titik reset (dari nilai tinggi ke 0)
	for i := 0; i < len(records)-1; i++ {
		current := records[i].TotalCounter
		next := records[i+1].TotalCounter
		
		if current > 0 && next == 0 {
			maxValueBeforeReset = int64(current)
			resetIndex = i + 1 // index dimana nilai menjadi 0
			break
		}
	}
	
	if resetIndex != -1 {
		// Ada reset, cari nilai terakhir setelah reset
		for i := resetIndex; i < len(records); i++ {
			if records[i].TotalCounter > 0 {
				// Cek apakah ini nilai yang masuk akal untuk shift baru
				currentValue := int64(records[i].TotalCounter)
				if currentValue < maxValueBeforeReset {
					// Nilai ini lebih kecil dari sebelum reset, kemungkinan counter baru
					// Cari nilai terakhir dari periode ini
					var lastInNewPeriod int64 = currentValue
					for j := i + 1; j < len(records); j++ {
						if records[j].TotalCounter > 0 {
							lastInNewPeriod = int64(records[j].TotalCounter)
						}
					}
					fmt.Printf("Early shift - Found counter after reset for %s: %d (was %d before reset)\n", 
						line, lastInNewPeriod, maxValueBeforeReset)
					return lastInNewPeriod
				}
			}
		}
	}
	
	// Tidak ada reset yang jelas, ambil nilai terakhir > 0
	var lastNonZero int64 = 0
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].TotalCounter > 0 {
			lastNonZero = int64(records[i].TotalCounter)
			fmt.Printf("Early shift - Found last non-zero for %s: %d at %v\n", 
				line, lastNonZero, records[i].Ts)
			break
		}
	}
	
	return lastNonZero
}

// Controller untuk Performance Output (optimized)
func PerformanceOutput(c *gin.Context) {
	line := c.Param("line")
	dateParam := c.Query("date")

	// Validasi line
	if getModelByLine(line) == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Line %s tidak valid. Gunakan d1-d14", line)})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")

	baseDate := time.Now().In(loc)
	if dateParam != "" {
		parsedDate, err := time.Parse("2006-01-02", dateParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal salah. Gunakan YYYY-MM-DD"})
			return
		}
		baseDate = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, loc)
	}

	now := time.Now().In(loc)
	var shifts []gin.H

	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)

		totalCounter := getLatestTotalCounter(line, start, end, now)
		actualMinutes := getActualShiftMinutes(start, end, now)

		expectedOutput := int64(0)
		performanceOutput := 0.0
		if actualMinutes > 0 {
			expectedOutput = actualMinutes * 40 * 2
			performanceOutput = float64(totalCounter) / float64(expectedOutput) * 100
		}

		shifts = append(shifts, gin.H{
			"shift":                i,
			"start_time":           start,
			"end_time":             end,
			"total_counter":        totalCounter,
			"actual_shift_minutes": actualMinutes,
			"expected_output":      expectedOutput,
			"performance_output":   performanceOutput,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"line":          line,
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}

// Ambil main_speed terakhir dalam shift
func getLastMainSpeed(line string, start, end, now time.Time) int64 {
	if now.Before(start) {
		return 0
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	startStr := start.In(loc).Format("2006-01-02 15:04:05")
	endStr := end.In(loc).Format("2006-01-02 15:04:05")

	tableName := getTableByLine(line)
	var record struct {
		MainSpeed int `json:"main_speed"`
	}

	query := fmt.Sprintf("SELECT main_speed FROM %s WHERE ts >= ? AND ts <= ? ORDER BY ts DESC LIMIT 1", tableName)
	result := config.DB.Raw(query, startStr, endStr).Scan(&record)

	if result.Error != nil {
		fmt.Printf("Error getting main speed for %s: %v\n", line, result.Error)
		return 0
	}

	return int64(record.MainSpeed)
}

// Controller untuk Output Gagal Filling
func OutputGagalFilling(c *gin.Context) {
	line := c.Param("line")
	dateParam := c.Query("date")

	// Validasi line
	if getModelByLine(line) == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Line %s tidak valid. Gunakan d1-d14", line)})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")

	baseDate := time.Now().In(loc)
	if dateParam != "" {
		parsedDate, err := time.Parse("2006-01-02", dateParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal salah. Gunakan YYYY-MM-DD"})
			return
		}
		baseDate = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, loc)
	}

	now := time.Now().In(loc)
	var shifts []gin.H

	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)

		totalCounter := getLatestTotalCounter(line, start, end, now)
		runtimeMinutes := getShiftRuntime(line, start, end, now) // akumulasi start_mesin = 1 dalam menit
		mainSpeed := getLastMainSpeed(line, start, end, now)

		var goodFilling, gagalFilling float64
		if runtimeMinutes > 0 && mainSpeed > 0 {
			denom := float64(runtimeMinutes) * float64(mainSpeed) * 2
			goodFilling = (float64(totalCounter) / denom) * 100
			if goodFilling > 100 {
				goodFilling = 100 // jangan lebih dari 100%
			}
			gagalFilling = 100 - goodFilling
		}

		shifts = append(shifts, gin.H{
			"shift":           i,
			"start_time":      start,
			"end_time":        end,
			"total_counter":   totalCounter,
			"runtime_minutes": runtimeMinutes,
			"main_speed":      mainSpeed,
			"good_filling":    goodFilling,
			"gagal_filling":   gagalFilling,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":          baseDate.Format("2006-01-02"),
		"line":          line,
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}