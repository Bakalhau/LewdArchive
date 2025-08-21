package service

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"lewdarchive/internal/utils"
)

type ArchiveService struct {
	baseDir string
}

func NewArchiveService(baseDir string) *ArchiveService {
	return &ArchiveService{
		baseDir: baseDir,
	}
}

func (s *ArchiveService) DownloadContent(url, author, categoryTitle string, publishedAt time.Time, hash string) {
	log.Printf("Starting download for: %s", url)

	if _, err := exec.LookPath("gallery-dl"); err != nil {
		log.Printf("gallery-dl not found in PATH: %v", err)
		return
	}

	archiveDir := s.buildArchivePath(author, categoryTitle, publishedAt, hash)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		log.Printf("Error creating directory %s: %v", archiveDir, err)
		return
	}

	if err := s.executeGalleryDL(archiveDir, url); err != nil {
		log.Printf("Error in gallery-dl for %s: %v", url, err)
		return
	}

	log.Printf("Download completed for: %s", url)
}

func (s *ArchiveService) buildArchivePath(author, categoryTitle string, publishedAt time.Time, hash string) string {
	sanitizedAuthor := utils.SanitizeForPath(author)
	sanitizedCategory := utils.SanitizeForPath(categoryTitle)
	year := fmt.Sprintf("%04d", publishedAt.Year())
	month := fmt.Sprintf("%02d - %s", int(publishedAt.Month()), publishedAt.Month().String())
	
	return filepath.Join(
		s.baseDir,
		fmt.Sprintf("%s - %s", sanitizedAuthor, sanitizedCategory),
		year,
		month,
		hash,
	)
}

func (s *ArchiveService) executeGalleryDL(destDir, url string) error {
	cmd := exec.Command("gallery-dl",
		"--dest", destDir,
		"--no-mtime",
		"--option", "directory=[]",
		url)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gallery-dl execution failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}