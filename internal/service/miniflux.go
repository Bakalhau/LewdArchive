package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type MinifluxService struct {
	apiURL   string
	apiToken string
	client   *http.Client
}

func NewMinifluxService(apiURL, apiToken string) *MinifluxService {
	if apiURL == "" || apiToken == "" {
		log.Println("WARNING: Miniflux API URL or token not configured. Entry marking will be skipped.")
		return &MinifluxService{
			apiURL:   apiURL,
			apiToken: apiToken,
			client:   nil,
		}
	}

	apiURL = strings.TrimSuffix(apiURL, "/")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  true,
			DisableKeepAlives:   false, 
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	return &MinifluxService{
		apiURL:   apiURL,
		apiToken: apiToken,
		client:   client,
	}
}

func (s *MinifluxService) MarkEntryAsRead(entryID int) error {
	if s.client == nil {
		log.Printf("Miniflux client not configured, skipping mark as read for entry %d", entryID)
		return nil
	}

	requestBody := map[string]interface{}{
		"entry_ids": []int64{int64(entryID)},
		"status":    "read",
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	log.Printf("Sending body to Miniflux for entry %d: %s", entryID, string(jsonBody))

	url := fmt.Sprintf("%s/entries", s.apiURL)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", s.apiToken)
	req.Header.Set("User-Agent", "LewdArchive/1.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connection", "keep-alive")

	req.ContentLength = int64(len(jsonBody))

	var resp *http.Response
	maxRetries := 5 
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err = s.client.Do(req)
		if err == nil {
			break
		}

		log.Printf("Attempt %d failed for entry %d: %v", attempt, entryID, err)
		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * 2 * time.Second) 

			req, err = http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
			if err != nil {
				return fmt.Errorf("failed to recreate request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Auth-Token", s.apiToken)
			req.Header.Set("User-Agent", "LewdArchive/1.0")
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Connection", "keep-alive")
			req.ContentLength = int64(len(jsonBody))
		}
	}

	if err != nil {
		return fmt.Errorf("failed to send request after %d attempts: %w", maxRetries, err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Warning: Failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		log.Printf("Miniflux API response - Status: %d, Body: %s", resp.StatusCode, string(responseBody))
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(responseBody))
	}

	log.Printf("Entry %d successfully marked as read in Miniflux (Status: %d)", entryID, resp.StatusCode)
	return nil
}
