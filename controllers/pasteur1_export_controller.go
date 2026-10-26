package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"backend-golang/config"
	"backend-golang/models"
	"backend-golang/services"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// ExportSensorPasteur1ToExcel — Export data sensor Pasteur 1 ke Excel
//
// Query Parameters:
//   start_date  : YYYY-MM-DD  (default: hari ini)
//   end_date    : YYYY-MM-DD  (default: hari ini)
//   shift       : 1 | 2 | 3   (default: semua shift)
//
// Response: file .xlsx sebagai attachment download
func ExportSensorPasteur1ToExcel(c *gin.Context) {
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

	// ── 2. Query data sensor ──────────────────────────────────────────────────
	queryStart := startDateStr + " 00:00:00"
	nextDay := endDate.AddDate(0, 0, 1).Format("2006-01-02")
	queryEnd := nextDay + " 06:00:00" // cover Shift 3

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

	// ── 3. Filter shift jika diminta ──────────────────────────────────────────
	var filtered []models.SensorPasteurisasi
	for _, s := range sensors {
		wib := toWIBTime(s.Waktu)
		shiftNum := getShiftNumber(wib)
		if shiftNum == 0 {
			continue // di luar jam produksi
		}
		if shiftFilter != "" && fmt.Sprintf("%d", shiftNum) != shiftFilter {
			continue
		}
		filtered = append(filtered, s)
	}

	// ── 4. Buat file Excel ────────────────────────────────────────────────────
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Data Sensor Pasteur 1"
	f.NewSheet(sheet)
	f.DeleteSheet("Sheet1") // hapus sheet default

	// ── 4a. Style header ──────────────────────────────────────────────────────
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Size:  11,
			Color: "FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"1F4E79"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "FFFFFF", Style: 1},
			{Type: "right", Color: "FFFFFF", Style: 1},
			{Type: "top", Color: "FFFFFF", Style: 1},
			{Type: "bottom", Color: "FFFFFF", Style: 1},
		},
	})

	dataStyleEven, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"EBF3FB"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "D0D0D0", Style: 1},
			{Type: "right", Color: "D0D0D0", Style: 1},
			{Type: "bottom", Color: "D0D0D0", Style: 1},
		},
	})

	dataStyleOdd, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"FFFFFF"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "D0D0D0", Style: 1},
			{Type: "right", Color: "D0D0D0", Style: 1},
			{Type: "bottom", Color: "D0D0D0", Style: 1},
		},
	})

	divertStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "C00000"},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"FFE0E0"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "D0D0D0", Style: 1},
			{Type: "right", Color: "D0D0D0", Style: 1},
			{Type: "bottom", Color: "D0D0D0", Style: 1},
		},
	})

	// ── 4b. Baris info laporan ────────────────────────────────────────────────
	infoStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: "1F4E79"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	f.MergeCell(sheet, "A1", "Z1")
	f.SetCellValue(sheet, "A1", "LAPORAN DATA SENSOR PASTEUR 1")
	f.SetCellStyle(sheet, "A1", "Z1", infoStyle)

	shiftLabel := "Semua Shift"
	if shiftFilter != "" {
		shiftLabel = "Shift " + shiftFilter
	}
	f.MergeCell(sheet, "A2", "Z2")
	f.SetCellValue(sheet, "A2", fmt.Sprintf("Periode: %s s/d %s   |   Filter Shift: %s   |   Total Data: %d baris",
		startDateStr, endDateStr, shiftLabel, len(filtered)))
	f.SetCellStyle(sheet, "A2", "Z2", infoStyle)

	f.SetRowHeight(sheet, 1, 22)
	f.SetRowHeight(sheet, 2, 18)
	f.SetRowHeight(sheet, 3, 14) // baris kosong pemisah

	// ── 4c. Header kolom (row 4) ──────────────────────────────────────────────
	headers := []struct {
		Col   string
		Label string
		Width float64
	}{
		{"A", "No", 6},
		{"B", "Tanggal", 12},
		{"C", "Waktu (WIB)", 14},
		{"D", "Shift", 8},
		{"E", "Status Divert", 14},
		{"F", "Speed Pompa Mixing", 18},
		{"G", "Pressure Mixing", 16},
		{"H", "Suhu Preheating (°C)", 20},
		{"I", "Level BT1 (%)", 14},
		{"J", "Speed Pump BT1", 14},
		{"K", "Level VD", 12},
		{"L", "Speed Pump VD", 14},
		{"M", "Flowrate (L/H)", 14},
		{"N", "Suhu Heating", 14},
		{"O", "Suhu Holding", 14},
		{"P", "Suhu Precooling (°C)", 20},
		{"Q", "Level BT2 (%)", 14},
		{"R", "Speed Pump BT2", 14},
		{"S", "Pressure BT2", 13},
		{"T", "Suhu Cooling (°C)", 18},
		{"U", "Press To Pasteur", 16},
		{"V", "VDHH", 10},
		{"W", "VDLL", 10},
		{"X", "MixingAM", 11},
		{"Y", "BT1AM", 10},
		{"Z", "VDAM", 10},
		{"AA", "PCV1", 10},
		{"AB", "Time Divert (s)", 15},
	}

	for _, h := range headers {
		cell := h.Col + "4"
		f.SetCellValue(sheet, cell, h.Label)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
		f.SetColWidth(sheet, h.Col, h.Col, h.Width)
	}
	f.SetRowHeight(sheet, 4, 36)

	// ── 4d. Isi data ──────────────────────────────────────────────────────────
	minTemp, maxTemp := services.GetLatestTempThresholdsPasteur1()
	for i, s := range filtered {
		row := i + 5 // mulai dari baris 5
		wib := toWIBTime(s.Waktu)
		shiftNum := getShiftNumber(wib)
		weekday := wib.Weekday()

		// Tentukan apakah sedang divert
		isDivert, _, _ := services.CheckDivertTriggersWithLimits(&s, minTemp, maxTemp, services.LimitFlowrateMax)

		statusDivert := "Normal"
		if isDivert {
			statusDivert = "DIVERT"
		}

		rowData := []interface{}{
			i + 1,
			wib.Format("2006-01-02"),
			wib.Format("15:04:05"),
			fmt.Sprintf("Shift %d", shiftNum),
			statusDivert,
			s.SpeedPompaMixing,
			s.PressureMixing,
			s.SuhuPreheating,
			s.LevelBT1,
			s.SpeedPumpBT1,
			s.LevelVD,
			s.SpeedPumpVD,
			s.Flowrate,
			s.SuhuHeating,
			s.SuhuHolding,
			s.SuhuPrecooling,
			s.LevelBT2,
			s.SpeedPumpBT2,
			s.PressureBT2,
			s.SuhuCooling,
			s.PressToPasteur,
			s.VDHH,
			s.VDLL,
			s.MixingAM,
			s.BT1AM,
			s.VDAM,
			s.PCV1,
			s.TimeDivert,
		}

		cols := []string{
			"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L",
			"M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X",
			"Y", "Z", "AA", "AB",
		}

		for j, col := range cols {
			cell := fmt.Sprintf("%s%d", col, row)
			f.SetCellValue(sheet, cell, rowData[j])
		}

		// Optimasi performa: terapkan style 1 baris sekaligus (A-AB) bukan per sel
		startCell := fmt.Sprintf("A%d", row)
		endCell := fmt.Sprintf("AB%d", row)
		if isDivert {
			f.SetCellStyle(sheet, startCell, endCell, divertStyle)
		} else if i%2 == 0 {
			f.SetCellStyle(sheet, startCell, endCell, dataStyleEven)
		} else {
			f.SetCellStyle(sheet, startCell, endCell, dataStyleOdd)
		}

		f.SetRowHeight(sheet, row, 18)
		_ = weekday // digunakan lewat getShiftLabel bila perlu
	}

	// ── 4e. Freeze baris header ───────────────────────────────────────────────
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      4,
		TopLeftCell: "A5",
		ActivePane:  "bottomLeft",
	})

	// ── 4f. Tambahkan sheet 'Proses masak Pasteur' ──
	appendSheetProsesMasakPasteur(f, sensors, startDateStr, endDateStr, shiftFilter)

	// ── 5. Kirim file sebagai response download ───────────────────────────────
	filename := fmt.Sprintf("sensor_pasteur1_%s_sd_%s.xlsx", startDateStr, endDateStr)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Cache-Control", "no-cache")

	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal menghasilkan file Excel",
			"error":   err.Error(),
		})
	}
}

