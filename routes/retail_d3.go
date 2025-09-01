package routes

import (
    "backend-golang/controllers"
    "github.com/gin-gonic/gin"
)

func RegisterRetailRoutesD3(r *gin.Engine) {
    api := r.Group("/api/retail-d3")
    {
        api.GET("/durasi/start", controllers.UptimeStartMesinRealtime)
        api.GET("/durasi/stop", controllers.DowntimeStopMesinRealtime)
        api.GET("/performance-output", controllers.PerformanceOutput)
        api.GET("/output-gagal-filling", controllers.OutputGagalFilling)
    }
}