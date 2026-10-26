package services

import (
	"backend-golang/models"
	"strings"
	"testing"
)

func TestCheckDivertTriggers(t *testing.T) {
	tests := []struct {
		name         string
		sensor       models.SensorPasteurisasi
		wantDivert   bool
		wantTriggers []string
	}{
		{
			name: "Normal Condition",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 115.0,
				SuhuHolding: 116.0,
				Flowrate:    6000,
			},
			wantDivert:   false,
			wantTriggers: nil,
		},
		{
			name: "Heating Under (< 110.0)",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 108.0,
				SuhuHolding: 115.0,
				Flowrate:    6000,
			},
			wantDivert:   true,
			wantTriggers: []string{"HEATING_UNDER"},
		},
		{
			name: "Heating Over (> 123.0)",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 125.0,
				SuhuHolding: 115.0,
				Flowrate:    6000,
			},
			wantDivert:   true,
			wantTriggers: []string{"HEATING_OVER"},
		},
		{
			name: "Holding Under (< 110.0)",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 115.0,
				SuhuHolding: 109.5,
				Flowrate:    6000,
			},
			wantDivert:   true,
			wantTriggers: []string{"HOLDING_UNDER"},
		},
		{
			name: "Holding Over (> 123.0)",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 115.0,
				SuhuHolding: 124.5,
				Flowrate:    6000,
			},
			wantDivert:   true,
			wantTriggers: []string{"HOLDING_OVER"},
		},
		{
			name: "Flowrate Over (> 7700)",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 115.0,
				SuhuHolding: 115.0,
				Flowrate:    7800,
			},
			wantDivert:   true,
			wantTriggers: []string{"FLOWRATE_OVER"},
		},
		{
			name: "Multiple Triggers: Heating Under and Flowrate Over",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 105.0,
				SuhuHolding: 115.0,
				Flowrate:    7800,
			},
			wantDivert:   true,
			wantTriggers: []string{"HEATING_UNDER", "FLOWRATE_OVER"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDivert, gotTriggers, keterangan := CheckDivertTriggers(&tt.sensor)
			if gotDivert != tt.wantDivert {
				t.Errorf("CheckDivertTriggers() gotDivert = %v, want %v", gotDivert, tt.wantDivert)
			}
			if len(gotTriggers) != len(tt.wantTriggers) {
				t.Errorf("CheckDivertTriggers() triggers count = %d, want %d (triggers: %v, ket: %s)",
					len(gotTriggers), len(tt.wantTriggers), gotTriggers, keterangan)
			}
			for _, expectedTrigger := range tt.wantTriggers {
				found := false
				for _, actual := range gotTriggers {
					if actual == expectedTrigger {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("CheckDivertTriggers() expected trigger %s not found in %v", expectedTrigger, gotTriggers)
				}
			}

			if tt.wantDivert && (keterangan == "" || strings.EqualFold(keterangan, "Normal")) {
				t.Errorf("CheckDivertTriggers() expected detailed keterangan for divert, got '%s'", keterangan)
			}
		})
	}
}

func TestCheckDivertTriggersWithDynamicLimits(t *testing.T) {
	// Simulasi data dari tabel master_data_minmaxpasteur1:
	// MaxSuhuPasteur = 1230 -> 123.0
	// MinSuhuPasteur = 1100 -> 110.0
	maxValDB := 1230.0
	minValDB := 1100.0

	minLimit := minValDB / 10.0 // 110.0
	maxLimit := maxValDB / 10.0 // 123.0
	maxFlow := 7700.0

	sensorNormal := models.SensorPasteurisasi{
		SuhuHeating: 115.0,
		SuhuHolding: 115.0,
		Flowrate:    5000,
	}
	isDiv, _, _ := CheckDivertTriggersWithLimits(&sensorNormal, minLimit, maxLimit, maxFlow)
	if isDiv {
		t.Errorf("Expected normal, got divert")
	}

	sensorUnder := models.SensorPasteurisasi{
		SuhuHeating: 109.9,
		SuhuHolding: 115.0,
		Flowrate:    5000,
	}
	isDiv, trigs, _ := CheckDivertTriggersWithLimits(&sensorUnder, minLimit, maxLimit, maxFlow)
	if !isDiv || len(trigs) == 0 || trigs[0] != "HEATING_UNDER" {
		t.Errorf("Expected HEATING_UNDER divert, got isDiv=%v, trigs=%v", isDiv, trigs)
	}

	sensorOver := models.SensorPasteurisasi{
		SuhuHeating: 115.0,
		SuhuHolding: 123.1,
		Flowrate:    5000,
	}
	isDiv, trigs, _ = CheckDivertTriggersWithLimits(&sensorOver, minLimit, maxLimit, maxFlow)
	if !isDiv || len(trigs) == 0 || trigs[0] != "HOLDING_OVER" {
		t.Errorf("Expected HOLDING_OVER divert, got isDiv=%v, trigs=%v", isDiv, trigs)
	}
}

