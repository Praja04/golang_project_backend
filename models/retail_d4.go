package models

import "time"

type RetailD4 struct {
    ID         uint      `gorm:"primaryKey"`
    Ts         time.Time `gorm:"column:ts"`
    StartMesin int       `gorm:"column:start_mesin"`
    TotalCounter int       `gorm:"column:total_counter"`
    MainSpeed int       `gorm:"column:main_speed"`
}

func (RetailD4) TableName() string  { return "retail_d4" }