package main

import (
    "backend-golang/config"
    "backend-golang/routes"
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "log"
)

func main() {
    godotenv.Load()          // load .env
    config.ConnectDB()       // connect ke DB (tanpa return value)

    r := gin.Default()
    routes.RegisterRetailRoutes(r)

    log.Println("Server running on 0.0.0.0:8080")
    if err := r.Run("0.0.0.0:8080"); err != nil {
        log.Fatal("Failed to start server:", err)
    }
}
