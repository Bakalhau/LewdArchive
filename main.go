package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

type WebhookPayload struct {
	EventType string    `json:"event_type"`
	Feed      Feed      `json:"feed"`
	Entries   []Entry   `json:"entries"`
}

type Feed struct {
	ID       int    `json:"id"`
	SiteURL  string `json:"site_url"`
	Title    string `json:"title"`
	FeedURL  string `json:"feed_url"`
}

type Entry struct {
	ID          int         `json:"id"`
	Hash        string      `json:"hash"`
	Title       string      `json:"title"`
	URL         string      `json:"url"`
	PublishedAt string      `json:"published_at"`
	Content     string      `json:"content"`
	Author      string      `json:"author"`
	Enclosures  []Enclosure `json:"enclosures"`
}

type Enclosure struct {
	ID       int    `json:"id"`
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
}

type Config struct {
	Port       string
	DBPath     string
	MinifluxSecretKey  string
	ArchiveDir string
}

type Server struct {
	db     *sql.DB
	config Config
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file:", err)
	}

	config := Config{
		Port:      getEnv("PORT", "8080"),
		DBPath:    getEnv("DB_PATH", "./data/lewdarchive.db"),
		MinifluxSecretKey: getEnv("MINIFLUX_SECRET", ""),
		ArchiveDir: getEnv("ARCHIVE_DIR", "./data/archive"),
	}

	if config.MinifluxSecretKey == "" {
		log.Println("WARNING: MINIFLUX_SECRET is not set. HMAC verification will be skipped.")
	}

	server, err := NewServer(config)
	if err != nil {
		log.Fatal("Error initializing server:", err)
	}
	defer server.db.Close()

	if err := os.MkdirAll(config.ArchiveDir, 0755); err != nil {
		log.Fatal("Error creating archive directory:", err)
	}

	http.HandleFunc("/webhook", server.handleWebhook)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Server started on port %s", config.Port)
	log.Printf("Database: %s", config.DBPath)
	log.Printf("Archive directory: %s", config.ArchiveDir)
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}

func NewServer(config Config) (*Server, error) {
	db, err := sql.Open("sqlite3", config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	server := &Server{
		db:     db,
		config: config,
	}

	if err := server.createTables(); err != nil {
		return nil, fmt.Errorf("error creating tables: %v", err)
	}

	return server, nil
}

func (s *Server) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		site_url TEXT NOT NULL,
		entry_id INTEGER NOT NULL,
		hash TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		published_at DATETIME NOT NULL,
		content TEXT,
		author TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		downloaded BOOLEAN DEFAULT FALSE
	);

	CREATE TABLE IF NOT EXISTS medias (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER NOT NULL,
		url TEXT NOT NULL,
		mime_type TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (post_id) REFERENCES posts (id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_posts_hash ON posts(hash);
	CREATE INDEX IF NOT EXISTS idx_posts_url ON posts(url);
	CREATE INDEX IF NOT EXISTS idx_medias_post_id ON medias(post_id);
	`

	_, err := s.db.Exec(query)
	return err
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if s.config.MinifluxSecretKey != "" {
		signature := r.Header.Get("X-Miniflux-Signature")
		if !s.verifySignature(body, signature) {
			log.Println("Invalid HMAC signature")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	eventType := r.Header.Get("X-Miniflux-Event-Type")
	if eventType != "new_entries" {
		log.Printf("Ignored event type: %s", eventType)
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.EventType != "new_entries" {
		log.Printf("Ignored event type in payload: %s", payload.EventType)
		w.WriteHeader(http.StatusOK)
		return
	}

	for _, entry := range payload.Entries {
		if err := s.processEntry(payload.Feed, entry); err != nil {
			log.Printf("Error processing entry %s: %v", entry.Hash, err)
			continue
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) processEntry(feed Feed, entry Entry) error {
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE hash = ?)", entry.Hash).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking existence: %v", err)
	}

	if exists {
		log.Printf("Entry already exists: %s", entry.Hash)
		return nil
	}

	publishedAt, err := time.Parse(time.RFC3339, entry.PublishedAt)
	if err != nil {
		log.Printf("Error parsing date %s: %v", entry.PublishedAt, err)
		publishedAt = time.Now()
	}

	result, err := s.db.Exec(`
		INSERT INTO posts (site_url, entry_id, hash, title, url, published_at, content, author)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, feed.SiteURL, entry.ID, entry.Hash, entry.Title, entry.URL, publishedAt, entry.Content, entry.Author)

	if err != nil {
		return fmt.Errorf("error inserting post: %v", err)
	}

	postID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("error getting post ID: %v", err)
	}

	for _, enclosure := range entry.Enclosures {
		_, err := s.db.Exec(`
			INSERT INTO medias (post_id, url, mime_type)
			VALUES (?, ?, ?)
		`, postID, enclosure.URL, enclosure.MimeType)

		if err != nil {
			log.Printf("Error inserting media: %v", err)
		}
	}

	log.Printf("Post saved: %s - %s", entry.Title, entry.Hash)
	go s.downloadWithGalleryDL(entry.URL, postID)

	return nil
}

func (s *Server) downloadWithGalleryDL(url string, postID int64) {
	log.Printf("Starting download for: %s", url)

	if _, err := exec.LookPath("gallery-dl"); err != nil {
		log.Printf("gallery-dl not found in PATH: %v", err)
		return
	}

	archiveDir := fmt.Sprintf("%s/post_%d", s.config.ArchiveDir, postID) // Renamed from OutputDir
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		log.Printf("Error creating directory %s: %v", archiveDir, err)
		return
	}

	cmd := exec.Command("gallery-dl",
		"--dest", archiveDir,
		"--no-mtime",
		url)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error in gallery-dl for %s: %v\nOutput: %s", url, err, string(output))
		return
	}

	_, err = s.db.Exec("UPDATE posts SET downloaded = TRUE WHERE id = ?", postID)
	if err != nil {
		log.Printf("Error marking post as downloaded: %v", err)
	}

	log.Printf("Download completed for: %s", url)
}

func (s *Server) verifySignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}

	signature = strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(s.config.MinifluxSecretKey))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}