// -----------------------------------------------------------------------------
// Sheet 2: Proses Masak Pasteur
// -----------------------------------------------------------------------------

type FinishedBatchItem struct {
	ID                 uint    `json:"id"`
	ProductionBatchID  uint    `json:"production_batch_id"`
	PONumber           string  `json:"po_number"`
	BatchRange         string  `json:"batch_range"`
	Variant            string  `json:"variant"`
	Storage            string  `json:"storage"`
	Tanggal            string  `json:"tanggal"`
	Shift              int     `json:"shift"`
	Group              string  `json:"group"`
	LinePasteurisasi   string  `json:"line_pasteurisasi"`
	JamMulaiPasteur    string  `json:"jam_mulai_pasteur"`
	JamSelesaiPasteur  string  `json:"jam_selesai_pasteur"`
	WaktuMulaiFull     string  `json:"waktu_mulai_full"`
	WaktuSelesaiFull   string  `json:"waktu_selesai_full"`
	StatusPasteurisasi string  `json:"status_pasteurisasi"`
	Volume             float64 `json:"volume"`
	VolumeAfterCooling float64 `json:"volume_after_cooling"`
}

type FinishedBatchResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Total   int                 `json:"total"`
	Data    []FinishedBatchItem `json:"data"`
}

func fetchFinishedPasteurBatches(startDateStr, endDateStr, shiftFilter string) []FinishedBatchItem {
	baseURL := os.Getenv("LARAVEL_API_URL")
	if baseURL == "" {
		baseURL = "http://10.11.10.130:8095"
	}

	queryParams := url.Values{}
	if startDateStr != "" {
		queryParams.Set("start_date", startDateStr)
	}
	if endDateStr != "" {
		queryParams.Set("end_date", endDateStr)
	}
	if shiftFilter != "" {
		queryParams.Set("shift", shiftFilter)
	}
	queryParams.Set("line", "1")

	primaryEndpoint := fmt.Sprintf("%s/api/monitoring-pasteurisasi/finished-batches?%s", baseURL, queryParams.Encode())
	fallbackEndpoint := fmt.Sprintf("http://127.0.0.1:8000/api/monitoring-pasteurisasi/finished-batches?%s", queryParams.Encode())

	client := &http.Client{Timeout: 3 * time.Second}

	// 1. Coba primary endpoint terlebih dahulu (server)
	batches, err := tryFetchBatches(client, primaryEndpoint)
	if err == nil {
		return batches
	}

	// 2. Fallback ke endpoint lokal jika primary belum tersedia / timeout
	batches, err = tryFetchBatches(client, fallbackEndpoint)
	if err == nil {
		return batches
	}

	return nil
}

