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
				SuhuHeating: 1150,
				SuhuHolding: 1160,
				Flowrate:    6000,
			},
			wantDivert:   false,
			wantTriggers: nil,
		},
		{
			name: "Heating Under (< 1110)",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 1080,
				SuhuHolding: 1150,
				Flowrate:    6000,
			},
			wantDivert:   true,
			wantTriggers: []string{"HEATING_UNDER"},
		},
		{
			name: "Heating Over (> 1230)",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 1250,
				SuhuHolding: 1150,
				Flowrate:    6000,
			},
			wantDivert:   true,
			wantTriggers: []string{"HEATING_OVER"},
		},
		{
			name: "Holding Under (< 1110)",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 1150,
				SuhuHolding: 1095,
				Flowrate:    6000,
			},
			wantDivert:   true,
			wantTriggers: []string{"HOLDING_UNDER"},
		},
		{
			name: "Holding Over (> 1230)",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 1150,
				SuhuHolding: 1245,
				Flowrate:    6000,
			},
			wantDivert:   true,
			wantTriggers: []string{"HOLDING_OVER"},
		},
		{
			name: "Flowrate Over (> 7500)",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 1150,
				SuhuHolding: 1150,
				Flowrate:    7600,
			},
			wantDivert:   true,
			wantTriggers: []string{"FLOWRATE_OVER"},
		},
		{
			name: "Multiple Triggers: Heating Under and Flowrate Over",
			sensor: models.SensorPasteurisasi{
				SuhuHeating: 1050,
				SuhuHolding: 1150,
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
