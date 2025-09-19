package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"backend-golang/config"
	"backend-golang/models"

	"github.com/gin-gonic/gin"
)

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
		tanggal = time.Now().Format("2006-01-02")
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
		tanggal = time.Now().Format("2006-01-02")
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



