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

    routes.RegisterRetailRoutesD1(r)
    routes.RegisterRetailRoutesD2(r)
    routes.RegisterRetailRoutesD3(r)
    routes.RegisterRetailRoutesD4(r)
    routes.RegisterRetailRoutesD5(r)
    routes.RegisterRetailRoutesD6(r)
    routes.RegisterRetailRoutesD7(r)
    routes.RegisterRetailRoutesD8(r)
    routes.RegisterRetailRoutesD9(r)
    routes.RegisterRetailRoutesD10(r)
    routes.RegisterRetailRoutesD14(r)

    log.Println("Server running on :8080")
    r.Run(":8080")
}