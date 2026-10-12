package models

import (
	"time"
)

// DivertLogPasteur1 memetakan tabel divert_logs_pasteur1
type DivertLogPasteur1 struct {
	ID           uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	WaktuMulai   time.Time  `json:"waktu_mulai" gorm:"column:waktu_mulai;index"`
	WaktuSelesai *time.Time `json:"waktu_selesai" gorm:"column:waktu_selesai"`
	DurasiDetik  int64      `json:"durasi_detik" gorm:"column:durasi_detik;default:0"`
	SuhuHeating  float64    `json:"suhu_heating" gorm:"column:suhu_heating"`
	SuhuHolding  float64    `json:"suhu_holding" gorm:"column:suhu_holding"`
	Flowrate     float64    `json:"flowrate" gorm:"column:flowrate"`
	Pemicu       string     `json:"pemicu" gorm:"column:pemicu;type:varchar(100);index"`
	Keterangan   string     `json:"keterangan" gorm:"column:keterangan;type:varchar(255)"`
	Status       string     `json:"status" gorm:"column:status;type:varchar(20);default:'CLOSED'"` // 'ACTIVE' atau 'CLOSED'
	Count        int        `json:"count" gorm:"column:count;default:1"`
	CreatedAt    time.Time  `json:"created_at" gorm:"column:created_at"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"column:updated_at"`
}

// Custom table name
func (DivertLogPasteur1) TableName() string {
	return "divert_logs_pasteur1"
}
