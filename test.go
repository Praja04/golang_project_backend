package main

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
)

func main() {
	// DSN tanpa password
	dsn := "admin:1q2w3e4r@tcp(10.11.11.200:3306)/retaildb?charset=utf8mb4&parseTime=True&loc=Local"

	// Coba buka koneksi
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("❌ Gagal koneksi ke MySQL:", err)
	}

	// Ping untuk pastikan koneksi aktif
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("❌ Gagal ambil DB instance:", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Fatal("❌ Ping ke MySQL gagal:", err)
	}

	log.Println("✅ Koneksi ke MySQL berhasil!")
}