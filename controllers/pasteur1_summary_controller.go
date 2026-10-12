package controllers

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"backend-golang/config"
	"backend-golang/models"

	"github.com/gin-gonic/gin"
)

// ─── Shift Definitions ────────────────────────────────────────────────────────
//
// Senin – Jumat:
//   Shift 1: 06:00 – 14:00 (8 jam)
//   Shift 2: 14:00 – 22:00 (8 jam)
//   Shift 3: 22:00 – 06:00 (8 jam, carry-over ke hari berikutnya)
//
// Sabtu & Minggu:
//   Shift 1: 06:00 – 11:00 (5 jam)
//   Shift 2: 14:00 – 22:00 (8 jam) — gap 11:00–14:00 tidak ada produksi
//   Shift 3: 22:00 – 06:00 (8 jam, carry-over)
// ──────────────────────────────────────────────────────────────────────────────

func isWeekend(weekday time.Weekday) bool {
	return weekday == time.Saturday || weekday == time.Sunday
}

// getShiftNumber mengembalikan nomor shift (1/2/3) dari sebuah timestamp.
// Mengembalikan 0 jika di luar jam produksi (gap Sabtu/Minggu 11:00–14:00).
// Semua pengecekan dilakukan dalam zona WIB.
func getShiftNumber(t time.Time) int {
	wib := t.In(jakartaLoc)
	minuteOfDay := wib.Hour()*60 + wib.Minute()
	weekend := isWeekend(wib.Weekday())

	// Shift 1 Sabtu/Minggu: 06:00–11:00 (360–660 menit)
	if weekend {
		if minuteOfDay >= 360 && minuteOfDay < 660 {
			return 1
		}
	} else {
		// Shift 1 Senin–Jumat: 06:00–14:00 (360–840 menit)
		if minuteOfDay >= 360 && minuteOfDay < 840 {
			return 1
		}
	}

	// Shift 2 semua hari: 14:00–22:00 (840–1320 menit)
	if minuteOfDay >= 840 && minuteOfDay < 1320 {
		return 2
	}

	// Shift 3 semua hari: 22:00–06:00 (>= 1320 atau < 360)
	if minuteOfDay >= 1320 || minuteOfDay < 360 {
		return 3
	}

	return 0 // gap 11:00–14:00 pada Sabtu/Minggu
}

// getShiftLabel mengembalikan label shift yang sesuai konteks hari
func getShiftLabel(shiftNum int, weekday time.Weekday) string {
	switch shiftNum {
	case 1:
		if isWeekend(weekday) {
			return "Shift 1 (06:00 - 11:00)"
		}
		return "Shift 1 (06:00 - 14:00)"
	case 2:
		return "Shift 2 (14:00 - 22:00)"
	case 3:
		return "Shift 3 (22:00 - 06:00)"
	}
	return "Unknown"
}

// getShiftDurationSeconds mengembalikan durasi standar shift dalam detik
func getShiftDurationSeconds(shiftNum int, weekday time.Weekday) int64 {
	if shiftNum == 1 && isWeekend(weekday) {
		return 5 * 3600 // 5 jam untuk Sabtu & Minggu Shift 1
	}
	return 8 * 3600 // 8 jam standar
}

// ─── Response Structures ──────────────────────────────────────────────────────