func tryFetchBatches(client *http.Client, targetURL string) ([]FinishedBatchItem, error) {
	resp, err := client.Get(targetURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp FinishedBatchResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	return apiResp.Data, nil
}

func appendSheetProsesMasakPasteur(f *excelize.File, allSensors []models.SensorPasteurisasi, startDateStr, endDateStr, shiftFilter string) {
	sheet2 := "Proses masak Pasteur"
	f.NewSheet(sheet2)

	// Style header kolom
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Size:  10,
			Color: "000000",
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "D9D9D9", Style: 1},
			{Type: "right", Color: "D9D9D9", Style: 1},
			{Type: "top", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	// Style data batch (kolom A-F)
	batchStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Size:  10,
			Color: "000000",
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "top",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "D9D9D9", Style: 1},
			{Type: "right", Color: "D9D9D9", Style: 1},
			{Type: "top", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	// Style data sensor (kolom G-J)
	sensorStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Size:  10,
			Color: "000000",
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "D9D9D9", Style: 1},
			{Type: "right", Color: "D9D9D9", Style: 1},
			{Type: "top", Color: "D9D9D9", Style: 1},
			{Type: "bottom", Color: "D9D9D9", Style: 1},
		},
	})

	// Set Header Kolom pada Row 3
	headers := []struct {
		Col   string
		Label string
		Width float64
	}{
		{"A", "Po", 16},
		{"B", "Batch", 12},
		{"C", "varian", 14},
		{"D", "storage", 12},
		{"E", "jam mulai pasteur", 18},
		{"F", "jam selesai pasteur", 18},
		{"G", "waktu logger", 16},
		{"H", "suhu holding", 14},
		{"I", "suhu heating", 14},
		{"J", "flowrate", 14},
	}

	for _, h := range headers {
		cell := h.Col + "3"
		f.SetCellValue(sheet2, cell, h.Label)
		f.SetCellStyle(sheet2, cell, cell, headerStyle)
		f.SetColWidth(sheet2, h.Col, h.Col, h.Width)
	}
	f.SetRowHeight(sheet2, 3, 24)

	// Ambil data batch yang telah selesai dari API Laravel
	batches := fetchFinishedPasteurBatches(startDateStr, endDateStr, shiftFilter)

	if len(batches) == 0 {
		f.SetCellValue(sheet2, "A4", "Tidak ada data proses masak pasteurisasi yang selesai pada periode ini")
		f.MergeCell(sheet2, "A4", "J4")
		emptyStyle, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Italic: true, Color: "888888"},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		f.SetCellStyle(sheet2, "A4", "J4", emptyStyle)
		f.SetRowHeight(sheet2, 4, 24)
		return
	}

	currentRow := 4
	for _, batch := range batches {
		batchStartRow := currentRow

		// Optimasi performa: Filter data sensor dari memori yang sudah di-load di Sheet 1 (0 extra query ke database!)
		var sensors []models.SensorPasteurisasi
		if batch.WaktuMulaiFull != "" && batch.WaktuSelesaiFull != "" {
			tMulai, err1 := time.ParseInLocation("2006-01-02 15:04:05", batch.WaktuMulaiFull, jakartaLoc)
			tSelesai, err2 := time.ParseInLocation("2006-01-02 15:04:05", batch.WaktuSelesaiFull, jakartaLoc)
			if err1 == nil && err2 == nil {
				for _, s := range allSensors {
					wib := toWIBTime(s.Waktu)
					if (wib.Equal(tMulai) || wib.After(tMulai)) && (wib.Equal(tSelesai) || wib.Before(tSelesai)) {
						sensors = append(sensors, s)
					}
				}
			}

			// Fallback ke database jika data sensor belum ada di memori
			if len(sensors) == 0 && len(allSensors) == 0 {
				config.DB.
					Where("Waktu >= ? AND Waktu <= ?", batch.WaktuMulaiFull, batch.WaktuSelesaiFull).
					Order("Waktu ASC").
					Find(&sensors)
			}
		}

		if len(sensors) == 0 {
			// Jika tidak ada sensor log di detik tersebut, tulis 1 baris metadata batch
			f.SetCellValue(sheet2, fmt.Sprintf("A%d", currentRow), batch.PONumber)
			f.SetCellValue(sheet2, fmt.Sprintf("B%d", currentRow), batch.BatchRange)
			f.SetCellValue(sheet2, fmt.Sprintf("C%d", currentRow), batch.Variant)
			f.SetCellValue(sheet2, fmt.Sprintf("D%d", currentRow), batch.Storage)
			f.SetCellValue(sheet2, fmt.Sprintf("E%d", currentRow), batch.JamMulaiPasteur)
			f.SetCellValue(sheet2, fmt.Sprintf("F%d", currentRow), batch.JamSelesaiPasteur)

			for _, col := range []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"} {
				cell := fmt.Sprintf("%s%d", col, currentRow)
				f.SetCellStyle(sheet2, cell, cell, batchStyle)
			}
			f.SetRowHeight(sheet2, currentRow, 18)
			currentRow++
		} else {
			// Tulis data sensor baris demi baris
			for _, s := range sensors {
				row := currentRow
				wib := toWIBTime(s.Waktu)

				f.SetCellValue(sheet2, fmt.Sprintf("G%d", row), wib.Format("15:04:05"))
				f.SetCellValue(sheet2, fmt.Sprintf("H%d", row), s.SuhuHolding)
				f.SetCellValue(sheet2, fmt.Sprintf("I%d", row), s.SuhuHeating)
				f.SetCellValue(sheet2, fmt.Sprintf("J%d", row), s.Flowrate)

				for _, col := range []string{"G", "H", "I", "J"} {
					cell := fmt.Sprintf("%s%d", col, row)
					f.SetCellStyle(sheet2, cell, cell, sensorStyle)
				}
				f.SetRowHeight(sheet2, row, 18)
				currentRow++
			}

			batchEndRow := currentRow - 1

			// Isi data batch pada batchStartRow
			f.SetCellValue(sheet2, fmt.Sprintf("A%d", batchStartRow), batch.PONumber)
			f.SetCellValue(sheet2, fmt.Sprintf("B%d", batchStartRow), batch.BatchRange)
			f.SetCellValue(sheet2, fmt.Sprintf("C%d", batchStartRow), batch.Variant)
			f.SetCellValue(sheet2, fmt.Sprintf("D%d", batchStartRow), batch.Storage)
			f.SetCellValue(sheet2, fmt.Sprintf("E%d", batchStartRow), batch.JamMulaiPasteur)
			f.SetCellValue(sheet2, fmt.Sprintf("F%d", batchStartRow), batch.JamSelesaiPasteur)

			// Merge kolom A-F secara vertikal bila baris sensor lebih dari 1
			if batchEndRow > batchStartRow {
				f.MergeCell(sheet2, fmt.Sprintf("A%d", batchStartRow), fmt.Sprintf("A%d", batchEndRow))
				f.MergeCell(sheet2, fmt.Sprintf("B%d", batchStartRow), fmt.Sprintf("B%d", batchEndRow))
				f.MergeCell(sheet2, fmt.Sprintf("C%d", batchStartRow), fmt.Sprintf("C%d", batchEndRow))
				f.MergeCell(sheet2, fmt.Sprintf("D%d", batchStartRow), fmt.Sprintf("D%d", batchEndRow))
				f.MergeCell(sheet2, fmt.Sprintf("E%d", batchStartRow), fmt.Sprintf("E%d", batchEndRow))
				f.MergeCell(sheet2, fmt.Sprintf("F%d", batchStartRow), fmt.Sprintf("F%d", batchEndRow))
			}

			// Terapkan style batch ke seluruh cell A-F
			for r := batchStartRow; r <= batchEndRow; r++ {
				for _, col := range []string{"A", "B", "C", "D", "E", "F"} {
					cell := fmt.Sprintf("%s%d", col, r)
					f.SetCellStyle(sheet2, cell, cell, batchStyle)
				}
			}
		}
	}

	// Freeze row 3 agar header tetap terlihat saat scroll
	f.SetPanes(sheet2, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      3,
		TopLeftCell: "A4",
		ActivePane:  "bottomLeft",
	})
}
