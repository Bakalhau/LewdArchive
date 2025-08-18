package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"lewdarchive/internal/config"
	"lewdarchive/internal/handler"
	"lewdarchive/internal/repository"
	"lewdarchive/internal/service"
	"lewdarchive/pkg/database"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}

	cfg := config.Load()
	
	if cfg.MinifluxSecretKey == "" {
		log.Println("WARNING: MINIFLUX_SECRET is not set. HMAC verification will be skipped.")
	}

	db, err := database.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	if err := os.MkdirAll(cfg.ArchiveDir, 0755); err != nil {
		log.Fatal("Error creating archive directory:", err)
	}

	postRepo := repository.NewPostRepository(db)

	archiveService := service.NewArchiveService(cfg.ArchiveDir)
	minifluxService := service.NewMinifluxService(cfg.MinifluxAPIURL, cfg.MinifluxAPIToken)

	webhookHandler := handler.NewWebhookHandler(cfg, postRepo, archiveService, minifluxService)

	http.HandleFunc("/webhook", webhookHandler.HandleWebhook)
	http.HandleFunc("/health", healthHandler)

	log.Printf("🚀 Server starting on port %s", cfg.Port)
	log.Printf("💾 Database: %s", cfg.DBPath)
	log.Printf("📁 Archive directory: %s", cfg.ArchiveDir)
	log.Printf("")
	log.Printf("📡 Available endpoints:")
	log.Printf("   Health Check: http://localhost:%s/health", cfg.Port)
	log.Printf("   Webhook:      http://localhost:%s/webhook", cfg.Port)
	log.Printf("")
	log.Printf("✅ Server is ready to receive requests!")
	
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatal("⛔ Server failed to start:", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := map[string]interface{}{
		"status": "OK",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service": "lewdarchive",
		"version": "1.0.0",
	}
	
	json.NewEncoder(w).Encode(response)
}