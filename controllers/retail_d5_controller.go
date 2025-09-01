package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"backend-golang/config"
	"backend-golang/models"
)

// Struct untuk aggregate query results
type ShiftStats struct {
	RuntimeMinutes  int64 `gorm:"column:runtime_minutes"`
	StoptimeMinutes int64 `gorm:"column:stoptime_minutes"`
	LatestCounter   int64 `gorm:"column:latest_counter"`
}

// Single query untuk ambil semua data shift sekaligus
func getShiftStatsOptimized(start, end time.Time) ShiftStats {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	startLocal := start.In(loc)
	endLocal := end.In(loc)
	
	startStr := startLocal.Format("2006-01-02 15:04:05")
	endStr := endLocal.Format("2006-01-02 15:04:05")
	
	var stats ShiftStats
	
	// Single raw SQL query untuk performa maksimal
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN start_mesin = 1 THEN 1 ELSE 0 END), 0) / 60 as runtime_minutes,
			COALESCE(SUM(CASE WHEN start_mesin = 0 THEN 1 ELSE 0 END), 0) / 60 as stoptime_minutes,
			COALESCE(MAX(CASE WHEN total_counter > 0 THEN total_counter ELSE 0 END), 0) as latest_counter
		FROM retail_d5 
		WHERE ts >= ? AND ts <= ?
	`
	
	config.DB.Raw(query, startStr, endStr).Scan(&stats)
	return stats
}

// Optimized version tanpa debug prints
func getActualShiftMinutesOptimized(start, end, now time.Time) int64 {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	start = start.In(loc)
	end = end.In(loc)
	now = now.In(loc)

	if now.Before(start) {
		return 0
	} else if now.After(end) {
		return int64(end.Sub(start).Minutes())
	} else {
		return int64(now.Sub(start).Minutes())
	}
}

func getShiftRangeOptimized(baseDate time.Time, shift int) (time.Time, time.Time) {
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

func getCurrentShiftOptimized(now time.Time) int {
	hour, min := now.Hour(), now.Minute()
	if hour >= 6 && (hour < 14 || (hour == 14 && min == 0)) {
		return 1
	} else if (hour > 14 || (hour == 14 && min >= 1)) && hour < 22 {
		return 2
	}
	return 3
}

// Ultra-fast uptime controller
func UptimeStartMesinRealtimeOptimized(c *gin.Context) {
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

	// Process all shifts at once
	for i := 1; i <= 3; i++ {
		start, end := getShiftRangeOptimized(baseDate, i)
		stats := getShiftStatsOptimized(start, end)
		actualMinutes := getActualShiftMinutesOptimized(start, end, now)
		
		uptime := 0.0
		if actualMinutes > 0 {
			uptime = float64(stats.RuntimeMinutes) / float64(actualMinutes) * 100
		}

		shifts = append(shifts, gin.H{
			"shift":                 i,
			"start_time":            start,
			"end_time":              end,
			"runtime_total_minutes": stats.RuntimeMinutes,
			"actual_shift_minutes":  actualMinutes,
			"uptime":                uptime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShiftOptimized(now),
		"shifts":        shifts,
	})
}

// Ultra-fast downtime controller
func DowntimeStopMesinRealtimeOptimized(c *gin.Context) {
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
		start, end := getShiftRangeOptimized(baseDate, i)
		stats := getShiftStatsOptimized(start, end)
		actualMinutes := getActualShiftMinutesOptimized(start, end, now)
		
		downtime := 0.0
		if actualMinutes > 0 {
			downtime = float64(stats.StoptimeMinutes) / float64(actualMinutes) * 100
		}

		shifts = append(shifts, gin.H{
			"shift":                  i,
			"start_time":             start,
			"end_time":               end,
			"downtime_total_minutes": stats.StoptimeMinutes,
			"actual_shift_minutes":   actualMinutes,
			"downtime":               downtime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShiftOptimized(now),
		"shifts":        shifts,
	})
}

// Ultra-fast performance output controller
func PerformanceOutputOptimized(c *gin.Context) {
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
		start, end := getShiftRangeOptimized(baseDate, i)
		stats := getShiftStatsOptimized(start, end)
		actualMinutes := getActualShiftMinutesOptimized(start, end, now)
		
		performanceOutput := 0.0
		expectedOutput := int64(0)
		if actualMinutes > 0 {
			expectedOutput = actualMinutes * 40 * 2
			performanceOutput = float64(stats.LatestCounter) / float64(expectedOutput) * 100
		}

		shifts = append(shifts, gin.H{
			"shift":               i,
			"start_time":          start,
			"end_time":            end,
			"total_counter":       stats.LatestCounter,
			"actual_shift_minutes": actualMinutes,
			"expected_output":     expectedOutput,
			"performance_output":  performanceOutput,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShiftOptimized(now),
		"shifts":        shifts,
	})
}

// Batch version - process semua shift dalam 1 query
func AllShiftsStatsOptimized(c *gin.Context) {
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
	
	// Get all day data in one query
	dayStart := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 0, 0, 0, 0, loc)
	dayEnd := time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day()+1, 23, 59, 59, 0, loc)
	
	dayStartStr := dayStart.Format("2006-01-02 15:04:05")
	dayEndStr := dayEnd.Format("2006-01-02 15:04:05")
	
	// Super optimized single query untuk semua data shift
	type AllShiftStats struct {
		Shift1Runtime   int64 `gorm:"column:shift1_runtime"`
		Shift1Stoptime  int64 `gorm:"column:shift1_stoptime"`
		Shift1Counter   int64 `gorm:"column:shift1_counter"`
		Shift2Runtime   int64 `gorm:"column:shift2_runtime"`
		Shift2Stoptime  int64 `gorm:"column:shift2_stoptime"`
		Shift2Counter   int64 `gorm:"column:shift2_counter"`
		Shift3Runtime   int64 `gorm:"column:shift3_runtime"`
		Shift3Stoptime  int64 `gorm:"column:shift3_stoptime"`
		Shift3Counter   int64 `gorm:"column:shift3_counter"`
	}
	
	var allStats AllShiftStats
	
	batchQuery := `
		SELECT 
			-- Shift 1 (06:00-14:00)
			COALESCE(SUM(CASE WHEN TIME(ts) >= '06:00:00' AND TIME(ts) <= '14:00:00' AND start_mesin = 1 THEN 1 ELSE 0 END), 0) / 60 as shift1_runtime,
			COALESCE(SUM(CASE WHEN TIME(ts) >= '06:00:00' AND TIME(ts) <= '14:00:00' AND start_mesin = 0 THEN 1 ELSE 0 END), 0) / 60 as shift1_stoptime,
			COALESCE(MAX(CASE WHEN TIME(ts) >= '06:00:00' AND TIME(ts) <= '14:00:00' AND total_counter > 0 THEN total_counter ELSE 0 END), 0) as shift1_counter,
			
			-- Shift 2 (14:01-22:00)
			COALESCE(SUM(CASE WHEN TIME(ts) >= '14:01:00' AND TIME(ts) <= '22:00:00' AND start_mesin = 1 THEN 1 ELSE 0 END), 0) / 60 as shift2_runtime,
			COALESCE(SUM(CASE WHEN TIME(ts) >= '14:01:00' AND TIME(ts) <= '22:00:00' AND start_mesin = 0 THEN 1 ELSE 0 END), 0) / 60 as shift2_stoptime,
			COALESCE(MAX(CASE WHEN TIME(ts) >= '14:01:00' AND TIME(ts) <= '22:00:00' AND total_counter > 0 THEN total_counter ELSE 0 END), 0) as shift2_counter,
			
			-- Shift 3 (22:01-05:59 next day)
			COALESCE(SUM(CASE WHEN (TIME(ts) >= '22:01:00' OR TIME(ts) <= '05:59:59') AND start_mesin = 1 THEN 1 ELSE 0 END), 0) / 60 as shift3_runtime,
			COALESCE(SUM(CASE WHEN (TIME(ts) >= '22:01:00' OR TIME(ts) <= '05:59:59') AND start_mesin = 0 THEN 1 ELSE 0 END), 0) / 60 as shift3_stoptime,
			COALESCE(MAX(CASE WHEN (TIME(ts) >= '22:01:00' OR TIME(ts) <= '05:59:59') AND total_counter > 0 THEN total_counter ELSE 0 END), 0) as shift3_counter
		FROM retail_d5 
		WHERE ts >= ? AND ts <= ?
	`
	
	config.DB.Raw(batchQuery, dayStartStr, dayEndStr).Scan(&allStats)
	
	return allStats
}

// Super fast uptime controller - 1 query only
func UptimeStartMesinRealtimeFast(c *gin.Context) {
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
	
	// Single query untuk semua data
	allStats := getShiftStatsOptimized(
		time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 6, 0, 0, 0, loc),
		time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day()+1, 5, 59, 59, 0, loc),
	)
	
	// Extract data per shift
	runtimeData := []int64{allStats.Shift1Runtime, allStats.Shift2Runtime, allStats.Shift3Runtime}
	
	var shifts []gin.H
	for i := 1; i <= 3; i++ {
		start, end := getShiftRangeOptimized(baseDate, i)
		actualMinutes := getActualShiftMinutesOptimized(start, end, now)
		
		uptime := 0.0
		if actualMinutes > 0 {
			uptime = float64(runtimeData[i-1]) / float64(actualMinutes) * 100
		}

		shifts = append(shifts, gin.H{
			"shift":                 i,
			"start_time":            start,
			"end_time":              end,
			"runtime_total_minutes": runtimeData[i-1],
			"actual_shift_minutes":  actualMinutes,
			"uptime":                uptime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShiftOptimized(now),
		"shifts":        shifts,
	})
}

// Super fast downtime controller - 1 query only
func DowntimeStopMesinRealtimeFast(c *gin.Context) {
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
	
	// Single query untuk semua data
	allStats := getShiftStatsOptimized(
		time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 6, 0, 0, 0, loc),
		time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day()+1, 5, 59, 59, 0, loc),
	)
	
	// Extract data per shift
	stoptimeData := []int64{allStats.Shift1Stoptime, allStats.Shift2Stoptime, allStats.Shift3Stoptime}
	
	var shifts []gin.H
	for i := 1; i <= 3; i++ {
		start, end := getShiftRangeOptimized(baseDate, i)
		actualMinutes := getActualShiftMinutesOptimized(start, end, now)
		
		downtime := 0.0
		if actualMinutes > 0 {
			downtime = float64(stoptimeData[i-1]) / float64(actualMinutes) * 100
		}

		shifts = append(shifts, gin.H{
			"shift":                  i,
			"start_time":             start,
			"end_time":               end,
			"downtime_total_minutes": stoptimeData[i-1],
			"actual_shift_minutes":   actualMinutes,
			"downtime":               downtime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShiftOptimized(now),
		"shifts":        shifts,
	})
}

// Super fast performance controller - 1 query only
func PerformanceOutputFast(c *gin.Context) {
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
	
	// Single query untuk semua data
	allStats := getShiftStatsOptimized(
		time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 6, 0, 0, 0, loc),
		time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day()+1, 5, 59, 59, 0, loc),
	)
	
	// Extract data per shift
	counterData := []int64{allStats.Shift1Counter, allStats.Shift2Counter, allStats.Shift3Counter}
	
	var shifts []gin.H
	for i := 1; i <= 3; i++ {
		start, end := getShiftRangeOptimized(baseDate, i)
		actualMinutes := getActualShiftMinutesOptimized(start, end, now)
		
		performanceOutput := 0.0
		expectedOutput := int64(0)
		if actualMinutes > 0 {
			expectedOutput = actualMinutes * 40 * 2
			performanceOutput = float64(counterData[i-1]) / float64(expectedOutput) * 100
		}

		shifts = append(shifts, gin.H{
			"shift":               i,
			"start_time":          start,
			"end_time":            end,
			"total_counter":       counterData[i-1],
			"actual_shift_minutes": actualMinutes,
			"expected_output":     expectedOutput,
			"performance_output":  performanceOutput,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShiftOptimized(now),
		"shifts":        shifts,
	})
}