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
	baseDir            string
	chibisafeService   *ChibisafeService
	cleanupAfterUpload bool
}

func NewArchiveService(baseDir string, chibisafeService *ChibisafeService, cleanupAfterUpload bool) *ArchiveService {
	return &ArchiveService{
		baseDir:            baseDir,
		chibisafeService:   chibisafeService,
		cleanupAfterUpload: cleanupAfterUpload,
	}
}

func (s *ArchiveService) DownloadContent(url, author, categoryTitle, title string, publishedAt time.Time, hash string) {
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

	if s.chibisafeService != nil && s.chibisafeService.IsConfigured() {
		log.Printf("Starting Chibisafe upload for: %s", archiveDir)
		if err := s.chibisafeService.UploadFiles(archiveDir, categoryTitle, author, title); err != nil {
			log.Printf("Error uploading to Chibisafe: %v", err)
		} else {
			log.Printf("Chibisafe upload completed for: %s", archiveDir)
			
			if s.cleanupAfterUpload {
				if err := s.cleanupDirectory(archiveDir); err != nil {
					log.Printf("Error cleaning up directory %s: %v", archiveDir, err)
				} else {
					log.Printf("Successfully cleaned up directory: %s", archiveDir)
				}
			}
		}
	}
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

func (s *ArchiveService) cleanupDirectory(dirPath string) error {
	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		log.Printf("Directory %s does not exist, nothing to clean up", dirPath)
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var filesRemoved int
	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(dirPath, entry.Name())
			if err := os.Remove(filePath); err != nil {
				log.Printf("Warning: failed to remove file %s: %v", filePath, err)
			} else {
				filesRemoved++
			}
		}
	}

	if err := os.Remove(dirPath); err != nil {
		log.Printf("Note: Could not remove directory %s (may contain subdirectories): %v", dirPath, err)
	}

	s.cleanupEmptyParentDirs(filepath.Dir(dirPath))

	log.Printf("Cleanup completed: removed %d files from %s", filesRemoved, dirPath)
	return nil
}

func (s *ArchiveService) cleanupEmptyParentDirs(dirPath string) {
	if dirPath == s.baseDir || dirPath == filepath.Dir(s.baseDir) {
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	if len(entries) == 0 {
		if err := os.Remove(dirPath); err == nil {
			log.Printf("Removed empty directory: %s", dirPath)
			s.cleanupEmptyParentDirs(filepath.Dir(dirPath))
		}
	}
}