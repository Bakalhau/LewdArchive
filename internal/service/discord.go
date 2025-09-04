package service

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"lewdarchive/internal/model"
)

type DiscordService struct {
	webhookURL string
}

func NewDiscordService(webhookURL string) *DiscordService {
	if webhookURL == "" {
		return nil
	}
	return &DiscordService{webhookURL: webhookURL}
}

type RSSFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Image struct {
			URL string `xml:"url"`
		} `xml:"image"`
	} `xml:"channel"`
}

type AtomFeed struct {
	XMLName xml.Name `xml:"feed"`
	Logo    string   `xml:"logo"`
	Icon    string   `xml:"icon"`
}

type DiscordEmbed struct {
	Embeds      []Embed     `json:"embeds"`
	Attachments []struct{}  `json:"attachments"`
}

type Embed struct {
	Title     string      `json:"title"`
	URL       string      `json:"url"`
	Color     int         `json:"color"`
	Author    EmbedAuthor `json:"author"`
	Footer    EmbedFooter `json:"footer"`
	Timestamp string      `json:"timestamp"`
	Image     EmbedImage  `json:"image"`
}

type EmbedAuthor struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	IconURL string `json:"icon_url"`
}

type EmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url"`
}

type EmbedImage struct {
	URL string `json:"url"`
}

func getIconURL(feedURL string) string {
	resp, err := http.Get(feedURL)
	if err != nil {
		log.Printf("Error fetching feed: %v", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading feed body: %v", err)
		return ""
	}

	var rssFeed RSSFeed
	err = xml.Unmarshal(body, &rssFeed)
	if err == nil && rssFeed.Channel.Image.URL != "" {
		return rssFeed.Channel.Image.URL
	}

	// Try Atom
	var atomFeed AtomFeed
	err = xml.Unmarshal(body, &atomFeed)
	if err == nil {
		if atomFeed.Logo != "" {
			return atomFeed.Logo
		}
		if atomFeed.Icon != "" {
			return atomFeed.Icon
		}
	}

	log.Printf("No icon found in feed XML")
	return ""
}

func extractImageFromContent(content string) string {
	imgRegex := regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	matches := imgRegex.FindAllStringSubmatch(content, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			url := match[1]
			if isImageURL(url) {
				log.Printf("Found image from <img> tag: %s", url)
				return url
			}
		}
	}
	
	linkRegex := regexp.MustCompile(`<a[^>]+href="([^"]+\.(?:jpg|jpeg|png|gif|webp|bmp|svg))"`)
	matches = linkRegex.FindAllStringSubmatch(content, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			url := match[1]
			log.Printf("Found image from <a> tag: %s", url)
			return url
		}
	}
	
	generalImageRegex := regexp.MustCompile(`https?://[^\s"<>]+\.(?:jpg|jpeg|png|gif|webp|bmp|svg|tiff)`)
	matches = generalImageRegex.FindAllStringSubmatch(content, -1)
	
	if len(matches) > 0 {
		url := matches[0][0]
		log.Printf("Found image from general URL pattern: %s", url)
		return url
	}
	
	log.Printf("No image found in content: %s", content)
	return ""
}

func isImageURL(url string) bool {
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg", ".tiff"}
	urlLower := strings.ToLower(url)
	
	for _, ext := range imageExtensions {
		if strings.Contains(urlLower, ext) {
			return true
		}
	}
	return false
}

var categoryColors = map[string]int{
	"default": 0xFF69B4,
	"Patreon": 0xFF5900,
	"Fanbox": 0xFAF18A,
	"SubscribeStar": 0x009587,
	"Mastodon": 0x563ACC,
	"Bluesky": 0x1185FE,
	"X": 0x000000,
}

var categoryIcons = map[string]string{
	"default": "https://i.imgur.com/Nyh7tRG.png",
	"Patreon": "https://i.imgur.com/07HA8CQ.png",
	"Fanbox": "https://i.imgur.com/uXT06Tq.png",
	"SubscribeStar": "https://i.imgur.com/San8fH3.png",
	"Bluesky": "https://i.imgur.com/1mcXqLF.png",
	"Mastodon": "https://i.imgur.com/tUeKKz2.png",
	"X": "https://i.imgur.com/wXxVrmo.png",
}

func (s *DiscordService) SendEmbed(feed model.Feed, entry model.Entry) error {
	iconURL := getIconURL(feed.FeedURL)
	categoryTitle := feed.Category.Title
	if categoryTitle == "" {
		categoryTitle = "Uncategorized"
	}

	categoryColor, ok := categoryColors[categoryTitle]
	if !ok {
		categoryColor = categoryColors["default"]
	}

	categoryIcon, ok := categoryIcons[categoryTitle]
	if !ok {
		categoryIcon = categoryIcons["default"]
	}

	if iconURL == "" {
		iconURL = categoryIcon
	}

	var imageURL string
	for _, enc := range entry.Enclosures {
		if strings.HasPrefix(enc.MimeType, "image/") {
			imageURL = enc.URL
			break
		}
	}
	if imageURL == "" {
		imageURL = extractImageFromContent(entry.Content)
	}
	if imageURL == "" {
		imageURL = "https://i.imgur.com/5zcBLRc.png"
	}

	embed := DiscordEmbed{
		Embeds: []Embed{{
			Title: entry.Title,
			URL:   entry.URL,
			Color: categoryColor,
			Author: EmbedAuthor{
				Name:    entry.Author,
				URL:     feed.SiteURL,
				IconURL: iconURL,
			},
			Footer: EmbedFooter{
				Text:    categoryTitle,
				IconURL: categoryIcon,
			},
			Timestamp: entry.PublishedAt,
			Image: EmbedImage{
				URL: imageURL,
			},
		}},
		Attachments: []struct{}{},
	}

	jsonData, err := json.Marshal(embed)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", s.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending webhook: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Printf("Discord notification sent for '%s'", entry.Title)
	time.Sleep(5 * time.Second)
	return nil
}