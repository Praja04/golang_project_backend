package routes

import (
	"backend-golang/controllers"
	"github.com/gin-gonic/gin"
)

func RegisterRetailRoutes(r *gin.Engine) {
	api := r.Group("/api/retail")
	{
		api.GET("/:line/durasi/start", controllers.UptimeStartMesinRealtime)
		api.GET("/:line/durasi/stop", controllers.DowntimeStopMesinRealtime)
		api.GET("/:line/performance-output", controllers.PerformanceOutput)
		api.GET("/:line/output-gagal-filling", controllers.OutputGagalFilling)
	}
}
