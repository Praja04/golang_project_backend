package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"backend-golang/config"
	"backend-golang/models"

	"github.com/gin-gonic/gin"
)

// Jakarta timezone
var jakartaLoc *time.Location

func init() {
	var err error
	jakartaLoc, err = time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Fallback ke UTC+7 jika gagal load
		jakartaLoc = time.FixedZone("WIB", 7*60*60)
	}
}

// Response structure untuk average data
type AvgResponse struct {
	Timestamp string  `json:"timestamp"`
	Average   float64 `json:"average"`
}

// GetLatestPasteurData -> ambil data terbaru
func GetLatestPasteurData(c *gin.Context) {
	var data models.SensorPasteurisasi

	if err := config.DB.Order("Waktu desc").First(&data).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Data pasteurisasi tidak ditemukan",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// GetPasteurDataPerHour -> ambil data per jam (tepat menit:detik = 00:00)
func GetPasteurDataPerHour(c *gin.Context) {
	tanggal := c.Query("tanggal") // format: YYYY-MM-DD
	if tanggal == "" {
		tanggal = time.Now().In(jakartaLoc).Format("2006-01-02")
	}

	var data []models.SensorPasteurisasi
	if err := config.DB.
		Where("DATE(Waktu) = ?", tanggal).
		Where("MINUTE(Waktu) = 0 AND SECOND(Waktu) = 0").
		Order("Waktu asc").
		Find(&data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil data",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"tanggal": tanggal,
		"count":   len(data),
		"data":    data,
	})
}

// GetPasteurAbnormal -> ambil periode suhu heating/holding di luar batas
func GetPasteurAbnormal(c *gin.Context) {
	tanggal := c.Query("tanggal") // YYYY-MM-DD
	if tanggal == "" {
		tanggal = time.Now().In(jakartaLoc).Format("2006-01-02")
	}

	// hasil query mentah
	type rawPeriod struct {
		Start      time.Time `json:"start"`
		End        time.Time `json:"end"`
		MinHeating float64   `json:"min_heating"`
		MaxHeating float64   `json:"max_heating"`
		MinHolding float64   `json:"min_holding"`
		MaxHolding float64   `json:"max_holding"`
	}

	// hasil akhir yang dikirim ke frontend
	type AbnormalPeriod struct {
		Start       string `json:"start"`
		End         string `json:"end"`
		SuhuHeating string `json:"suhu_heating,omitempty"`
		SuhuHolding string `json:"suhu_holding,omitempty"`
	}

	var raws []rawPeriod
	var result []AbnormalPeriod

	query := `
	WITH flagged AS (
		SELECT Waktu, SuhuHeating, SuhuHolding
		FROM readsensors_pasteurisasi1
		WHERE DATE(Waktu) = ?
	),
	with_groups AS (
		SELECT *,
			ROW_NUMBER() OVER (ORDER BY Waktu) -
			ROW_NUMBER() OVER (PARTITION BY (SuhuHeating < 105 OR SuhuHeating > 120) ORDER BY Waktu) AS grp_heat,
			ROW_NUMBER() OVER (ORDER BY Waktu) -
			ROW_NUMBER() OVER (PARTITION BY (SuhuHolding < 105 OR SuhuHolding > 120) ORDER BY Waktu) AS grp_hold
		FROM flagged
	)
	SELECT 
		MIN(Waktu) AS start,
		MAX(Waktu) AS end,
		MIN(SuhuHeating) AS min_heating,
		MAX(SuhuHeating) AS max_heating,
		MIN(SuhuHolding) AS min_holding,
		MAX(SuhuHolding) AS max_holding
	FROM with_groups
	GROUP BY grp_heat, grp_hold
	ORDER BY start;
	`

	if err := config.DB.Raw(query, tanggal).Scan(&raws).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Gagal mengambil data abnormal",
			"error":   err.Error(),
		})
		return
	}

	// mapping ke hasil akhir
	for _, r := range raws {
		ab := AbnormalPeriod{
			Start: r.Start.Format("2006-01-02 15:04:05"),
			End:   r.End.Format("2006-01-02 15:04:05"),
		}

		if r.MaxHeating > 120 {
			ab.SuhuHeating = ">120"
		} else if r.MinHeating < 105 {
			ab.SuhuHeating = "<105"
		}

		if r.MaxHolding > 120 {
			ab.SuhuHolding = ">120"
		} else if r.MinHolding < 105 {
			ab.SuhuHolding = "<105"
		}

		if ab.SuhuHeating != "" || ab.SuhuHolding != "" {
			result = append(result, ab)
		}
	}

	// encode JSON tanpa escape < >
	c.Writer.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(c.Writer)
	enc.SetEscapeHTML(false)
	enc.Encode(gin.H{
		"success": true,
		"tanggal": tanggal,
		"count":   len(result),
		"data":    result,
	})
}

