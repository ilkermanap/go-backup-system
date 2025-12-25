package handlers

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ilker/backup-server/internal/middleware"
	"github.com/ilker/backup-server/internal/models"
	"gorm.io/gorm"
)

type BackupHandler struct {
	db          *gorm.DB
	storagePath string
}

func NewBackupHandler(db *gorm.DB, storagePath string) *BackupHandler {
	return &BackupHandler{
		db:          db,
		storagePath: storagePath,
	}
}

type BackupResponse struct {
	ID        uint    `json:"id"`
	FileName  string  `json:"file_name"`
	FileSize  int64   `json:"file_size"`
	SizeMB    float64 `json:"size_mb"`
	Checksum  string  `json:"checksum"`
	CreatedAt string  `json:"created_at"`
}

// GET /api/v1/devices/:id/backups
func (h *BackupHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}

	// Verify device belongs to user
	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	var backups []models.Backup
	if err := h.db.Where("device_id = ?", deviceID).Order("created_at DESC").Find(&backups).Error; err != nil {
		InternalError(c, "Failed to fetch backups")
		return
	}

	response := make([]BackupResponse, len(backups))
	for i, b := range backups {
		response[i] = BackupResponse{
			ID:        b.ID,
			FileName:  b.FileName,
			FileSize:  b.FileSize,
			SizeMB:    b.FileSizeMB(),
			Checksum:  b.Checksum,
			CreatedAt: b.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	Success(c, response)
}

// POST /api/v1/devices/:id/backups
func (h *BackupHandler) Upload(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}

	// Verify device belongs to user
	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	// Check quota
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	currentUsage := h.calculateUsage(userID)
	quotaBytes := int64(user.Plan) * 1024 * 1024 * 1024 // GB to bytes

	file, err := c.FormFile("file")
	if err != nil {
		BadRequest(c, "No file uploaded")
		return
	}

	if currentUsage+file.Size > quotaBytes {
		Error(c, 413, "QUOTA_EXCEEDED", "Storage quota exceeded")
		return
	}

	// Create directory structure with timestamp (matching Python's format: yyyyMMdd-HHmmss)
	userHash := h.hashEmail(user.Email)
	// Get session_id from form or use current timestamp
	sessionID := c.PostForm("session_id")
	if sessionID == "" {
		sessionID = time.Now().Format("20060102-150405")
	}
	backupDir := filepath.Join(h.storagePath, userHash, fmt.Sprintf("%d", deviceID), sessionID)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		InternalError(c, "Failed to create backup directory")
		return
	}

	// Save file
	filePath := filepath.Join(backupDir, file.Filename)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		InternalError(c, "Failed to save file")
		return
	}

	// Calculate checksum
	checksum, err := h.calculateChecksum(filePath)
	if err != nil {
		InternalError(c, "Failed to calculate checksum")
		return
	}

	backup := models.Backup{
		DeviceID: uint(deviceID),
		FileName: file.Filename,
		FilePath: filePath,
		FileSize: file.Size,
		Checksum: checksum,
	}

	if err := h.db.Create(&backup).Error; err != nil {
		InternalError(c, "Failed to save backup record")
		return
	}

	Created(c, BackupResponse{
		ID:        backup.ID,
		FileName:  backup.FileName,
		FileSize:  backup.FileSize,
		SizeMB:    backup.FileSizeMB(),
		Checksum:  backup.Checksum,
		CreatedAt: backup.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// GET /api/v1/devices/:id/backups/:backupId
func (h *BackupHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}
	backupID, err := strconv.ParseUint(c.Param("backupId"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid backup ID")
		return
	}

	// Verify device belongs to user
	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	var backup models.Backup
	if err := h.db.Where("id = ? AND device_id = ?", backupID, deviceID).First(&backup).Error; err != nil {
		NotFound(c, "Backup not found")
		return
	}

	Success(c, BackupResponse{
		ID:        backup.ID,
		FileName:  backup.FileName,
		FileSize:  backup.FileSize,
		SizeMB:    backup.FileSizeMB(),
		Checksum:  backup.Checksum,
		CreatedAt: backup.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// GET /api/v1/devices/:id/backups/:backupId/download
func (h *BackupHandler) Download(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}
	backupID, err := strconv.ParseUint(c.Param("backupId"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid backup ID")
		return
	}

	// Verify device belongs to user
	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	var backup models.Backup
	if err := h.db.Where("id = ? AND device_id = ?", backupID, deviceID).First(&backup).Error; err != nil {
		NotFound(c, "Backup not found")
		return
	}

	if _, err := os.Stat(backup.FilePath); os.IsNotExist(err) {
		NotFound(c, "Backup file not found on disk")
		return
	}

	c.FileAttachment(backup.FilePath, backup.FileName)
}

// DELETE /api/v1/devices/:id/backups/:backupId
func (h *BackupHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}
	backupID, err := strconv.ParseUint(c.Param("backupId"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid backup ID")
		return
	}

	// Verify device belongs to user
	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	var backup models.Backup
	if err := h.db.Where("id = ? AND device_id = ?", backupID, deviceID).First(&backup).Error; err != nil {
		NotFound(c, "Backup not found")
		return
	}

	// Delete file from disk
	os.Remove(backup.FilePath)

	if err := h.db.Delete(&backup).Error; err != nil {
		InternalError(c, "Failed to delete backup")
		return
	}

	NoContent(c)
}

// GET /api/v1/devices/:id/backups/latest
func (h *BackupHandler) Latest(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}

	// Verify device belongs to user
	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	var backup models.Backup
	if err := h.db.Where("device_id = ?", deviceID).Order("created_at DESC").First(&backup).Error; err != nil {
		NotFound(c, "No backups found")
		return
	}

	Success(c, BackupResponse{
		ID:        backup.ID,
		FileName:  backup.FileName,
		FileSize:  backup.FileSize,
		SizeMB:    backup.FileSizeMB(),
		Checksum:  backup.Checksum,
		CreatedAt: backup.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

func (h *BackupHandler) hashEmail(email string) string {
	hash := sha256.Sum256([]byte(email))
	return hex.EncodeToString(hash[:])
}

func (h *BackupHandler) calculateChecksum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (h *BackupHandler) calculateUsage(userID uint) int64 {
	var totalSize int64

	var devices []models.Device
	h.db.Where("user_id = ?", userID).Find(&devices)

	for _, device := range devices {
		var backups []models.Backup
		h.db.Where("device_id = ?", device.ID).Find(&backups)
		for _, backup := range backups {
			totalSize += backup.FileSize
		}
	}

	return totalSize
}

// =============================================
// Catalog Handlers (Encrypted SQLite dumps)
// =============================================

type CatalogResponse struct {
	ID        uint   `json:"id"`
	SessionID string `json:"session_id"`
	FileName  string `json:"file_name"`
	FileSize  int64  `json:"file_size"`
	CreatedAt string `json:"created_at"`
}

// POST /api/v1/devices/:id/catalogs - Upload encrypted catalog
func (h *BackupHandler) UploadCatalog(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}

	// Verify device belongs to user
	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	// Get user for directory structure
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	file, err := c.FormFile("catalog")
	if err != nil {
		BadRequest(c, "No catalog file uploaded")
		return
	}

	sessionID := c.PostForm("session_id")
	if sessionID == "" {
		sessionID = time.Now().Format("20060102-150405")
	}

	// Create catalog directory
	userHash := h.hashEmail(user.Email)
	catalogDir := filepath.Join(h.storagePath, userHash, fmt.Sprintf("%d", deviceID), "catalogs")
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		InternalError(c, "Failed to create catalog directory")
		return
	}

	// Save catalog file
	fileName := fmt.Sprintf("%s.katalog.enc", sessionID)
	filePath := filepath.Join(catalogDir, fileName)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		InternalError(c, "Failed to save catalog file")
		return
	}

	// Save catalog record to database
	catalog := models.Catalog{
		DeviceID:  uint(deviceID),
		SessionID: sessionID,
		FileName:  fileName,
		FilePath:  filePath,
		FileSize:  file.Size,
	}

	if err := h.db.Create(&catalog).Error; err != nil {
		InternalError(c, "Failed to save catalog record")
		return
	}

	Created(c, CatalogResponse{
		ID:        catalog.ID,
		SessionID: catalog.SessionID,
		FileName:  catalog.FileName,
		FileSize:  catalog.FileSize,
		CreatedAt: catalog.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// GET /api/v1/devices/:id/catalogs - List catalogs
func (h *BackupHandler) ListCatalogs(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}

	// Verify device belongs to user
	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	var catalogs []models.Catalog
	if err := h.db.Where("device_id = ?", deviceID).Order("created_at DESC").Find(&catalogs).Error; err != nil {
		InternalError(c, "Failed to fetch catalogs")
		return
	}

	// Return download URLs
	var urls []string
	for _, cat := range catalogs {
		url := fmt.Sprintf("/api/v1/devices/%d/catalogs/%d/download", deviceID, cat.ID)
		urls = append(urls, url)
	}

	Success(c, urls)
}

// GET /api/v1/devices/:id/catalogs/:catalogId/download - Download catalog
func (h *BackupHandler) DownloadCatalog(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}
	catalogID, err := strconv.ParseUint(c.Param("catalogId"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid catalog ID")
		return
	}

	// Verify device belongs to user
	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	var catalog models.Catalog
	if err := h.db.Where("id = ? AND device_id = ?", catalogID, deviceID).First(&catalog).Error; err != nil {
		NotFound(c, "Catalog not found")
		return
	}

	if _, err := os.Stat(catalog.FilePath); os.IsNotExist(err) {
		NotFound(c, "Catalog file not found on disk")
		return
	}

	c.FileAttachment(catalog.FilePath, catalog.FileName)
}

// =============================================
// File Restore API (Time Machine style)
// =============================================

// FileRestoreRequest represents a single file to restore
type FileRestoreRequest struct {
	HashedName string `json:"hashed_name"` // SHA256 hash of the file path
	TargetDate string `json:"target_date"` // Format: 2006-01-02 or 2006-01-02T15:04:05
}

// RestoreFilesRequest is the request body for RestoreFiles
type RestoreFilesRequest struct {
	Files []FileRestoreRequest `json:"files"`
}

// POST /api/v1/devices/:id/restore-files
// Client sends list of hashed file names, server returns those files from tar archives
func (h *BackupHandler) RestoreFiles(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}

	// Verify device belongs to user
	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	// Get user for directory structure
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	// Parse request body
	var req RestoreFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "Invalid request body")
		return
	}

	if len(req.Files) == 0 {
		BadRequest(c, "No files specified")
		return
	}

	// Build a map of requested files
	requestedFiles := make(map[string]time.Time)
	for _, f := range req.Files {
		targetTime := time.Now()
		if f.TargetDate != "" {
			// Try various date formats (use local timezone to match database storage)
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", f.TargetDate, time.Local); err == nil {
				targetTime = t
			} else if t, err := time.ParseInLocation("2006-01-02T15:04:05", f.TargetDate, time.Local); err == nil {
				targetTime = t
			} else if t, err := time.ParseInLocation("2006-01-02", f.TargetDate, time.Local); err == nil {
				targetTime = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second) // End of day
			} else if t, err := time.Parse(time.RFC3339, f.TargetDate); err == nil {
				targetTime = t
			} else if t, err := time.ParseInLocation("20060102-150405", f.TargetDate, time.Local); err == nil {
				// Python timestamp format
				targetTime = t
			}
		}
		fmt.Printf("[RestoreFiles] File %s requested at targetTime=%v (from TargetDate=%s)\n", f.HashedName, targetTime, f.TargetDate)
		requestedFiles[f.HashedName] = targetTime
	}

	// Find device backup directory
	userHash := h.hashEmail(user.Email)
	deviceDir := filepath.Join(h.storagePath, userHash, fmt.Sprintf("%d", deviceID))

	if _, err := os.Stat(deviceDir); os.IsNotExist(err) {
		NotFound(c, "No backups found")
		return
	}

	// Find all tar files, sorted by date (newest first)
	tarFiles, err := h.findTarFiles(deviceDir)
	if err != nil || len(tarFiles) == 0 {
		NotFound(c, "No backup archives found")
		return
	}

	// For each requested file, find the best matching version
	foundFiles := make(map[string]tarFileInfo) // hashedName -> tarFileInfo

	for hashedName, targetTime := range requestedFiles {
		bestMatch := h.findBestFileVersion(tarFiles, hashedName, targetTime)
		if bestMatch != nil {
			foundFiles[hashedName] = *bestMatch
		}
	}

	if len(foundFiles) == 0 {
		NotFound(c, "Requested files not found in backups")
		return
	}

	// Stream response as tar.gz
	c.Header("Content-Type", "application/x-gzip")
	c.Header("Content-Disposition", "attachment; filename=restore.tar.gz")

	gzWriter := gzip.NewWriter(c.Writer)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Extract and add each file to the response tar
	for hashedName, info := range foundFiles {
		if err := h.extractFileToTar(info.tarPath, hashedName, tarWriter); err != nil {
			fmt.Printf("[RestoreFiles] Error extracting %s: %v\n", hashedName, err)
			continue
		}
	}
}

type tarFileInfo struct {
	tarPath   string
	tarDate   time.Time
	fileName  string
}

// parseTimestampFromPath extracts timestamp from directory name
// Expected formats: "20060102-150405" (Python format) or "2006-01-02" (old format)
func (h *BackupHandler) parseTimestampFromPath(tarPath string) (time.Time, error) {
	// Get the parent directory name (which should contain the timestamp)
	dir := filepath.Dir(tarPath)
	dirName := filepath.Base(dir)

	// Try Python format first: 20060102-150405
	if t, err := time.ParseInLocation("20060102-150405", dirName, time.Local); err == nil {
		return t, nil
	}

	// Try old date format: 2006-01-02
	if t, err := time.ParseInLocation("2006-01-02", dirName, time.Local); err == nil {
		// Set time to end of day for comparison purposes
		return t.Add(23*time.Hour + 59*time.Minute + 59*time.Second), nil
	}

	// Fallback: use file modification time
	info, err := os.Stat(tarPath)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// findTarFiles returns all tar files in the device directory, sorted by date (newest first)
func (h *BackupHandler) findTarFiles(deviceDir string) ([]string, error) {
	var tarFiles []string

	err := filepath.Walk(deviceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".tar") {
			tarFiles = append(tarFiles, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by timestamp parsed from directory name (newest first)
	sort.Slice(tarFiles, func(i, j int) bool {
		iTime, errI := h.parseTimestampFromPath(tarFiles[i])
		jTime, errJ := h.parseTimestampFromPath(tarFiles[j])
		if errI != nil || errJ != nil {
			// Fallback to modification time if parsing fails
			iInfo, _ := os.Stat(tarFiles[i])
			jInfo, _ := os.Stat(tarFiles[j])
			if iInfo == nil || jInfo == nil {
				return false
			}
			return iInfo.ModTime().After(jInfo.ModTime())
		}
		return iTime.After(jTime)
	})

	return tarFiles, nil
}

// findBestFileVersion searches through tar files for the best version of a file
// It uses the directory name (timestamp format) to determine file version, not file modification time
func (h *BackupHandler) findBestFileVersion(tarFiles []string, hashedName string, targetTime time.Time) *tarFileInfo {
	fmt.Printf("[findBestFileVersion] Looking for %s at targetTime=%v\n", hashedName, targetTime)
	var bestMatch *tarFileInfo
	var bestDiff time.Duration = -1

	for _, tarPath := range tarFiles {
		// Parse timestamp from directory name (not file modification time!)
		tarDate, err := h.parseTimestampFromPath(tarPath)
		if err != nil {
			fmt.Printf("[findBestFileVersion] Failed to parse timestamp for %s: %v\n", tarPath, err)
			continue
		}

		dirName := filepath.Base(filepath.Dir(tarPath))
		fmt.Printf("[findBestFileVersion] Checking tar: %s (dir=%s), tarDate=%v\n", filepath.Base(tarPath), dirName, tarDate)

		// Only consider files at or before target time
		if tarDate.After(targetTime) {
			fmt.Printf("[findBestFileVersion] Skipping - tarDate %v > targetTime %v\n", tarDate, targetTime)
			continue
		}

		// Check if this tar contains the file
		if h.tarContainsFile(tarPath, hashedName) {
			diff := targetTime.Sub(tarDate)
			fmt.Printf("[findBestFileVersion] Found file in tar, diff=%v\n", diff)
			if bestDiff < 0 || diff < bestDiff {
				bestDiff = diff
				bestMatch = &tarFileInfo{
					tarPath:  tarPath,
					tarDate:  tarDate,
					fileName: hashedName,
				}
			}
		}
	}

	if bestMatch != nil {
		fmt.Printf("[findBestFileVersion] Best match: %s (dir=%s) at %v\n", filepath.Base(bestMatch.tarPath), filepath.Base(filepath.Dir(bestMatch.tarPath)), bestMatch.tarDate)
	} else {
		fmt.Printf("[findBestFileVersion] No match found!\n")
	}
	return bestMatch
}

// tarContainsFile checks if a tar archive contains a file with the given name
func (h *BackupHandler) tarContainsFile(tarPath string, hashedName string) bool {
	file, err := os.Open(tarPath)
	if err != nil {
		return false
	}
	defer file.Close()

	tr := tar.NewReader(file)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}

		// File names in tar might be hashed_name.enc
		baseName := filepath.Base(hdr.Name)
		if baseName == hashedName || baseName == hashedName+".enc" ||
			strings.TrimSuffix(baseName, ".enc") == hashedName {
			return true
		}
	}
	return false
}

// extractFileToTar extracts a file from source tar and writes it to destination tar
func (h *BackupHandler) extractFileToTar(srcTarPath string, hashedName string, destTar *tar.Writer) error {
	file, err := os.Open(srcTarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	tr := tar.NewReader(file)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		baseName := filepath.Base(hdr.Name)
		if baseName == hashedName || baseName == hashedName+".enc" ||
			strings.TrimSuffix(baseName, ".enc") == hashedName {
			// Write header (keep original name)
			if err := destTar.WriteHeader(hdr); err != nil {
				return err
			}
			// Copy file content
			if _, err := io.Copy(destTar, tr); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("file not found in archive")
}
