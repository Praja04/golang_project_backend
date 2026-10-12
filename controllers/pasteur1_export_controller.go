package controllers

import (
	"fmt"
	"net/http"
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
	for i, s := range filtered {
		row := i + 5 // mulai dari baris 5
		wib := toWIBTime(s.Waktu)
		shiftNum := getShiftNumber(wib)
		weekday := wib.Weekday()

		// Tentukan apakah sedang divert
		isDivert, _, _ := services.CheckDivertTriggers(&s)

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

			// Warna baris divert merah muda
			if isDivert {
				f.SetCellStyle(sheet, cell, cell, divertStyle)
			} else if i%2 == 0 {
				f.SetCellStyle(sheet, cell, cell, dataStyleEven)
			} else {
				f.SetCellStyle(sheet, cell, cell, dataStyleOdd)
			}
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
