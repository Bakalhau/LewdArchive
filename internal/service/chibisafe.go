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
	"sync"

	"lewdarchive/internal/model"
	"lewdarchive/internal/utils"
)

type ChibisafeService struct {
	apiURL           string
	apiKey           string
	client           *http.Client
	useNetworkStorage *bool 
	settingsMutex     sync.RWMutex
}

type ChibisafeSettings struct {
	UseNetworkStorage bool `json:"useNetworkStorage"`
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

func (s *ChibisafeService) getSettings() (*ChibisafeSettings, error) {
	s.settingsMutex.RLock()
	if s.useNetworkStorage != nil {
		useS3 := *s.useNetworkStorage
		s.settingsMutex.RUnlock()
		return &ChibisafeSettings{UseNetworkStorage: useS3}, nil
	}
	s.settingsMutex.RUnlock()

	req, err := http.NewRequest("GET", s.apiURL+"/api/settings", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create settings request: %w", err)
	}

	req.Header.Set("x-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get settings failed: %d - %s", resp.StatusCode, string(body))
	}

	var settings ChibisafeSettings
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return nil, fmt.Errorf("failed to decode settings: %w", err)
	}

	s.settingsMutex.Lock()
	s.useNetworkStorage = &settings.UseNetworkStorage
	s.settingsMutex.Unlock()

	log.Printf("Chibisafe settings: useNetworkStorage=%v", settings.UseNetworkStorage)
	return &settings, nil
}

