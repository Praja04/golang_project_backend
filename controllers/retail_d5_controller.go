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
	Runtime   int64     `json:"runtime_total_seconds"`
}

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

func DurasiStartMesinRealtime(c *gin.Context) {
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

	var shifts []ShiftInfo
	for i := 1; i <= 3; i++ {
		start, end := getShiftRange(date, i)
		var count int64
		config.DB.Model(&models.RetailD5{}).
			Where("start_mesin = ? AND ts >= ? AND ts <= ?", 1, start, end).
			Count(&count)

		shifts = append(shifts, ShiftInfo{
			Shift:     i,
			StartTime: start,
			EndTime:   end,
			Runtime:   count,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":          date.Format("2006-01-02"),
		"current_shift": getCurrentShift(time.Now()),
		"shifts":        shifts,
	})
}