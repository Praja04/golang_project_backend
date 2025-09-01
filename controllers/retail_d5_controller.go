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
	result := config.DB.Model(&models.RetailD5{}).
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
	result := config.DB.Model(&models.RetailD5{}).
		Where("start_mesin = ? AND ts >= ? AND ts <= ?", 0, startStr, endStr).
		Count(&countSeconds)

	if result.Error != nil {
		fmt.Println("DB Error:", result.Error)
		return 0
	}

	fmt.Printf("DB Result Stoptime - Count: %d seconds (%d minutes)\n", countSeconds, countSeconds/60)
	
	return countSeconds / 60 // convert detik → menit
}




// Hitung actual shift minutes (sampai "now") → khusus pakai Asia/Jakarta
func getActualShiftMinutes(start, end, now time.Time) int64 {
	// Pastikan semua waktu dalam timezone yang sama
	loc, _ := time.LoadLocation("Asia/Jakarta")
	start = start.In(loc)
	end = end.In(loc)
	now = now.In(loc)

	var actualMinutes int64
	
	if now.Before(start) {
		// Shift belum dimulai
		actualMinutes = 0
	} else if now.After(end) {
		// Shift sudah selesai, hitung full duration
		actualMinutes = int64(end.Sub(start).Minutes())
	} else {
		// Shift sedang berjalan, hitung dari start sampai now
		actualMinutes = int64(now.Sub(start).Minutes())
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

// Enhanced debugging function
func debugDatabaseContent(start, end time.Time) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	startLocal := start.In(loc)
	endLocal := end.In(loc)
	
	startStr := startLocal.Format("2006-01-02 15:04:05")
	endStr := endLocal.Format("2006-01-02 15:04:05")
	
	fmt.Printf("=== DEBUG DATABASE CONTENT ===\n")
	fmt.Printf("Query range: %s to %s\n", startStr, endStr)
	
	// Cek total records di database
	var totalCount int64
	config.DB.Model(&models.RetailD5{}).Count(&totalCount)
	fmt.Printf("Total records in database: %d\n", totalCount)
	
	// Cek records dalam range tanpa filter
	var rangeCount int64
	config.DB.Model(&models.RetailD5{}).
		Where("ts >= ? AND ts <= ?", startStr, endStr).
		Count(&rangeCount)
	fmt.Printf("Records in exact range: %d\n", rangeCount)
	
	// Cek dengan LIKE pattern untuk hari ini
	today := time.Now().In(loc).Format("2006-01-02")
	var todayCount int64
	config.DB.Model(&models.RetailD5{}).
		Where("ts LIKE ?", today+"%").
		Count(&todayCount)
	fmt.Printf("Records for today (%s): %d\n", today, todayCount)
	
	// Sample beberapa record terakhir hari ini
	var todayRecords []models.RetailD5
	config.DB.Model(&models.RetailD5{}).
		Where("ts LIKE ?", today+"%").
		Order("ts DESC").
		Limit(3).
		Find(&todayRecords)
	
	fmt.Printf("Latest records today:\n")
	for i, record := range todayRecords {
		fmt.Printf("  [%d] Ts: %s, TotalCounter: %d, StartMesin: %d\n", 
			i+1, record.Ts, record.TotalCounter, record.StartMesin)
	}
	
	fmt.Printf("=== END DEBUG ===\n")
}

// Enhanced getLatestTotalCounter dengan debug lebih detail
func getLatestTotalCounter(start, end, now time.Time) int64 {
	// Jika shift belum dimulai, return 0
	if now.Before(start) {
		fmt.Printf("Shift belum dimulai. Now: %v, Start: %v\n", now, start)
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
	fmt.Printf("Query DB Latest Counter - Start: %s, End: %s\n", startStr, endStr)
	
	// Enhanced debugging
	debugDatabaseContent(start, end)
	
	// Try multiple query strategies
	
	// Strategy 1: Original logic
	var zeroRecord models.RetailD5
	zeroResult := config.DB.Model(&models.RetailD5{}).
		Where("ts >= ? AND ts <= ? AND total_counter = ?", startStr, endStr, 0).
		Order("ts ASC").
		First(&zeroRecord)

	if zeroResult.Error == nil {
		fmt.Printf("Found zero counter at: %s\n", zeroRecord.Ts)
		
		var lastNonZeroRecord models.RetailD5
		nonZeroResult := config.DB.Model(&models.RetailD5{}).
			Where("ts >= ? AND ts < ? AND total_counter > ?", startStr, zeroRecord.Ts, 0).
			Order("ts DESC").
			First(&lastNonZeroRecord)

		if nonZeroResult.Error == nil {
			fmt.Printf("DB Result - Latest total_counter before zero: %d at %s\n", 
				lastNonZeroRecord.TotalCounter, lastNonZeroRecord.Ts)
			return int64(lastNonZeroRecord.TotalCounter)
		} else {
			fmt.Printf("No non-zero data found before zero value. Error: %v\n", nonZeroResult.Error)
		}
	}
	
	// Strategy 2: Ambil data terakhir > 0 dalam shift
	var latestRecord models.RetailD5
	result := config.DB.Model(&models.RetailD5{}).
		Where("ts >= ? AND ts <= ? AND total_counter > ?", startStr, endStr, 0).
		Order("ts DESC").
		First(&latestRecord)

	if result.Error == nil {
		fmt.Printf("DB Result - Latest total_counter (>0): %d at %s\n", 
			latestRecord.TotalCounter, latestRecord.Ts)
		return int64(latestRecord.TotalCounter)
	}
	
	// Strategy 3: Ambil data terakhir tanpa filter total_counter
	var anyRecord models.RetailD5
	anyResult := config.DB.Model(&models.RetailD5{}).
		Where("ts >= ? AND ts <= ?", startStr, endStr).
		Order("ts DESC").
		First(&anyRecord)
		
	if anyResult.Error == nil {
		fmt.Printf("DB Result - Latest record (any): %d at %s\n", 
			anyRecord.TotalCounter, anyRecord.Ts)
		return int64(anyRecord.TotalCounter)
	}
	
	// Strategy 4: Coba dengan LIKE pattern untuk hari tersebut
	dateStr := startLocal.Format("2006-01-02")
	var dayRecord models.RetailD5
	dayResult := config.DB.Model(&models.RetailD5{}).
		Where("ts LIKE ?", dateStr+"%").
		Order("ts DESC").
		First(&dayRecord)
		
	if dayResult.Error == nil {
		fmt.Printf("DB Result - Latest record for day: %d at %s\n", 
			dayRecord.TotalCounter, dayRecord.Ts)
		return int64(dayRecord.TotalCounter)
	}
	
	fmt.Printf("No data found with any strategy. Last error: %v\n", dayResult.Error)
	return 0
}

// Controller untuk Performance Output dengan enhanced debugging
func PerformanceOutput(c *gin.Context) {
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
	
	// Debug: print current time dan base date
	fmt.Printf("Current time (Asia/Jakarta): %v\n", now)
	fmt.Printf("Base date for query: %v\n", baseDate)
	
	var shifts []gin.H

	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)
		
		// Debug: print shift times
		fmt.Printf("\n=== SHIFT %d ===\n", i)
		fmt.Printf("Start: %v\n", start)
		fmt.Printf("End: %v\n", end)
		fmt.Printf("Now: %v\n", now)

		totalCounter := getLatestTotalCounter(start, end, now)
		actualMinutes := getActualShiftMinutes(start, end, now)
		
		// Debug: print calculations
		fmt.Printf("TotalCounter: %d\n", totalCounter)
		fmt.Printf("ActualMinutes: %d\n", actualMinutes)

		// Rumus: performance_output = total_counter / (actualshiftminutes x 40 x 2)
		performanceOutput := 0.0
		expectedOutput := int64(0)
		if actualMinutes > 0 {
			expectedOutput = actualMinutes * 40 * 2 // target output per menit
			performanceOutput = float64(totalCounter) / float64(expectedOutput) * 100 // dalam persen
		}

		fmt.Printf("ExpectedOutput: %d\n", expectedOutput)
		fmt.Printf("PerformanceOutput: %.2f%%\n", performanceOutput)

		shifts = append(shifts, gin.H{
			"shift":               i,
			"start_time":          start,
			"end_time":            end,
			"total_counter":       totalCounter,
			"actual_shift_minutes": actualMinutes,
			"expected_output":     expectedOutput,
			"performance_output":  performanceOutput,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}

// Test endpoint untuk melihat raw data
func DebugDatabaseRaw(c *gin.Context) {
	dateParam := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	
	// Sample queries untuk debugging
	var records []models.RetailD5
	
	// Query 1: All records for the date
	config.DB.Model(&models.RetailD5{}).
		Where("ts LIKE ?", dateParam+"%").
		Order("ts DESC").
		Limit(10).
		Find(&records)
	
	var response []gin.H
	for _, record := range records {
		response = append(response, gin.H{
			"ts":            record.Ts,
			"total_counter": record.TotalCounter,
			"start_mesin":   record.StartMesin,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{
		"date":    dateParam,
		"count":   len(records),
		"records": response,
	})
}