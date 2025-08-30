package routes

import (
    "retail-runtime/controllers"
    "github.com/gin-gonic/gin"
)

func RegisterRetailRoutes(r *gin.Engine) {
    api := r.Group("/api/retail-d5")
    {
        api.GET("/durasi", controllers.DurasiStartMesinRealtime)
    }
}