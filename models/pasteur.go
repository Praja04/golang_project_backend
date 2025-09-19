package models

import "time"

// Struct sesuai field di tabel readsensors_pasteurisasi1
type SensorPasteurisasi struct {
	Waktu             time.Time `json:"waktu" gorm:"column:Waktu;primaryKey"`
	SpeedPompaMixing  float64   `json:"speed_pompa_mixing" gorm:"column:Speed_Pompa_Mixing"`
	PressureMixing    float64   `json:"pressure_mixing" gorm:"column:Pressure_Mixing"`
	SuhuPreheating    float64   `json:"suhu_preheating" gorm:"column:Suhu_P reheating"`
	LevelBT1          float64   `json:"level_bt1" gorm:"column:Level_BT1"`
	SpeedPumpBT1      float64   `json:"speed_pump_bt1" gorm:"column:Speed_Pump_BT1"`
	LevelVD           float64   `json:"level_vd" gorm:"column:Level_VD"`
	SpeedPumpVD       float64   `json:"speed_pump_vd" gorm:"column:Speed_Pump_VD"`
	Flowrate          float64   `json:"flowrate" gorm:"column:Flowrate"`
	SuhuHeating       float64   `json:"suhu_heating" gorm:"column:SuhuHeating"`
	SuhuHolding       float64   `json:"suhu_holding" gorm:"column:SuhuHolding"`
	SuhuPrecooling    float64   `json:"suhu_precooling" gorm:"column:SuhuPrecooling"`
	LevelBT2          float64   `json:"level_bt2" gorm:"column:Level_BT2"`
	SpeedPumpBT2      float64   `json:"speed_pump_bt2" gorm:"column:Speed_Pump_BT2"`
	PressureBT2       float64   `json:"pressure_bt2" gorm:"column:Pressure_BT2"`
	SuhuCooling       float64   `json:"suhu_cooling" gorm:"column:SuhuCooling"`
	PressToPasteur    float64   `json:"press_to_pasteur" gorm:"column:Press_To_Pasteur"`
	VDHH              float64   `json:"vdhh" gorm:"column:VDHH"`
	VDLL              float64   `json:"vdll" gorm:"column:VDLL"`
	MixingAM          float64   `json:"mixing_am" gorm:"column:MixingAM"`
	BT1AM             float64   `json:"bt1_am" gorm:"column:BT1AM"`
	VDAM              float64   `json:"vd_am" gorm:"column:VDAM"`
	PCV1              float64   `json:"pcv1" gorm:"column:PCV1"`
	TimeDivert        float64   `json:"time_divert" gorm:"column:Time_Divert"`
}

// Custom table name
func (SensorPasteurisasi) TableName() string {
	return "readsensors_pasteurisasi1"
}
