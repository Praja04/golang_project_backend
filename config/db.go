package config

import (
	"log"
	"os"
	"time"

	"backend-golang/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	dsn := os.Getenv("DB_DSN")

	// Konfigurasi logger untuk nonaktifkan log lambat
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: time.Hour,     // ubah threshold jadi sangat tinggi
			LogLevel:      logger.Silent, // nonaktifkan semua log
			Colorful:      true,
		},
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	// Auto migrate tabel divert_logs_pasteur1 jika belum ada
	if err := db.AutoMigrate(&models.DivertLogPasteur1{}); err != nil {
		log.Println("Warning: Failed to auto migrate divert_logs_pasteur1:", err)
	}

	// Konfigurasi Connection Pool untuk melindungi server MySQL dari beban berlebih
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(25)                  // Maksimal koneksi terbuka
		sqlDB.SetMaxIdleConns(10)                  // Maksimal koneksi idle
		sqlDB.SetConnMaxLifetime(5 * time.Minute)  // Daur ulang koneksi tiap 5 menit
		sqlDB.SetConnMaxIdleTime(2 * time.Minute)  // Tutup koneksi idle setelah 2 menit
	}

	DB = db
	log.Println("Database connected (Connection pool configured)")
}