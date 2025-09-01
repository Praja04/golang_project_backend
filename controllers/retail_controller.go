package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"backend-golang/config"
	"backend-golang/models"
)

// ======================= MODEL DYNAMIC =======================
func getRetailModel(line string) interface{} {
	switch line {
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

// ======================= SHIFT HELPERS =======================
func getShiftRange(baseDate time.Time, shift int) (time.Time, time.Time) {
	loc := baseDate.Location()
	switch shift {
	case 1:
		return time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 6, 0, 0, 0, loc),
			time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 14, 0, 0, 0, loc)
	case 2:
		return time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 14, 1, 0, 0, loc),
			time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 22, 0, 0, 0, loc)
	case 3:
		return time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 22, 1, 0, 0, loc),
			time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day()+1, 5, 59, 59, 0, loc)
	default:
		return baseDate, baseDate
	}
}

func getCurrentShift(now time.Time) int {
	hour := now.Hour()
	switch {
	case hour >= 6 && hour < 14:
		return 1
	case hour >= 14 && hour < 22:
		return 2
	default:
		return 3
	}
}

func getActualShiftMinutes(start, end, now time.Time) int64 {
	var actual int64
	if now.Before(start) {
		actual = 0
	} else if now.After(end) {
		actual = int64(end.Sub(start).Minutes())
	} else {
		actual = int64(now.Sub(start).Minutes())
	}

	if start.Weekday() == time.Saturday && actual > 300 {
		actual = 300
	} else if actual > 420 {
		actual = 420
	}

	return actual
}

