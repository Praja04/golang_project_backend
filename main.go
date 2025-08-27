package main

import (
    "backend-golang/config"
    "backend-golang/controllers"
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "log"
)

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Println("Gagal load .env, lanjut tanpa environment file")
    }

    config.ConnectDB()

    r := gin.Default()
    r.GET("/api/retail-d5/durasi", controllers.DurasiStartMesinRealtime)
    r.Run(":8080")
}