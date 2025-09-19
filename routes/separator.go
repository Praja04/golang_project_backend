package routes

import (
	"backend-golang/controllers"
	"github.com/gin-gonic/gin"
)

func RegisterSeparatorRoutes(r *gin.Engine) {
	api := r.Group("/api/separator")
	{
		// Real-time monitoring endpoints
		api.GET("/latest", controllers.GetLatestSeparatorData)
		api.GET("/status", controllers.GetSeparatorStatus)
		api.GET("/history", controllers.GetSeparatorHistoryByDate)
		api.GET("/logs", controllers.GetSeparatorLogs)
		api.GET("/sensor", controllers.GetSeparatorSensorByShift)
	}
}
