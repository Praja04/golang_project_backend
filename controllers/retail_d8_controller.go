package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"backend-golang/config"
	"backend-golang/models"
)

// Ambil total runtime (start_mesin = 1) dari DB
func getShiftRuntime(start, end, now time.Time) int64 {
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
	fmt.Printf("Query DB Runtime - Start: %s, End: %s\n", startStr, endStr)
	
	var countSeconds int64
	result := config.DB.Model(&models.RetailD8{}).
		Where("start_mesin = ? AND ts >= ? AND ts <= ?", 1, startStr, endStr).
		Count(&countSeconds)

	if result.Error != nil {
		fmt.Println("DB Error:", result.Error)
		return 0
	}

	fmt.Printf("DB Result Runtime - Count: %d seconds (%d minutes)\n", countSeconds, countSeconds/60)
	
	return countSeconds / 60 // convert detik → menit
}

// Ambil total stoptime (start_mesin = 0) dari DB
func getShiftStoptime(start, end, now time.Time) int64 {
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
	fmt.Printf("Query DB Stoptime - Start: %s, End: %s\n", startStr, endStr)
	
	var countSeconds int64
	result := config.DB.Model(&models.RetailD8{}).
		Where("start_mesin = ? AND ts >= ? AND ts <= ?", 0, startStr, endStr).
		Count(&countSeconds)

	if result.Error != nil {
		fmt.Println("DB Error:", result.Error)
		return 0
	}

	fmt.Printf("DB Result Stoptime - Count: %d seconds (%d minutes)\n", countSeconds, countSeconds/60)
	
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
	return baseDate, baseDate
}

// Tentukan shift sekarang
func getCurrentShift(now time.Time) int {
	hour, min := now.Hour(), now.Minute()
	if hour >= 6 && (hour < 14 || (hour == 14 && min == 0)) {
		return 1
	} else if (hour > 14 || (hour == 14 && min >= 1)) && hour < 22 {
		return 2
	}
	return 3
}

// Controller untuk Uptime (Runtime)
func UptimeStartMesinRealtime(c *gin.Context) {
	dateParam := c.Query("date")

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
	fmt.Printf("Current time (Asia/Jakarta): %v\n", now)
	
	var shifts []gin.H

	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)
		
		// Debug: print shift times
		fmt.Printf("Shift %d: Start=%v, End=%v\n", i, start, end)

		runtimeMinutes := getShiftRuntime(start, end, now)
		actualMinutes := getActualShiftMinutes(start, end, now)
		
		// Debug: print calculations
		fmt.Printf("Shift %d: Runtime=%d, Actual=%d\n", i, runtimeMinutes, actualMinutes)

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
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}

// Controller untuk Downtime (Stoptime)
func DowntimeStopMesinRealtime(c *gin.Context) {
	dateParam := c.Query("date")

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
	fmt.Printf("Current time (Asia/Jakarta): %v\n", now)
	
	var shifts []gin.H

	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)
		
		// Debug: print shift times
		fmt.Printf("Shift %d: Start=%v, End=%v\n", i, start, end)

		downtimeMinutes := getShiftStoptime(start, end, now)
		actualMinutes := getActualShiftMinutes(start, end, now)
		
		// Debug: print calculations
		fmt.Printf("Shift %d: Downtime=%d, Actual=%d\n", i, downtimeMinutes, actualMinutes)

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
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}

// Optimized getLatestTotalCounter (1 query saja)
func getLatestTotalCounter(start, end, now time.Time) int64 {
	if now.Before(start) {
		return 0
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	startStr := start.In(loc).Format("2006-01-02 15:04:05")
	endStr := end.In(loc).Format("2006-01-02 15:04:05")

	var records []models.RetailD8
	result := config.DB.Model(&models.RetailD8{}).
		Where("ts >= ? AND ts <= ?", startStr, endStr).
		Order("ts ASC").
		Select("ts, total_counter").
		Find(&records)

	if result.Error != nil || len(records) == 0 {
		return 0
	}

	var lastNonZero int64 = 0
	for _, r := range records {
		if r.TotalCounter > 0 {
			lastNonZero = int64(r.TotalCounter)
		} else if r.TotalCounter == 0 && lastNonZero > 0 {
			// begitu ketemu nol setelah ada angka >0, stop
			break
		}
	}

	return lastNonZero
}


// Controller untuk Performance Output (optimized)
func PerformanceOutput(c *gin.Context) {
	dateParam := c.Query("date")
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

		totalCounter := getLatestTotalCounter(start, end, now)
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
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
	
}

// Ambil main_speed terakhir dalam shift
func getLastMainSpeed(start, end, now time.Time) int64 {
	if now.Before(start) {
		return 0
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	startStr := start.In(loc).Format("2006-01-02 15:04:05")
	endStr := end.In(loc).Format("2006-01-02 15:04:05")

	var record models.RetailD8
	result := config.DB.Model(&models.RetailD8{}).
		Where("ts >= ? AND ts <= ?", startStr, endStr).
		Order("ts DESC").
		Select("main_speed").
		First(&record)

	if result.Error != nil {
		return 0
	}

	return int64(record.MainSpeed)
}

// Controller untuk Output Gagal Filling
func OutputGagalFilling(c *gin.Context) {
	dateParam := c.Query("date")
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

		totalCounter := getLatestTotalCounter(start, end, now)
		runtimeMinutes := getShiftRuntime(start, end, now) // akumulasi start_mesin = 1 dalam menit
		mainSpeed := getLastMainSpeed(start, end, now)

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
			"shift":          i,
			"start_time":     start,
			"end_time":       end,
			"total_counter":  totalCounter,
			"runtime_minutes": runtimeMinutes,
			"main_speed":     mainSpeed,
			"good_filling":   goodFilling,
			"gagal_filling":  gagalFilling,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}
