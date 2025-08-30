package routes

import (
    "backend-golang/controllers"
    "github.com/gin-gonic/gin"
)

func RegisterRetailRoutes(r *gin.Engine) {
    api := r.Group("/api/retail-d5")
    {
        api.GET("/durasi", controllers.UptimeStartMesinRealtime)
    }
}