// getTimeRange - Helper function untuk mendapatkan start dan end time (Asia/Jakarta)
func getTimeRange(c *gin.Context) (time.Time, time.Time, error) {
	// Default: 8 jam terakhir (waktu Jakarta)
	endTime := time.Now().In(jakartaLoc)
	startTime := endTime.Add(-8 * time.Hour)

	// Parse start_date jika ada (format: 2006-01-02 15:04:05 atau 2006-01-02T15:04:05)
	if startStr := c.Query("start_date"); startStr != "" {
		parsed, err := time.ParseInLocation("2006-01-02 15:04:05", startStr, jakartaLoc)
		if err != nil {
			// Coba format ISO 8601
			parsed, err = time.ParseInLocation("2006-01-02T15:04:05", startStr, jakartaLoc)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
		}
		startTime = parsed
	}

	// Parse end_date jika ada
	if endStr := c.Query("end_date"); endStr != "" {
		parsed, err := time.ParseInLocation("2006-01-02 15:04:05", endStr, jakartaLoc)
		if err != nil {
			// Coba format ISO 8601
			parsed, err = time.ParseInLocation("2006-01-02T15:04:05", endStr, jakartaLoc)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
		}
		endTime = parsed
	}

	return startTime, endTime, nil
}

// GetAverageFlowrate - Menampilkan rata-rata flowrate per menit
func GetAverageFlowrate(c *gin.Context) {
	var results []AvgResponse

	// Dapatkan time range
	startTime, endTime, err := getTimeRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid date format. Use: YYYY-MM-DD HH:MM:SS or YYYY-MM-DDTHH:MM:SS",
			"details": err.Error(),
		})
		return
	}

	// Query untuk menghitung average per menit dengan filter tanggal
	// Data di database sudah dalam timezone WIB, jadi tidak perlu CONVERT_TZ
	// Format waktu ke string untuk memastikan MySQL menerima waktu yang tepat
	startStr := startTime.Format("2006-01-02 15:04:05")
	endStr := endTime.Format("2006-01-02 15:04:05")
	
	query := `
		SELECT 
			DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00') as timestamp,
			AVG(Flowrate) as average
		FROM readsensors_pasteurisasi1
		WHERE Waktu >= ? AND Waktu <= ?
		GROUP BY DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00')
		ORDER BY timestamp ASC
	`

	if err := config.DB.Raw(query, startStr, endStr).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch average flowrate data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"count":  len(results),
		"filter": gin.H{
			"start_date": startTime.Format("2006-01-02 15:04:05"),
			"end_date":   endTime.Format("2006-01-02 15:04:05"),
			"timezone":   "Asia/Jakarta (WIB)",
		},
	})
}

// GetAverageSuhuHeating - Menampilkan rata-rata suhu heating per menit
func GetAverageSuhuHeating(c *gin.Context) {
	var results []AvgResponse

	// Dapatkan time range
	startTime, endTime, err := getTimeRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid date format. Use: YYYY-MM-DD HH:MM:SS or YYYY-MM-DDTHH:MM:SS",
			"details": err.Error(),
		})
		return
	}

	// Format waktu ke string untuk memastikan MySQL menerima waktu yang tepat
	startStr := startTime.Format("2006-01-02 15:04:05")
	endStr := endTime.Format("2006-01-02 15:04:05")
	
	query := `
		SELECT 
			DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00') as timestamp,
			AVG(SuhuHeating) as average
		FROM readsensors_pasteurisasi1
		WHERE Waktu >= ? AND Waktu <= ?
		GROUP BY DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00')
		ORDER BY timestamp ASC
	`

	if err := config.DB.Raw(query, startStr, endStr).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch average suhu heating data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"count":  len(results),
		"filter": gin.H{
			"start_date": startTime.Format("2006-01-02 15:04:05"),
			"end_date":   endTime.Format("2006-01-02 15:04:05"),
			"timezone":   "Asia/Jakarta (WIB)",
		},
	})
}

// GetAverageSuhuHolding - Menampilkan rata-rata suhu holding per menit
func GetAverageSuhuHolding(c *gin.Context) {
	var results []AvgResponse

	// Dapatkan time range
	startTime, endTime, err := getTimeRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid date format. Use: YYYY-MM-DD HH:MM:SS or YYYY-MM-DDTHH:MM:SS",
			"details": err.Error(),
		})
		return
	}

	// Format waktu ke string untuk memastikan MySQL menerima waktu yang tepat
	startStr := startTime.Format("2006-01-02 15:04:05")
	endStr := endTime.Format("2006-01-02 15:04:05")
	
	query := `
		SELECT 
			DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00') as timestamp,
			AVG(SuhuHolding) as average
		FROM readsensors_pasteurisasi1
		WHERE Waktu >= ? AND Waktu <= ?
		GROUP BY DATE_FORMAT(Waktu, '%Y-%m-%d %H:%i:00')
		ORDER BY timestamp ASC
	`

	if err := config.DB.Raw(query, startStr, endStr).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch average suhu holding data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"count":  len(results),
		"filter": gin.H{
			"start_date": startTime.Format("2006-01-02 15:04:05"),
			"end_date":   endTime.Format("2006-01-02 15:04:05"),
			"timezone":   "Asia/Jakarta (WIB)",
		},
	})
}