func (s *ChibisafeService) UploadFiles(archiveDir, categoryTitle, author, title string) error {
	if !s.IsConfigured() {
		log.Printf("Chibisafe not configured, skipping upload for %s", archiveDir)
		return nil
	}

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

		if tagUUID != "" && fileUUID != "" {
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

func (s *ChibisafeService) getContentType(filePath, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	specificTypes := map[string]string{
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".mkv":  "video/x-matroska",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".bmp":  "image/bmp",
		".tiff": "image/tiff",
		".svg":  "image/svg+xml",
	}

	if contentType, exists := specificTypes[ext]; exists {
		return contentType
	}

	if detectedType := mime.TypeByExtension(ext); detectedType != "" {
		return detectedType
	}

	return "application/octet-stream"
}

func (s *ChibisafeService) uploadFile(filePath, filename, albumUUID string) (string, error) {
	settings, err := s.getSettings()
	if err != nil {
		log.Printf("Warning: Could not get Chibisafe settings, falling back to direct upload: %v", err)
		return s.uploadFileDirect(filePath, filename, albumUUID)
	}

	if settings.UseNetworkStorage {
		log.Printf("Using S3 upload method for %s", filename)
		return s.uploadFileS3(filePath, filename, albumUUID)
	} else {
		log.Printf("Using direct upload method for %s", filename)
		return s.uploadFileDirect(filePath, filename, albumUUID)
	}
}

func (s *ChibisafeService) getSignedURL(filename string, fileSize int64, contentType string) (string, string, error) {
	reqBody := model.ChibisafeUploadRequest{
		Name:        filename,
		Size:        fileSize,
		ContentType: contentType,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", s.apiURL+"/api/upload", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("failed to get signed URL: %d - %s", resp.StatusCode, string(body))
	}

	var response model.ChibisafeUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("Got signed URL for %s: identifier=%s", filename, response.Identifier)
	return response.URL, response.Identifier, nil
}

func (s *ChibisafeService) uploadToS3(signedURL string, filePath string, contentType string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	req, err := http.NewRequest("PUT", signedURL, file)
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.ContentLength = fileInfo.Size()

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 upload failed: %d - %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully uploaded file to S3")
	return nil
}

func (s *ChibisafeService) processUpload(identifier, filename, contentType, albumUUID string) (string, error) {
	reqBody := map[string]string{
		"identifier": identifier,
		"name":       filename,
		"type":       contentType,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	processURL := fmt.Sprintf("%s/api/upload/process", s.apiURL)
	req, err := http.NewRequest("POST", processURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create process request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)

	if albumUUID != "" {
		req.Header.Set("albumuuid", albumUUID)
	}

	log.Printf("Processing upload with body: %s", string(jsonBody))
	if albumUUID != "" {
		log.Printf("Using album UUID header: %s", albumUUID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to process upload: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	log.Printf("Process upload response - Status: %d, Body: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("process upload failed: %d - %s", resp.StatusCode, string(body))
	}

	var processResponse map[string]interface{}
	if err := json.Unmarshal(body, &processResponse); err != nil {
		return "", fmt.Errorf("failed to decode process response: %w", err)
	}

	var fileUUID string

	if file, ok := processResponse["file"].(map[string]interface{}); ok {
		if uuid, ok := file["uuid"].(string); ok {
			fileUUID = uuid
		}
	}

	if fileUUID == "" {
		if uuid, ok := processResponse["uuid"].(string); ok {
			fileUUID = uuid
		}
	}

	if fileUUID == "" {
		if files, ok := processResponse["files"].([]interface{}); ok && len(files) > 0 {
			if file, ok := files[0].(map[string]interface{}); ok {
				if uuid, ok := file["uuid"].(string); ok {
					fileUUID = uuid
				}
			}
		}
	}

	if fileUUID == "" {
		log.Printf("WARNING: Could not extract file UUID from response: %s", string(body))
		return "", fmt.Errorf("file UUID not found in response")
	}

	log.Printf("Successfully processed upload: %s", fileUUID)
	return fileUUID, nil
}

func (s *ChibisafeService) uploadFileS3(filePath, filename, albumUUID string) (string, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	contentType := s.getContentType(filePath, filename)

	log.Printf("Starting S3 upload for %s (size: %d bytes, content-type: %s)",
		filename, fileInfo.Size(), contentType)

	signedURL, identifier, err := s.getSignedURL(filename, fileInfo.Size(), contentType)
	if err != nil {
		return "", fmt.Errorf("failed to get signed URL: %w", err)
	}

	if err := s.uploadToS3(signedURL, filePath, contentType); err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	fileUUID, err := s.processUpload(identifier, filename, contentType, albumUUID)
	if err != nil {
		return "", fmt.Errorf("failed to process upload: %w", err)
	}

	log.Printf("Successfully uploaded file via S3: %s -> UUID: %s",
		filename, fileUUID)

	return fileUUID, nil
}

func (s *ChibisafeService) uploadFileDirect(filePath, filename, albumUUID string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	contentType := s.getContentType(filePath, filename)

	log.Printf("Starting direct upload for %s (size: %d bytes, content-type: %s)", filename, fileInfo.Size(), contentType)

	headers := textproto.MIMEHeader{}
	headers.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename="%s"`, filename))
	headers.Set("Content-Type", contentType)

	if strings.ToLower(filepath.Ext(filename)) == ".mp4" {
		headers.Set("Content-Transfer-Encoding", "binary")
	}

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

	log.Printf("Direct upload request headers: Content-Type=%s, albumuuid=%s",
		writer.FormDataContentType(), albumUUID)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Direct upload failed for %s: status=%d, body=%s", filename, resp.StatusCode, string(body))
		return "", fmt.Errorf("upload failed: %d - %s", resp.StatusCode, string(body))
	}

	var response model.ChibisafeUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	log.Printf("Successfully uploaded file via direct upload: %s (%s) -> UUID: %s, Public URL: %s",
		response.Name, filename, response.UUID, response.PublicURL)
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

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("Add tag failed - Status: %d, Body: %s", resp.StatusCode, string(body))
		return fmt.Errorf("add tag to file failed: %d - %s", resp.StatusCode, string(body))
	}

	log.Printf("Added tag %s to file %s", tagUUID, fileUUID)
	return nil
}