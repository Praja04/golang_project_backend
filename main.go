package main

import (
    "backend-golang/config"
    "backend-golang/routes"
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "log"
)

func main() {
    // Load environment variables
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found")
    }

    // Connect to database
    if err := config.ConnectDB(); err != nil {
        log.Fatal("Failed to connect to database:", err)
    }

    // Initialize Gin router
    r := gin.Default()

    // Register routes
    routes.RegisterRetailRoutes(r)

    log.Println("Server running on 0.0.0.0:8080")
    
    // Explicitly listen on all interfaces
    if err := r.Run("0.0.0.0:8080"); err != nil {
        log.Fatal("Failed to start server:", err)
    }
}
