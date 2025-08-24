package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"lewdarchive/internal/model"
	"lewdarchive/internal/utils"
)

type ChibisafeService struct {
	apiURL string
	apiKey string
	client *http.Client
}

func NewChibisafeService(apiURL, apiKey string) *ChibisafeService {
	if apiURL == "" || apiKey == "" {
		log.Println("WARNING: Chibisafe API URL or key not configured. Chibisafe uploads will be skipped.")
		return &ChibisafeService{
			apiURL: apiURL,
			apiKey: apiKey,
			client: &http.Client{},
		}
	}

	return &ChibisafeService{
		apiURL: strings.TrimSuffix(apiURL, "/"),
		apiKey: apiKey,
		client: &http.Client{},
	}
}

func (s *ChibisafeService) IsConfigured() bool {
	return s.apiURL != "" && s.apiKey != ""
}

func (s *ChibisafeService) UploadFiles(archiveDir, categoryTitle, author, title string) error {
	if !s.IsConfigured() {
		log.Printf("Chibisafe not configured, skipping upload for %s", archiveDir)
		return nil
	}

	// Get or create album
	albumUUID, err := s.getOrCreateAlbum(categoryTitle)
	if err != nil {
		return fmt.Errorf("failed to get/create album: %w", err)
	}

	tagUUID, err := s.getOrCreateTag(author)
	if err != nil {
		log.Printf("Warning: failed to get/create tag for %s: %v", author, err)
	}

	return s.uploadDirectoryFiles(archiveDir, albumUUID, tagUUID, title)
}

func (s *ChibisafeService) getOrCreateAlbum(categoryTitle string) (string, error) {
	albums, err := s.searchAlbums(categoryTitle)
	if err != nil {
		return "", err
	}

	for _, album := range albums {
		if strings.EqualFold(album.Name, categoryTitle) {
			log.Printf("Found existing album: %s (%s)", album.Name, album.UUID)
			return album.UUID, nil
		}
	}

	log.Printf("Creating new album: %s", categoryTitle)
	return s.createAlbum(categoryTitle)
}

func (s *ChibisafeService) searchAlbums(search string) ([]model.ChibisafeAlbum, error) {
	req, err := http.NewRequest("GET", s.apiURL+"/api/albums", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("search", search)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("x-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search albums failed: %d - %s", resp.StatusCode, string(body))
	}

	var response model.ChibisafeAlbumsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Albums, nil
}

func (s *ChibisafeService) createAlbum(name string) (string, error) {
	reqBody := model.ChibisafeCreateAlbumRequest{
		Name: name,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", s.apiURL+"/api/album/create", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create album failed: %d - %s", resp.StatusCode, string(body))
	}

	var response model.ChibisafeCreateAlbumResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	log.Printf("Created album: %s (%s)", response.Album.Name, response.Album.UUID)
	return response.Album.UUID, nil
}

func (s *ChibisafeService) getOrCreateTag(author string) (string, error) {
	tags, err := s.searchTags(author)
	if err != nil {
		return "", err
	}

	for _, tag := range tags {
		if strings.EqualFold(tag.Name, author) {
			log.Printf("Found existing tag: %s (%s)", tag.Name, tag.UUID)
			return tag.UUID, nil
		}
	}

	log.Printf("Creating new tag: %s", author)
	return s.createTag(author)
}

func (s *ChibisafeService) searchTags(search string) ([]model.ChibisafeTag, error) {
	req, err := http.NewRequest("GET", s.apiURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("search", search)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("x-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search tags failed: %d - %s", resp.StatusCode, string(body))
	}

	var response model.ChibisafeTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Tags, nil
}

func (s *ChibisafeService) createTag(name string) (string, error) {
	reqBody := model.ChibisafeCreateTagRequest{
		Name: name,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", s.apiURL+"/api/tag/create", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create tag failed: %d - %s", resp.StatusCode, string(body))
	}

	var response model.ChibisafeCreateTagResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	log.Printf("Created tag: %s (%s)", response.Tag.Name, response.Tag.UUID)
	return response.Tag.UUID, nil
}

func (s *ChibisafeService) uploadDirectoryFiles(dirPath, albumUUID, tagUUID, title string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	var supportedFiles []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !s.isSupportedFile(entry.Name()) {
			log.Printf("Skipping non-supported file: %s", entry.Name())
			continue
		}
		supportedFiles = append(supportedFiles, entry)
	}

	if len(supportedFiles) == 0 {
		return nil
	}

	sanitizedTitle := utils.SanitizeForPath(title)
	if sanitizedTitle == "" {
		sanitizedTitle = "unknown"
	}

	for i, entry := range supportedFiles {
		filePath := filepath.Join(dirPath, entry.Name())
		ext := filepath.Ext(entry.Name())
		var filename string
		if len(supportedFiles) == 1 {
			filename = sanitizedTitle + ext
		} else {
			filename = fmt.Sprintf("%s-%d%s", sanitizedTitle, i+1, ext)
		}

		log.Printf("Uploading file: %s as %s", entry.Name(), filename)
		fileUUID, err := s.uploadFile(filePath, filename, albumUUID)
		if err != nil {
			log.Printf("Error uploading file %s: %v", filename, err)
			continue 
		}

		if tagUUID != "" {
			if err := s.addTagToFile(fileUUID, tagUUID); err != nil {
				log.Printf("Error adding tag to file %s: %v", filename, err)
			}
		}
	}

	return nil
}

func (s *ChibisafeService) isSupportedFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	supportedExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tiff", ".svg", ".mp4"}
	
	for _, suppExt := range supportedExts {
		if ext == suppExt {
			return true
		}
	}
	return false
}

func (s *ChibisafeService) uploadFile(filePath, filename, albumUUID string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	ext := filepath.Ext(filename)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	headers := textproto.MIMEHeader{}
	headers.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename="%s"`, filename))
	headers.Set("Content-Type", contentType)

	part, err := writer.CreatePart(headers)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", s.apiURL+"/api/upload", &buf)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-api-key", s.apiKey)
	req.Header.Set("albumuuid", albumUUID)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed: %d - %s", resp.StatusCode, string(body))
	}

	var response model.ChibisafeUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	log.Printf("Uploaded file: %s (%s)", response.Name, response.UUID)
	return response.UUID, nil
}

func (s *ChibisafeService) addTagToFile(fileUUID, tagUUID string) error {
	url := fmt.Sprintf("%s/api/file/%s/tag/%s", s.apiURL, fileUUID, tagUUID)
	
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("x-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add tag to file failed: %d - %s", resp.StatusCode, string(body))
	}

	log.Printf("Added tag %s to file %s", tagUUID, fileUUID)
	return nil
}