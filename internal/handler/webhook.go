package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"lewdarchive/internal/config"
	"lewdarchive/internal/model"
	"lewdarchive/internal/repository"
	"lewdarchive/internal/service"
)

type WebhookHandler struct {
	config          config.Config
	postRepo        *repository.PostRepository
	archiveService  *service.ArchiveService
	minifluxService *service.MinifluxService
	discordService  *service.DiscordService
}

func NewWebhookHandler(cfg config.Config, postRepo *repository.PostRepository, archiveService *service.ArchiveService, minifluxService *service.MinifluxService, discordService *service.DiscordService) *WebhookHandler {
	return &WebhookHandler{
		config:          cfg,
		postRepo:        postRepo,
		archiveService:  archiveService,
		minifluxService: minifluxService,
		discordService:  discordService,
	}
}

func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if h.config.MinifluxSecretKey != "" {
		signature := r.Header.Get("X-Miniflux-Signature")
		if !h.verifySignature(body, signature) {
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

	var payload model.WebhookPayload
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
		if err := h.processEntry(payload.Feed, entry); err != nil {
			log.Printf("Error processing entry %s: %v", entry.Hash, err)
			continue
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) processEntry(feed model.Feed, entry model.Entry) error {
	exists, err := h.postRepo.ExistsByHash(entry.Hash)
	if err != nil {
		return err
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

	post := &model.Post{
		SiteURL:       feed.SiteURL,
		EntryID:       entry.ID,
		Hash:          entry.Hash,
		Title:         entry.Title,
		URL:           entry.URL,
		PublishedAt:   publishedAt,
		Content:       entry.Content,
		Author:        entry.Author,
		CategoryID:    feed.Category.ID,
		CategoryTitle: feed.Category.Title,
	}

	if err := h.postRepo.Create(post); err != nil {
		return err
	}

	log.Printf("Post saved: %s - %s", entry.Title, entry.Hash)

	if err := h.minifluxService.MarkEntryAsRead(entry.ID); err != nil {
		log.Printf("Error marking entry %d as read: %v", entry.ID, err)
	}

	go h.archiveService.DownloadContent(entry.URL, entry.Author, feed.Category.Title, entry.Title, publishedAt, entry.Hash)

	if h.discordService != nil {
		if err := h.discordService.SendEmbed(feed, entry); err != nil {
			log.Printf("Error sending Discord notification for entry %s: %v", entry.Hash, err)
		}
	}

	return nil
}

func (h *WebhookHandler) verifySignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}

	signature = strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(h.config.MinifluxSecretKey))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}