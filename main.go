package main

import (
    "backend-golang/config"
    "backend-golang/routes"
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "log"
    "net/http"
)

// CORS Middleware
func CORSMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Access-Control-Allow-Origin", "*")
        c.Header("Access-Control-Allow-Credentials", "true")
        c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
        c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }

        c.Next()
    }
}

func main() {
    godotenv.Load() // load .env
    config.ConnectDB() // connect ke DB (tanpa return value)

    r := gin.Default()
    
    // Use CORS middleware
    r.Use(CORSMiddleware())

    // Health check endpoint
    r.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "status":  "ok",
            "message": "Server is running",
        })
    })

    // Register routes
    routes.RegisterRetailRoutes(r)

    log.Println("Server running on 0.0.0.0:8080")
    if err := r.Run("0.0.0.0:8080"); err != nil {
        log.Fatal("Failed to start server:", err)
    }
}