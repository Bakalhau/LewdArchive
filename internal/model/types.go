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