package service

import (
	"fmt"
	"log"

	miniflux "miniflux.app/v2/client"
)

type MinifluxService struct {
	client *miniflux.Client
}

func NewMinifluxService(apiURL, apiToken string) *MinifluxService {
	if apiURL == "" || apiToken == "" {
		log.Println("WARNING: Miniflux API URL or token not configured. Entry marking will be skipped.")
		return &MinifluxService{client: nil}
	}

	client := miniflux.NewClient(apiURL, apiToken)
	return &MinifluxService{client: client}
}

func (s *MinifluxService) MarkEntryAsRead(entryID int) error {
	if s.client == nil {
		log.Printf("Miniflux client not configured, skipping mark as read for entry %d", entryID)
		return nil
	}

	err := s.client.UpdateEntries([]int64{int64(entryID)}, "read")
	if err != nil {
		return fmt.Errorf("failed to mark entry %d as read: %w", entryID, err)
	}

	log.Printf("Entry %d marked as read in Miniflux", entryID)
	return nil
}