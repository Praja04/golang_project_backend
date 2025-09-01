package models

import "time"

type RetailD5 struct {
    ID         uint      `gorm:"primaryKey"`
    Ts         time.Time `gorm:"column:ts"`
    StartMesin int       `gorm:"column:start_mesin"`
    TotalCounter int       `gorm:"column:total_counter"`
}