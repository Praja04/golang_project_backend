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
func getShiftRuntime(start, end time.Time) int64 {
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
	loc := baseDate.Location()
	start = start.In(loc)
	end = end.In(loc)
	now = now.In(loc)

	if now.Before(start) {
	return 0
	} else if now.After(end) {
		return int64(end.Sub(start).Minutes()) // full shift duration
	} else {
		return int64(now.Sub(start).Minutes()) // partial shift
	}
	return int64(now.Sub(start).Minutes())
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

	// default pakai UTC
	baseDate := time.Now().UTC()
	var err error

	if dateParam != "" {
		baseDate, err = time.Parse("2006-01-02", dateParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal salah. Gunakan YYYY-MM-DD"})
			return
		}
	}

	now := time.Now().UTC()
	var shifts []gin.H

	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(baseDate, i)

		runtimeMinutes := getShiftRuntime(start, end)
		actualMinutes := getActualShiftMinutes(start, end, now)

		uptime := 0.0
		if actualMinutes > 0 {
			uptime = float64(runtimeMinutes) / float64(actualMinutes)
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
