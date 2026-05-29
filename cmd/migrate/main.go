package main

import (
	"log"

	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/infrastructure/database"
)

// Standalone migration runner — useful in CI/CD pipelines.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("migration: %v", err)
	}

	log.Println("migrations completed successfully")
}
