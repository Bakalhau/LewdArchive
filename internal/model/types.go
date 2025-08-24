package model

import "time"

type WebhookPayload struct {
	EventType string  `json:"event_type"`
	Feed      Feed    `json:"feed"`
	Entries   []Entry `json:"entries"`
}

type Feed struct {
	ID       int      `json:"id"`
	SiteURL  string   `json:"site_url"`
	Title    string   `json:"title"`
	FeedURL  string   `json:"feed_url"`
	Category Category `json:"category"`
}

type Category struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
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

type Post struct {
	ID            int       `json:"id"`
	SiteURL       string    `json:"site_url"`
	EntryID       int       `json:"entry_id"`
	Hash          string    `json:"hash"`
	Title         string    `json:"title"`
	URL           string    `json:"url"`
	PublishedAt   time.Time `json:"published_at"`
	Content       string    `json:"content"`
	Author        string    `json:"author"`
	CategoryID    int       `json:"category_id"`
	CategoryTitle string    `json:"category_title"`
}

// Chibisafe types
type ChibisafeAlbumsResponse struct {
	Message string           `json:"message"`
	Albums  []ChibisafeAlbum `json:"albums"`
	Count   int              `json:"count"`
}

type ChibisafeAlbum struct {
	UUID        string      `json:"uuid"`
	Name        string      `json:"name"`
	Description interface{} `json:"description"`
	NSFW        bool        `json:"nsfw"`
	ZippedAt    interface{} `json:"zippedAt"`
	CreatedAt   string      `json:"createdAt"`
	EditedAt    string      `json:"editedAt"`
	Cover       string      `json:"cover"`
	Count       int         `json:"count"`
}

type ChibisafeCreateAlbumRequest struct {
	Name string `json:"name"`
}

type ChibisafeCreateAlbumResponse struct {
	Message string         `json:"message"`
	Album   ChibisafeAlbum `json:"album"`
}

type ChibisafeUploadRequest struct {
	Size        int64  `json:"size"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
}

type ChibisafeUploadResponse struct {
	Name       string `json:"name"`
	UUID       string `json:"uuid"`
	URL        string `json:"url"`
	Identifier string `json:"identifier"`
	PublicURL  string `json:"publicUrl"`
}

type ChibisafeTagsResponse struct {
	Message string          `json:"message"`
	Tags    []ChibisafeTag  `json:"tags"`
	Count   int             `json:"count"`
}

type ChibisafeTag struct {
	UUID  string `json:"uuid"`
	Name  string `json:"name"`
	Count struct {
		Files int `json:"files"`
	} `json:"_count"`
}

type ChibisafeCreateTagRequest struct {
	Name string `json:"name"`
}

type ChibisafeCreateTagResponse struct {
	Message string       `json:"message"`
	Tag     ChibisafeTag `json:"tag"`
}