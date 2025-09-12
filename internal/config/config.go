package config

import "os"

type Config struct {
	Port               string
	DBPath             string
	MinifluxSecretKey  string
	MinifluxAPIURL     string
	MinifluxAPIToken   string
	ArchiveDir         string
	DiscordWebhookURL  string
	ChibisafeAPIURL    string
	ChibisafeAPIKey    string
	CleanupAfterUpload bool
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		DBPath:             getEnv("DB_PATH", "./data/lewdarchive.db"),
		MinifluxSecretKey:  getEnv("MINIFLUX_SECRET", ""),
		MinifluxAPIURL:     getEnv("MINIFLUX_API_URL", ""),
		MinifluxAPIToken:   getEnv("MINIFLUX_API_TOKEN", ""),
		ArchiveDir:         getEnv("ARCHIVE_DIR", "./data/archive"),
		DiscordWebhookURL:  getEnv("DISCORD_WEBHOOK_URL", ""),
		ChibisafeAPIURL:    getEnv("CHIBISAFE_API_URL", ""),
		ChibisafeAPIKey:    getEnv("CHIBISAFE_API_KEY", ""),
		CleanupAfterUpload: getBoolEnv("CLEANUP_AFTER_UPLOAD", false),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "1" || value == "yes"
}