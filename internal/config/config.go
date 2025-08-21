package config

import "os"

type Config struct {
	Port                string
	DBPath              string
	MinifluxSecretKey   string
	MinifluxAPIURL      string
	MinifluxAPIToken    string
	ArchiveDir          string
	DiscordWebhookURL   string
}

func Load() Config {
	return Config{
		Port:              getEnv("PORT", "8080"),
		DBPath:            getEnv("DB_PATH", "./data/lewdarchive.db"),
		MinifluxSecretKey: getEnv("MINIFLUX_SECRET", ""),
		MinifluxAPIURL:    getEnv("MINIFLUX_API_URL", ""),
		MinifluxAPIToken:  getEnv("MINIFLUX_API_TOKEN", ""),
		ArchiveDir:        getEnv("ARCHIVE_DIR", "./data/archive"),
		DiscordWebhookURL: getEnv("DISCORD_WEBHOOK_URL", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}