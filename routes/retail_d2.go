package routes

import (
    "backend-golang/controllers"
    "github.com/gin-gonic/gin"
)

func RegisterRetailRoutesD2(r *gin.Engine) {
    api := r.Group("/api/retail-d2")
    {
        api.GET("/durasi/start", controllers.UptimeStartMesinRealtime)
        api.GET("/durasi/stop", controllers.DowntimeStopMesinRealtime)
        api.GET("/performance-output", controllers.PerformanceOutput)
        api.GET("/output-gagal-filling", controllers.OutputGagalFilling)
    }
}