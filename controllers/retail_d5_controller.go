package controllers

import (
    "backend-golang/config"
    "backend-golang/models"
    "github.com/gin-gonic/gin"
    "time"
)

type Shift struct {
    Name  string
    Start time.Time
    End   time.Time
}

func getShiftSchedule(tanggal time.Time) []Shift {
    weekday := tanggal.Weekday()
    if weekday == time.Saturday {
        return []Shift{
            {"Shift 1", time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 6, 0, 0, 0, tanggal.Location()),
             time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 11, 0, 0, 0, tanggal.Location())},
            {"Shift 2", time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 11, 0, 1, 0, tanggal.Location()),
             time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 16, 0, 0, 0, tanggal.Location())},
            {"Shift 3", time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 16, 0, 1, 0, tanggal.Location()),
             time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 21, 0, 0, 0, tanggal.Location())},
        }
    }

    return []Shift{
        {"Shift 1", time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 6, 0, 0, 0, tanggal.Location()),
         time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 14, 0, 0, 0, tanggal.Location())},
        {"Shift 2", time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 14, 0, 1, 0, tanggal.Location()),
         time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 22, 0, 0, 0, tanggal.Location())},
        {"Shift 3", time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 22, 0, 1, 0, tanggal.Location()),
         time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day()+1, 5, 59, 59, 0, tanggal.Location())},
    }
}

func DurasiStartMesinRealtime(c *gin.Context) {
    loc, _ := time.LoadLocation("Asia/Jakarta")
    now := time.Now().In(loc)
    tanggalStr := c.Query("tanggal")

    var tanggal time.Time
    var err error
    if tanggalStr != "" {
        tanggal, err = time.ParseInLocation("2006-01-02", tanggalStr, loc)
        if err != nil {
            c.JSON(400, gin.H{"error": "Format tanggal tidak valid"})
            return
        }
    } else {
        tanggal = now
    }

    shifts := getShiftSchedule(tanggal)
    hasil := map[string]interface{}{}

    for _, shift := range shifts {
        var count int64
        config.DB.Model(&models.RetailD5{}).
            Where("ts BETWEEN ? AND ?", shift.Start, shift.End).
            Where("start_mesin = ?", 1).
            Count(&count)

        var menitBerjalan int
        if now.After(shift.Start) && now.Before(shift.End) {
            menitBerjalan = int(now.Sub(shift.Start).Minutes())
        } else {
            menitBerjalan = int(shift.End.Sub(shift.Start).Minutes())
        }

        isSaturday := tanggal.Weekday() == time.Saturday
        pembagi := menitBerjalan
        if now.After(shift.End) {
            pembagi = 300
            if !isSaturday {
                pembagi = 420
            }
        }

        key := "shift" + shift.Name[len(shift.Name)-1:]
        hasil[key] = gin.H{
            "menit_shift": menitBerjalan,
            "detik":       count,
            "hasil":       float64(count) / 60 / float64(pembagi),
        }
    }

    c.JSON(200, gin.H{"result": hasil})
}