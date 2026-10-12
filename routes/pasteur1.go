package routes

import (
	"backend-golang/controllers"

	"github.com/gin-gonic/gin"
)

func RegisterPasteurRoutes(r *gin.Engine) {
	api := r.Group("/api/pasteur")
	{
		api.GET("/latest", controllers.GetLatestPasteurData)
		api.GET("/by-hour", controllers.GetPasteurDataPerHour)
		api.GET("/abnormal", controllers.GetPasteurAbnormal)
		api.GET("/average/flowrate", controllers.GetAverageFlowrate)
		api.GET("/average/suhu-heating", controllers.GetAverageSuhuHeating)
		api.GET("/average/suhu-holding", controllers.GetAverageSuhuHolding)

		// Divert Monitoring & Logs Pasteur 1
		api.GET("/divert/status", controllers.GetDivertStatusPasteur1)
		api.GET("/divert/summary", controllers.GetDivertSummaryPasteur1)
		api.GET("/divert/logs", controllers.GetDivertLogsPasteur1)
		api.POST("/divert/detect", controllers.DetectAndRecordDivertPasteur1)
		api.POST("/divert/logs", controllers.CreateDivertLogPasteur1)
	}
}
