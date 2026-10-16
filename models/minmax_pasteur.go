package models

import (
	"time"
)

// MasterDataMinMaxPasteur1 memetakan tabel master_data_minmaxpasteur1
type MasterDataMinMaxPasteur1 struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ParameterName string    `json:"parameter_name" gorm:"column:parameter_name"`
	Value         float64   `json:"value" gorm:"column:value"`
	CreatedAt     time.Time `json:"created_at" gorm:"column:created_at"`
}

// TableName menentukan nama tabel di database MySQL
func (MasterDataMinMaxPasteur1) TableName() string {
	return "master_data_minmaxpasteur1"
}