type SumStatFloat struct {
	Avg float64 `json:"avg"`
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type SumStatFloatWithPct struct {
	Avg       float64 `json:"avg"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	PctNormal float64 `json:"pct_normal"` // % data dalam rentang normal
}

type SumDivertReport struct {
	TotalKejadian        int64          `json:"total_kejadian"`
	TotalDurasiDetik     int64          `json:"total_durasi_detik"`
	TotalDurasiMenit     string         `json:"total_durasi_menit"`
	PctDivertDariOperasi float64        `json:"pct_divert_dari_operasi"`
	BreakdownPemicu      map[string]int `json:"breakdown_pemicu"`
}

type SumAvailability struct {
	PctNormal float64 `json:"pct_normal"`
	PctDivert float64 `json:"pct_divert"`
}

type ShiftSummaryRow struct {
	Tanggal            string `json:"tanggal"`
	HariLabel          string `json:"hari"`
	Shift              int    `json:"shift"`
	ShiftLabel         string `json:"shift_label"`
	HasData            bool   `json:"has_data"`
	TotalDataPoint     int64  `json:"total_data_point"`
	DurasiOperasiDetik int64  `json:"durasi_operasi_detik"`

	SuhuHeating    SumStatFloatWithPct `json:"suhu_heating"`
	SuhuHolding    SumStatFloatWithPct `json:"suhu_holding"`
	Flowrate       SumStatFloatWithPct `json:"flowrate"`
	SuhuPreheating SumStatFloat        `json:"suhu_preheating"`
	SuhuPrecooling SumStatFloat        `json:"suhu_precooling"`
	SuhuCooling    SumStatFloat        `json:"suhu_cooling"`
	LevelBT2       SumStatFloat        `json:"level_bt2"`
	LevelVD        SumStatFloat        `json:"level_vd"`

	Divert       SumDivertReport `json:"divert"`
	Availability SumAvailability `json:"availability"`
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func roundF(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func calcStat(values []float64) (avg, minV, maxV float64) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	minV, maxV = values[0], values[0]
	sum := 0.0
	for _, v := range values {
		sum += v
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	return roundF(sum/float64(len(values)), 2), roundF(minV, 2), roundF(maxV, 2)
}

func calcPctInRange(values []float64, lo, hi float64) float64 {
	if len(values) == 0 {
		return 0
	}
	n := 0
	for _, v := range values {
		if v >= lo && v <= hi {
			n++
		}
	}
	return roundF(float64(n)/float64(len(values))*100, 2)
}

func calcPctBelowMax(values []float64, maxV float64) float64 {
	if len(values) == 0 {
		return 0
	}
	n := 0
	for _, v := range values {
		if v <= maxV {
			n++
		}
	}
	return roundF(float64(n)/float64(len(values))*100, 2)
}

func hariLabel(weekday time.Weekday) string {
	days := map[time.Weekday]string{
		time.Monday:    "Senin",
		time.Tuesday:   "Selasa",
		time.Wednesday: "Rabu",
		time.Thursday:  "Kamis",
		time.Friday:    "Jumat",
		time.Saturday:  "Sabtu",
		time.Sunday:    "Minggu",
	}
	if label, ok := days[weekday]; ok {
		return label
	}
	return weekday.String()
}

// ─── Main Handler ─────────────────────────────────────────────────────────────

// GetSummaryPasteur1 — Summary Report Pasteur 1 per shift dengan filter date range.
//
// Query Parameters:
//   start_date  : YYYY-MM-DD  (default: hari ini)
//   end_date    : YYYY-MM-DD  (default: hari ini)
//   shift       : 1 | 2 | 3   (default: semua shift)
//
// Contoh:
//   GET /api/pasteur/summary
//   GET /api/pasteur/summary?start_date=2026-10-01&end_date=2026-10-12
//   GET /api/pasteur/summary?start_date=2026-10-12&end_date=2026-10-12&shift=2
func GetSummaryPasteur1(c *gin.Context) {
	// ── 1. Parse parameter ────────────────────────────────────────────────────
	today := time.Now().In(jakartaLoc).Format("2006-01-02")
	startDateStr := c.DefaultQuery("start_date", today)
	endDateStr := c.DefaultQuery("end_date", today)
	shiftFilter := c.Query("shift") // kosong = semua

	startDate, err := time.ParseInLocation("2006-01-02", startDateStr, jakartaLoc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Format start_date tidak valid (YYYY-MM-DD)"})
		return
	}
	endDate, err := time.ParseInLocation("2006-01-02", endDateStr, jakartaLoc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Format end_date tidak valid (YYYY-MM-DD)"})
		return
	}
	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "end_date tidak boleh sebelum start_date"})
		return
	}
	if endDate.Sub(startDate).Hours() > 31*24 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Rentang maksimal 31 hari"})
		return
	}

	// ── 2. Ambil data sensor dari database ────────────────────────────────────
	// startDate sudah dalam WIB, tambahkan 1 hari ekstra untuk Shift 3 carry-over
	queryStart := startDate
	queryEnd := endDate.Add(24*time.Hour + 6*time.Hour) // +30 jam untuk cover Shift 3

	var sensors []models.SensorPasteurisasi
	if err := config.DB.
		Where("Waktu >= ? AND Waktu < ?", queryStart, queryEnd).
		Order("Waktu ASC").
		Find(&sensors).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil data sensor",
			"error":   err.Error(),
		})
		return
	}

	// ── 3. Ambil data divert dari database ────────────────────────────────────
	var divertLogs []models.DivertLogPasteur1
	config.DB.
		Where("waktu_mulai >= ? AND waktu_mulai < ?", queryStart, queryEnd).
		Order("waktu_mulai ASC").
		Find(&divertLogs)

	// ── 4. Definisi tipe bucket ───────────────────────────────────────────────
	type shiftKey struct {
		Tanggal string
		Shift   int
	}
	type sensorBucket struct {
		weekday    time.Weekday
		heating    []float64
		holding    []float64
		flowrate   []float64
		preheating []float64
		precooling []float64
		cooling    []float64
		levelBT2   []float64
		levelVD    []float64
	}
	type divertAgg struct {
		kejadian         int64
		totalDurasiDetik int64
		pemicuCount      map[string]int
	}

	sensorBuckets := map[shiftKey]*sensorBucket{}
	divertByKey := map[shiftKey]*divertAgg{}

	// ── 5. Kelompokkan data sensor ke bucket tanggal-shift ────────────────────
	for _, s := range sensors {
		wib := s.Waktu.In(jakartaLoc)
		shiftNum := getShiftNumber(s.Waktu)
		if shiftNum == 0 {
			continue // di luar jam produksi (gap Sabtu/Minggu 11:00–14:00)
		}

		// Shift 3 yang melewati tengah malam: atribusikan ke hari shift dimulai
		tanggalWIB := wib.Format("2006-01-02")
		weekdayForKey := wib.Weekday()
		if shiftNum == 3 && wib.Hour() < 6 {
			prev := wib.AddDate(0, 0, -1)
			tanggalWIB = prev.Format("2006-01-02")
			weekdayForKey = prev.Weekday()
		}

		key := shiftKey{Tanggal: tanggalWIB, Shift: shiftNum}
		if _, ok := sensorBuckets[key]; !ok {
			sensorBuckets[key] = &sensorBucket{weekday: weekdayForKey}
		}
		b := sensorBuckets[key]
		b.heating = append(b.heating, s.SuhuHeating)
		b.holding = append(b.holding, s.SuhuHolding)
		b.flowrate = append(b.flowrate, s.Flowrate)
		b.preheating = append(b.preheating, s.SuhuPreheating)
		b.precooling = append(b.precooling, s.SuhuPrecooling)
		b.cooling = append(b.cooling, s.SuhuCooling)
		b.levelBT2 = append(b.levelBT2, s.LevelBT2)
		b.levelVD = append(b.levelVD, s.LevelVD)
	}

	// ── 6. Kelompokkan data divert ke bucket tanggal-shift ───────────────────
	for _, d := range divertLogs {
		wib := d.WaktuMulai.In(jakartaLoc)
		shiftNum := getShiftNumber(d.WaktuMulai)
		if shiftNum == 0 {
			continue
		}
		tanggalWIB := wib.Format("2006-01-02")
		if shiftNum == 3 && wib.Hour() < 6 {
			tanggalWIB = wib.AddDate(0, 0, -1).Format("2006-01-02")
		}
		key := shiftKey{Tanggal: tanggalWIB, Shift: shiftNum}
		if _, ok := divertByKey[key]; !ok {
			divertByKey[key] = &divertAgg{
				pemicuCount: map[string]int{
					"heating_under": 0, "heating_over": 0,
					"holding_under": 0, "holding_over": 0,
					"flowrate_over": 0,
				},
			}
		}
		agg := divertByKey[key]
		agg.kejadian++
		agg.totalDurasiDetik += d.DurasiDetik
		for _, p := range strings.Split(d.Pemicu, ",") {
			switch strings.TrimSpace(strings.ToUpper(p)) {
			case "HEATING_UNDER":
				agg.pemicuCount["heating_under"]++
			case "HEATING_OVER":
				agg.pemicuCount["heating_over"]++
			case "HOLDING_UNDER":
				agg.pemicuCount["holding_under"]++
			case "HOLDING_OVER":
				agg.pemicuCount["holding_over"]++
			case "FLOWRATE_OVER":
				agg.pemicuCount["flowrate_over"]++
			}
		}
	}

	// ── 7. Susun hasil per hari & per shift ───────────────────────────────────
	var results []ShiftSummaryRow

	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		tanggalStr := d.Format("2006-01-02")
		weekday := d.Weekday()
		hari := hariLabel(weekday)

		for _, shiftNum := range []int{1, 2, 3} {
			// Filter shift jika diminta
			if shiftFilter != "" && fmt.Sprintf("%d", shiftNum) != shiftFilter {
				continue
			}

			key := shiftKey{Tanggal: tanggalStr, Shift: shiftNum}
			shiftDurasi := getShiftDurationSeconds(shiftNum, weekday)
			shiftLabelStr := getShiftLabel(shiftNum, weekday)
			bucket, hasData := sensorBuckets[key]

			// Divert aggregation
			dvReport := SumDivertReport{
				TotalDurasiMenit: "0.0",
				BreakdownPemicu: map[string]int{
					"heating_under": 0, "heating_over": 0,
					"holding_under": 0, "holding_over": 0,
					"flowrate_over": 0,
				},
			}
			if agg, ok := divertByKey[key]; ok {
				dvReport.TotalKejadian = agg.kejadian
				dvReport.TotalDurasiDetik = agg.totalDurasiDetik
				dvReport.TotalDurasiMenit = fmt.Sprintf("%.1f", float64(agg.totalDurasiDetik)/60.0)
				if shiftDurasi > 0 {
					dvReport.PctDivertDariOperasi = roundF(float64(agg.totalDurasiDetik)/float64(shiftDurasi)*100, 2)
					if dvReport.PctDivertDariOperasi > 100 {
						dvReport.PctDivertDariOperasi = 100
					}
				}
				dvReport.BreakdownPemicu = agg.pemicuCount
			}

			pctDivert := dvReport.PctDivertDariOperasi
			row := ShiftSummaryRow{
				Tanggal:            tanggalStr,
				HariLabel:          hari,
				Shift:              shiftNum,
				ShiftLabel:         shiftLabelStr,
				HasData:            hasData,
				DurasiOperasiDetik: shiftDurasi,
				Divert:             dvReport,
				Availability: SumAvailability{
					PctNormal: roundF(100-pctDivert, 2),
					PctDivert: pctDivert,
				},
			}

			if hasData {
				hAvg, hMin, hMax := calcStat(bucket.heating)
				hoAvg, hoMin, hoMax := calcStat(bucket.holding)
				frAvg, frMin, frMax := calcStat(bucket.flowrate)
				preAvg, preMin, preMax := calcStat(bucket.preheating)
				pcAvg, pcMin, pcMax := calcStat(bucket.precooling)
				cAvg, cMin, cMax := calcStat(bucket.cooling)
				bt2Avg, bt2Min, bt2Max := calcStat(bucket.levelBT2)
				vdAvg, vdMin, vdMax := calcStat(bucket.levelVD)

				row.TotalDataPoint = int64(len(bucket.heating))
				row.SuhuHeating = SumStatFloatWithPct{
					Avg: hAvg, Min: hMin, Max: hMax,
					PctNormal: calcPctInRange(bucket.heating, 1110, 1230),
				}
				row.SuhuHolding = SumStatFloatWithPct{
					Avg: hoAvg, Min: hoMin, Max: hoMax,
					PctNormal: calcPctInRange(bucket.holding, 1110, 1230),
				}
				row.Flowrate = SumStatFloatWithPct{
					Avg: frAvg, Min: frMin, Max: frMax,
					PctNormal: calcPctBelowMax(bucket.flowrate, 7500),
				}
				row.SuhuPreheating = SumStatFloat{Avg: preAvg, Min: preMin, Max: preMax}
				row.SuhuPrecooling = SumStatFloat{Avg: pcAvg, Min: pcMin, Max: pcMax}
				row.SuhuCooling = SumStatFloat{Avg: cAvg, Min: cMin, Max: cMax}
				row.LevelBT2 = SumStatFloat{Avg: bt2Avg, Min: bt2Min, Max: bt2Max}
				row.LevelVD = SumStatFloat{Avg: vdAvg, Min: vdMin, Max: vdMax}
			}

			results = append(results, row)
		}
	}

	// ── 8. Response ───────────────────────────────────────────────────────────
	filterShiftLabel := "all"
	if shiftFilter != "" {
		filterShiftLabel = "Shift " + shiftFilter
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"filter": gin.H{
			"start_date": startDateStr,
			"end_date":   endDateStr,
			"shift":      filterShiftLabel,
		},
		"total_records": len(results),
		"summary":       results,
	})
}
