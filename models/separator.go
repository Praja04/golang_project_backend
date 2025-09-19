package models

import (
	"time"
)

type SeparatorSensor struct {
	Waktu      time.Time ` gorm:"column:waktu;primaryKey"`
	Separator1 int       ` gorm:"column:separator1"`
	Separator2 int       ` gorm:"column:separator2"`
	Separator3 int       ` gorm:"column:separator3"`
	Separator4 int       ` gorm:"column:separator4"`
}

func (SeparatorSensor) TableName() string {
	return "readsensors_separator"
}