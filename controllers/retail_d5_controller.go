package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"backend-golang/config"
	"backend-golang/models"
)

// Ambil total runtime (full shift) dari DB
func getShiftRuntime(start, end, now time.Time) int64 {
	// Jika shift belum dimulai, return 0
	if now.Before(start) {
		return 0
	}
	
	var countSeconds int64
	result := config.DB.Model(&models.RetailD5{}).
		Where("start_mesin = ? AND ts >= ? AND ts <= ?", 1, start, end).
		Count(&countSeconds)

	if result.Error != nil {
		fmt.Println("DB Error:", result.Error)
		return 0
	}

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

// Controller utama
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