// ======================= DB HELPERS =======================
func getShiftRuntime(model interface{}, start, end, now time.Time) int64 {
	if now.Before(start) {
		return 0
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	startStr := start.In(loc).Format("2006-01-02 15:04:05")
	endStr := end.In(loc).Format("2006-01-02 15:04:05")

	var count int64
	result := config.DB.Model(model).Where("start_mesin = ? AND ts >= ? AND ts <= ?", 1, startStr, endStr).Count(&count)
	if result.Error != nil {
		fmt.Println("DB Error:", result.Error)
		return 0
	}
	return count / 60
}

func getShiftStoptime(model interface{}, start, end, now time.Time) int64 {
	if now.Before(start) {
		return 0
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	startStr := start.In(loc).Format("2006-01-02 15:04:05")
	endStr := end.In(loc).Format("2006-01-02 15:04:05")

	var count int64
	result := config.DB.Model(model).Where("start_mesin = ? AND ts >= ? AND ts <= ?", 0, startStr, endStr).Count(&count)
	if result.Error != nil {
		fmt.Println("DB Error:", result.Error)
		return 0
	}
	return count / 60
}

func getLatestTotalCounter(model interface{}, start, end, now time.Time) int64 {
	if now.Before(start) {
		return 0
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	startStr := start.In(loc).Format("2006-01-02 15:04:05")
	endStr := end.In(loc).Format("2006-01-02 15:04:05")

	var records []map[string]interface{}
	result := config.DB.Model(model).Where("ts >= ? AND ts <= ?", startStr, endStr).Order("ts ASC").Select("ts, total_counter").Find(&records)
	if result.Error != nil || len(records) == 0 {
		return 0
	}

	var last int64
	for _, r := range records {
		if v, ok := r["total_counter"].(int64); ok && v > 0 {
			last = v
		}
	}
	return last
}

func getLastMainSpeed(model interface{}, start, end, now time.Time) int64 {
	if now.Before(start) {
		return 0
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	startStr := start.In(loc).Format("2006-01-02 15:04:05")
	endStr := end.In(loc).Format("2006-01-02 15:04:05")

	var record map[string]interface{}
	result := config.DB.Model(model).Where("ts >= ? AND ts <= ?", startStr, endStr).Order("ts DESC").Select("main_speed").First(&record)
	if result.Error != nil {
		return 0
	}
	if v, ok := record["main_speed"].(int64); ok {
		return v
	}
	return 0
}

// ======================= CONTROLLERS =======================

func UptimeStartMesinRealtime(c *gin.Context) {
	line := c.Param("line")
	model := getRetailModel(line)
	if model == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Line tidak valid"})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	baseDate := time.Now().In(loc)
	if dateParam := c.Query("date"); dateParam != "" {
		if d, err := time.Parse("2006-01-02", dateParam); err == nil {
			baseDate = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
		}
	}

	now := time.Now().In(loc)
	var shifts []gin.H
	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)
		runtime := getShiftRuntime(model, start, end, now)
		actual := getActualShiftMinutes(start, end, now)
		uptime := 0.0
		if actual > 0 {
			uptime = float64(runtime) / float64(actual) * 100
		}
		shifts = append(shifts, gin.H{
			"shift":                 i,
			"start_time":            start,
			"end_time":              end,
			"runtime_total_minutes": runtime,
			"actual_shift_minutes":  actual,
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

func DowntimeStopMesinRealtime(c *gin.Context) {
	line := c.Param("line")
	model := getRetailModel(line)
	if model == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Line tidak valid"})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	baseDate := time.Now().In(loc)
	now := time.Now().In(loc)
	var shifts []gin.H

	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)
		downtime := getShiftStoptime(model, start, end, now)
		actual := getActualShiftMinutes(start, end, now)
		downtimePct := 0.0
		if actual > 0 {
			downtimePct = float64(downtime) / float64(actual) * 100
		}
		shifts = append(shifts, gin.H{
			"shift":                  i,
			"start_time":             start,
			"end_time":               end,
			"downtime_total_minutes": downtime,
			"actual_shift_minutes":   actual,
			"downtime":               downtimePct,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"line":          line,
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}

func PerformanceOutput(c *gin.Context) {
	line := c.Param("line")
	model := getRetailModel(line)
	if model == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Line tidak valid"})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	baseDate := time.Now().In(loc)
	if dateParam := c.Query("date"); dateParam != "" {
		if d, err := time.Parse("2006-01-02", dateParam); err == nil {
			baseDate = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
		}
	}

	now := time.Now().In(loc)
	var shifts []gin.H
	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)
		total := getLatestTotalCounter(model, start, end, now)
		actual := getActualShiftMinutes(start, end, now)
		expected := int64(0)
		perf := 0.0
		if actual > 0 {
			expected = actual * 40 * 2
			perf = float64(total) / float64(expected) * 100
		}
		shifts = append(shifts, gin.H{
			"shift":                i,
			"start_time":           start,
			"end_time":             end,
			"total_counter":        total,
			"actual_shift_minutes": actual,
			"expected_output":      expected,
			"performance_output":   perf,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"line":          line,
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}

func OutputGagalFilling(c *gin.Context) {
	line := c.Param("line")
	model := getRetailModel(line)
	if model == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Line tidak valid"})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	baseDate := time.Now().In(loc)
	if dateParam := c.Query("date"); dateParam != "" {
		if d, err := time.Parse("2006-01-02", dateParam); err == nil {
			baseDate = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
		}
	}

	now := time.Now().In(loc)
	var shifts []gin.H
	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)
		total := getLatestTotalCounter(model, start, end, now)
		runtime := getShiftRuntime(model, start, end, now)
		mainSpeed := getLastMainSpeed(model, start, end, now)

		var good, gagal float64
		if runtime > 0 && mainSpeed > 0 {
			denom := float64(runtime) * float64(mainSpeed) * 2
			good = float64(total) / denom * 100
			if good > 100 {
				good = 100
			}
			gagal = 100 - good
		}

		shifts = append(shifts, gin.H{
			"shift":           i,
			"start_time":      start,
			"end_time":        end,
			"total_counter":   total,
			"runtime_minutes": runtime,
			"main_speed":      mainSpeed,
			"good_filling":    good,
			"gagal_filling":   gagal,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"line":          line,
		"date":          baseDate.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}
