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
	}
}
