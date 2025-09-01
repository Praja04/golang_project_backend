package controllers

import (
	"backend-golang/config"
	"backend-golang/models"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type ShiftInfo struct {
	Shift     int       `json:"shift"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Runtime   int64     `json:"runtime_total_minutes"`
	Actual    int64     `json:"actual_shift_minutes"`
	Uptime    float64   `json:"uptime"`
}

// Tentukan rentang shift
func getShiftRange(date time.Time, shift int) (time.Time, time.Time) {
	loc := date.Location()
	isSaturday := date.Weekday() == time.Saturday

	switch {
	case isSaturday && shift == 1:
		return time.Date(date.Year(), date.Month(), date.Day(), 6, 0, 0, 0, loc),
			time.Date(date.Year(), date.Month(), date.Day(), 11, 0, 0, 0, loc)
	case isSaturday && shift == 2:
		return time.Date(date.Year(), date.Month(), date.Day(), 11, 1, 0, 0, loc),
			time.Date(date.Year(), date.Month(), date.Day(), 16, 0, 0, 0, loc)
	case isSaturday && shift == 3:
		return time.Date(date.Year(), date.Month(), date.Day(), 16, 1, 0, 0, loc),
			time.Date(date.Year(), date.Month(), date.Day(), 21, 0, 0, 0, loc)
	case !isSaturday && shift == 1:
		return time.Date(date.Year(), date.Month(), date.Day(), 6, 0, 0, 0, loc),
			time.Date(date.Year(), date.Month(), date.Day(), 14, 0, 0, 0, loc)
	case !isSaturday && shift == 2:
		return time.Date(date.Year(), date.Month(), date.Day(), 14, 1, 0, 0, loc),
			time.Date(date.Year(), date.Month(), date.Day(), 22, 0, 0, 0, loc)
	case !isSaturday && shift == 3:
		return time.Date(date.Year(), date.Month(), date.Day(), 22, 1, 0, 0, loc),
			time.Date(date.Year(), date.Month(), date.Day()+1, 5, 59, 59, 0, loc)
	default:
		return time.Time{}, time.Time{}
	}
}

// Tentukan shift saat ini
func getCurrentShift(t time.Time) int {
	h, m := t.Hour(), t.Minute()
	isSaturday := t.Weekday() == time.Saturday

	switch {
	case isSaturday && (h < 11 || (h == 11 && m == 0)):
		return 1
	case isSaturday && (h < 16 || (h == 16 && m == 0)):
		return 2
	case isSaturday:
		return 3
	case !isSaturday && (h < 14 || (h == 14 && m == 0)):
		return 1
	case !isSaturday && (h < 22 || (h == 22 && m == 0)):
		return 2
	default:
		return 3
	}
}

// Hitung runtime dari database (full shift)
func getShiftRuntime(start, end time.Time) int64 {
	var countSeconds int64
	config.DB.Model(&models.RetailD5{}).
		Where("start_mesin = ? AND ts >= ? AND ts <= ?", 1, start, end).
		Count(&countSeconds)
	return countSeconds / 60
}

// Hitung durasi aktual shift sampai sekarang
func getActualShiftMinutes(start, end, now time.Time) int64 {
	if now.Before(start) {
		// belum mulai shift
		return 0
	} else if now.After(end) {
		// shift sudah selesai
		return int64(end.Sub(start).Minutes())
	}
	// shift sedang berjalan → dari start sampai sekarang
	return int64(now.Sub(start).Minutes())
}

// API controller
func UptimeStartMesinRealtime(c *gin.Context) {
	dateParam := c.Query("date")
	var date time.Time
	var err error

	if dateParam == "" {
		date = time.Now()
	} else {
		date, err = time.Parse("2006-01-02", dateParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal salah. Gunakan YYYY-MM-DD"})
			return
		}
	}

	now := time.Now()
	var shifts []gin.H

	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(date, i)

		runtimeMinutes := getShiftRuntime(start, end)           // runtime dari DB
		actualMinutes := getActualShiftMinutes(start, end, now) // aktual

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
		"date":          date.Format("2006-01-02"),
		"current_shift": getCurrentShift(now),
		"shifts":        shifts,
	})
}
