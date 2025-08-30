package main

import (
     "backend-golang/config"
    "backend-golang/routes"
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "log"
)

func main() {
    godotenv.Load()
    config.ConnectDB()

    r := gin.Default()

    routes.RegisterRetailRoutes(r)

    log.Println("Server running on :8080")
    r.Run(":8080")
}