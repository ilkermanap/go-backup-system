package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ilker/backup-client/internal/catalog"
	"github.com/ilker/backup-client/internal/config"
	"github.com/ilker/backup-client/internal/crypto"
)

const (
	maxTarSize = 25 * 1024 * 1024 // 25MB per tar file
)

type Service struct {
	config     *config.Config
	catalog    *catalog.Catalog
	client     *http.Client
	isRunning  bool
	shouldStop bool
	mu         sync.Mutex
	OnProgress func(Progress)
}

type Progress struct {
	Phase       string  `json:"phase"`
	Message     string  `json:"message"`
	CurrentFile string  `json:"current_file"`
	CurrentDir  string  `json:"current_dir"`
	TotalFiles  int     `json:"total_files"`
	DoneFiles   int     `json:"done_files"`
	TotalBytes  int64   `json:"total_bytes"`
	DoneBytes   int64   `json:"done_bytes"`
	Percent     float64 `json:"percent"`
}

type LoginResult struct {
	User  map[string]interface{} `json:"user"`
	Token string                 `json:"token"`
}

type Device struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type BackupEntry struct {
	ID        uint   `json:"id"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

type QuotaInfo struct {
	TotalGB   int     `json:"total_gb"`
	UsedBytes int64   `json:"used_bytes"`
	UsedGB    float64 `json:"used_gb"`
	Percent   float64 `json:"percent"`
}

type UsageInfo struct {
	DeviceCount int    `json:"device_count"`
	BackupCount int    `json:"backup_count"`
	TotalSize   int64  `json:"total_size"`
	LastBackup  string `json:"last_backup"`
}

type BackupStatus struct {
	IsRunning  bool   `json:"is_running"`
	LastBackup string `json:"last_backup"`
	NextBackup string `json:"next_backup"`
	FilesCount int    `json:"files_count"`
	TotalSize  int64  `json:"total_size"`
	DeviceID   uint   `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// fileToBackup holds info about a file to be backed up
type fileToBackup struct {
	path        string
	size        int64
	modTime     time.Time
	hashedName  string
	contentHash string
}

func NewService(cfg *config.Config, cat *catalog.Catalog) *Service {
	return &Service{
		config:  cfg,
		catalog: cat,
		client: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

func (s *Service) Login(email, password string) (*LoginResult, error) {
	body := map[string]string{
		"email":    email,
		"password": password,
	}

	resp, err := s.post("/api/v1/auth/login", body, "")
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool        `json:"success"`
		Data    LoginResult `json:"data"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf(result.Error.Message)
	}

	return &result.Data, nil
}

func (s *Service) GetDevices() ([]Device, error) {
	resp, err := s.get("/api/v1/devices")
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool     `json:"success"`
		Data    []Device `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (s *Service) RegisterDevice(name string) (*Device, error) {
	body := map[string]string{
		"name": name,
	}

	resp, err := s.post("/api/v1/devices", body, s.config.Token)
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool   `json:"success"`
		Data    Device `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	s.config.DeviceID = result.Data.ID
	s.config.DeviceName = result.Data.Name

	return &result.Data, nil
}

// Run performs incremental backup with zero-knowledge architecture (Time Machine style)
func (s *Service) Run() error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("backup already running")
	}
	s.isRunning = true
	s.shouldStop = false
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

	if s.config.DeviceID == 0 {
		return fmt.Errorf("no device registered")
	}

	if s.config.EncryptionKey == "" {
		return fmt.Errorf("encryption key not set")
	}

	// Derive AES key from passphrase
	key := crypto.DeriveKey(s.config.EncryptionKey)

	// Generate backup ID for this session
	backupID := time.Now().Format("20060102-150405")

	// Create temp directory for this backup
	tempDir := filepath.Join(s.config.DataDir, "temp_"+backupID)
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// Scan directories - only files that changed since last backup
	s.emitProgress(Progress{Phase: "scanning", Message: "Dizinler taranıyor..."})

	var filesToBackup []fileToBackup
	seenPaths := make(map[string]bool) // Prevent duplicate files from overlapping directories
	var totalScanned, totalSkipped int

	for i, dir := range s.config.BackupDirs {
		fmt.Printf("[RUN] Scanning directory %d: %s (seenPaths has %d entries)\n", i+1, dir, len(seenPaths))
		s.emitProgress(Progress{
			Phase:      "scanning",
			Message:    fmt.Sprintf("Dizin taranıyor: %s (%d/%d)", filepath.Base(dir), i+1, len(s.config.BackupDirs)),
			CurrentDir: dir,
		})

		files, scanned, skipped, err := s.scanDirectoryIncrementalWithProgress(dir, seenPaths)
		if err != nil {
			continue
		}
		filesToBackup = append(filesToBackup, files...)
		totalScanned += scanned
		totalSkipped += skipped
	}

	s.emitProgress(Progress{
		Phase:      "scanning",
		Message:    fmt.Sprintf("Tarama tamamlandı. %d dosya tarandı, %d değişmemiş, %d yedeklenecek.", totalScanned, totalSkipped, len(filesToBackup)),
		TotalFiles: len(filesToBackup),
	})

	totalFiles := len(filesToBackup)
	if totalFiles == 0 {
		s.emitProgress(Progress{Phase: "complete", Message: fmt.Sprintf("Yedeklenecek değişiklik yok. %d dosya tarandı, hepsi güncel.", totalScanned), Percent: 100})
		return nil
	}

	// Calculate total bytes
	var totalBytes int64
	for _, f := range filesToBackup {
		totalBytes += f.size
	}

	// Process files: encrypt and add to tar
	s.emitProgress(Progress{
		Phase:      "encrypting",
		Message:    "Dosyalar şifreleniyor...",
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
	})

	var tarSize int64
	tarPart := 1
	tarPath := filepath.Join(tempDir, fmt.Sprintf("%s-%06d.tar", backupID, tarPart))
	tarFile, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	tarWriter := tar.NewWriter(tarFile)

	timestamp := time.Now()
	var catalogEntries []catalog.FileEntry
	var doneBytes int64

	for i, file := range filesToBackup {
		if s.shouldStop {
			tarWriter.Close()
			tarFile.Close()
			return fmt.Errorf("backup cancelled")
		}

		s.emitProgress(Progress{
			Phase:       "encrypting",
			Message:     fmt.Sprintf("Şifreleniyor: %s", filepath.Base(file.path)),
			CurrentFile: filepath.Base(file.path),
			TotalFiles:  totalFiles,
			DoneFiles:   i,
			TotalBytes:  totalBytes,
			DoneBytes:   doneBytes,
			Percent:     float64(i) / float64(totalFiles) * 100,
		})

		// Encrypt file to temp location with hashed name
		encFileName := file.hashedName + ".enc"
		encPath := filepath.Join(tempDir, encFileName)

		packedSize, err := crypto.EncryptFile(file.path, encPath, key)
		if err != nil {
			continue // Skip failed files
		}

		// Add to tar
		encInfo, err := os.Stat(encPath)
		if err != nil {
			os.Remove(encPath)
			continue
		}

		header := &tar.Header{
			Name:    encFileName,
			Size:    encInfo.Size(),
			Mode:    0600,
			ModTime: timestamp,
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			os.Remove(encPath)
			continue
		}

		encFile, err := os.Open(encPath)
		if err != nil {
			os.Remove(encPath)
			continue
		}

		if _, err := io.Copy(tarWriter, encFile); err != nil {
			encFile.Close()
			os.Remove(encPath)
			continue
		}
		encFile.Close()
		os.Remove(encPath)

		tarSize += encInfo.Size()
		doneBytes += file.size

		// Add new version to catalog (Time Machine style - same file can have multiple entries)
		entry := catalog.FileEntry{
			Timestamp:   timestamp,
			Directory:   filepath.Dir(file.path),
			OrigPath:    file.path,
			HashedName:  file.hashedName,
			ContentHash: file.contentHash,
			Size:        file.size,
			PackedSize:  packedSize,
		}
		catalogEntries = append(catalogEntries, entry)

		// If tar is too big, close it, upload, and start a new one
		if tarSize > maxTarSize {
			tarWriter.Close()
			tarFile.Close()

			s.emitProgress(Progress{
				Phase:      "uploading",
				Message:    fmt.Sprintf("Parça yükleniyor (%d. parça)...", tarPart),
				DoneFiles:  i + 1,
				TotalFiles: totalFiles,
				TotalBytes: totalBytes,
				DoneBytes:  doneBytes,
				Percent:    float64(i+1) / float64(totalFiles) * 100,
			})

			fmt.Printf("[UPLOAD] Uploading tar part %d: %s\n", tarPart, tarPath)
			if err := s.uploadTar(tarPath, backupID); err != nil {
				fmt.Printf("[UPLOAD] ERROR: %v\n", err)
				return fmt.Errorf("failed to upload tar: %w", err)
			}
			fmt.Printf("[UPLOAD] Part %d uploaded successfully\n", tarPart)
			os.Remove(tarPath)

			// Start new tar
			tarPart++
			tarSize = 0
			tarPath = filepath.Join(tempDir, fmt.Sprintf("%s-%06d.tar", backupID, tarPart))
			tarFile, err = os.Create(tarPath)
			if err != nil {
				return err
			}
			tarWriter = tar.NewWriter(tarFile)
		}
	}

	// Close and upload final tar if it has content
	tarWriter.Close()
	tarFile.Close()

	if tarSize > 0 {
		s.emitProgress(Progress{
			Phase:      "uploading",
			Message:    "Son parça yükleniyor...",
			Percent:    95,
			TotalFiles: totalFiles,
			DoneFiles:  totalFiles,
			TotalBytes: totalBytes,
			DoneBytes:  doneBytes,
		})
		fmt.Printf("[UPLOAD] Uploading final tar: %s (size: %d bytes)\n", tarPath, tarSize)
		if err := s.uploadTar(tarPath, backupID); err != nil {
			fmt.Printf("[UPLOAD] ERROR: %v\n", err)
			return fmt.Errorf("failed to upload tar: %w", err)
		}
		fmt.Println("[UPLOAD] Final tar uploaded successfully")
	}
	os.Remove(tarPath)

	// Add entries to main catalog (versions accumulate over time)
	fmt.Printf("[CATALOG] Adding %d entries to catalog\n", len(catalogEntries))
	if len(catalogEntries) > 0 {
		s.emitProgress(Progress{Phase: "updating_catalog", Message: "Katalog güncelleniyor...", Percent: 96})
		if err := s.catalog.AddEntries(catalogEntries); err != nil {
			fmt.Printf("[CATALOG] ERROR: %v\n", err)
			return fmt.Errorf("failed to update catalog: %w", err)
		}
		fmt.Println("[CATALOG] Entries added successfully")
	}

	// Export and upload encrypted catalog dump (for recovery from other machines)
	s.emitProgress(Progress{Phase: "uploading_catalog", Message: "Katalog yedekleniyor...", Percent: 98})

	catalogDumpPath := filepath.Join(tempDir, "catalog.db")
	if err := s.catalog.ExportToFile(catalogDumpPath); err != nil {
		return fmt.Errorf("failed to export catalog: %w", err)
	}

	encCatalogPath := catalogDumpPath + ".enc"
	if _, err := crypto.EncryptFile(catalogDumpPath, encCatalogPath, key); err != nil {
		return fmt.Errorf("failed to encrypt catalog: %w", err)
	}

	if err := s.uploadCatalog(encCatalogPath, backupID); err != nil {
		os.Remove(encCatalogPath)
		return fmt.Errorf("failed to upload catalog: %w", err)
	}

	s.emitProgress(Progress{
		Phase:      "complete",
		Message:    fmt.Sprintf("Yedekleme tamamlandı! %d dosya yedeklendi.", totalFiles),
		TotalFiles: totalFiles,
		DoneFiles:  totalFiles,
		TotalBytes: totalBytes,
		DoneBytes:  totalBytes,
		Percent:    100,
	})

	return nil
}

func (s *Service) Stop() {
	s.mu.Lock()
	s.shouldStop = true
	s.mu.Unlock()
}

func (s *Service) GetStatus() *BackupStatus {
	count, totalSize, _, _ := s.catalog.GetStats()
	return &BackupStatus{
		IsRunning:  s.isRunning,
		FilesCount: int(count),
		TotalSize:  totalSize,
		DeviceID:   s.config.DeviceID,
		DeviceName: s.config.DeviceName,
	}
}

func (s *Service) GetHistory(deviceID uint) ([]BackupEntry, error) {
	resp, err := s.get(fmt.Sprintf("/api/v1/devices/%d/backups", deviceID))
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool          `json:"success"`
		Data    []BackupEntry `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// DeleteBackup deletes a backup from the server
func (s *Service) DeleteBackup(backupID uint) error {
	url := fmt.Sprintf("%s/api/v1/devices/%d/backups/%d", s.config.ServerURL, s.config.DeviceID, backupID)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+s.config.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: %s", string(body))
	}

	return nil
}

// DeleteAllBackups deletes all backups for the current device
func (s *Service) DeleteAllBackups() error {
	backups, err := s.GetHistory(s.config.DeviceID)
	if err != nil {
		return err
	}

	for _, b := range backups {
		if err := s.DeleteBackup(b.ID); err != nil {
			fmt.Printf("[DeleteAllBackups] Error deleting backup %d: %v\n", b.ID, err)
		}
	}

	return nil
}

// RecoverCatalog downloads encrypted catalogs from server and rebuilds local catalog
func (s *Service) RecoverCatalog() error {
	if s.config.EncryptionKey == "" {
		return fmt.Errorf("encryption key required to recover catalog")
	}

	key := crypto.DeriveKey(s.config.EncryptionKey)

	// Get list of catalogs from server
	resp, err := s.get(fmt.Sprintf("/api/v1/devices/%d/catalogs", s.config.DeviceID))
	if err != nil {
		return err
	}

	var result struct {
		Success bool     `json:"success"`
		Data    []string `json:"data"` // List of catalog URLs
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	tempDir := filepath.Join(s.config.DataDir, "catalog_recovery")
	os.MkdirAll(tempDir, 0700)
	defer os.RemoveAll(tempDir)

	for _, catalogURL := range result.Data {
		// Download encrypted catalog
		encPath := filepath.Join(tempDir, "catalog.enc")
		if err := s.downloadFile(catalogURL, encPath); err != nil {
			continue
		}

		// Decrypt
		decPath := filepath.Join(tempDir, "catalog.db")
		if err := crypto.DecryptFile(encPath, decPath, key); err != nil {
			os.Remove(encPath)
			continue
		}
		os.Remove(encPath)

		// Import into main catalog
		if err := s.catalog.ImportFromFile(decPath); err != nil {
			os.Remove(decPath)
			continue
		}
		os.Remove(decPath)
	}

	return nil
}

// RestoreToTime restores files to their state at a specific point in time (Time Machine style)
func (s *Service) RestoreToTime(targetTime time.Time, targetDir string) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("operation already running")
	}
	s.isRunning = true
	s.shouldStop = false
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

	key := crypto.DeriveKey(s.config.EncryptionKey)

	// Get files at the target time
	files, err := s.catalog.GetFilesAtTime(targetTime)
	if err != nil {
		return fmt.Errorf("failed to get files at time: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found at specified time")
	}

	s.emitProgress(Progress{Phase: "preparing", TotalFiles: len(files)})

	// Group files by their tar archives (based on backup date)
	// For simplicity, we'll download all relevant tars

	restoredCount := 0
	for i, file := range files {
		if s.shouldStop {
			return fmt.Errorf("restore cancelled")
		}

		s.emitProgress(Progress{
			Phase:       "restoring",
			CurrentFile: filepath.Base(file.OrigPath),
			TotalFiles:  len(files),
			DoneFiles:   i,
			Percent:     float64(i) / float64(len(files)) * 100,
		})

		// Download the encrypted file from server
		// The file is stored with its hashed name
		encFileName := file.HashedName + ".enc"
		url := fmt.Sprintf("%s/api/v1/devices/%d/files/%s",
			s.config.ServerURL, s.config.DeviceID, encFileName)

		tempEncPath := filepath.Join(s.config.DataDir, encFileName)
		if err := s.downloadFile(url, tempEncPath); err != nil {
			continue // Skip files we can't download
		}

		// Determine destination path
		var destPath string
		if targetDir != "" {
			destPath = filepath.Join(targetDir, file.OrigPath)
		} else {
			destPath = file.OrigPath
		}

		// Create directory structure
		os.MkdirAll(filepath.Dir(destPath), 0755)

		// Decrypt file
		if err := crypto.DecryptFile(tempEncPath, destPath, key); err != nil {
			os.Remove(tempEncPath)
			continue
		}
		os.Remove(tempEncPath)
		restoredCount++
	}

	s.emitProgress(Progress{
		Phase:      "complete",
		TotalFiles: len(files),
		DoneFiles:  restoredCount,
		Percent:    100,
	})
	return nil
}

// FileRestoreRequest represents a file to restore at a specific version
type FileRestoreRequest struct {
	OrigPath   string    `json:"orig_path"`
	HashedName string    `json:"hashed_name"`
	TargetDate time.Time `json:"target_date"`
}

// RestoreFile restores a single file to a specific version (Time Machine style)
func (s *Service) RestoreFile(origPath string, targetDate time.Time, targetDir string) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("operation already running")
	}
	s.isRunning = true
	s.shouldStop = false
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

	// Get file info from catalog for this date
	file, err := s.catalog.GetFileAtTime(origPath, targetDate)
	if err != nil {
		return fmt.Errorf("dosya bulunamadı: %w", err)
	}
	if file == nil {
		return fmt.Errorf("dosya bu tarihte mevcut değil")
	}

	s.emitProgress(Progress{
		Phase:       "downloading",
		Message:     "Dosya sunucudan indiriliyor...",
		CurrentFile: filepath.Base(origPath),
		TotalFiles:  1,
	})

	// Request file from server
	// Use file.Timestamp (actual backup time from catalog) instead of targetDate (user-selected time)
	// This ensures the server finds the exact directory containing this version
	requestBody := struct {
		Files []struct {
			HashedName string `json:"hashed_name"`
			TargetDate string `json:"target_date"`
		} `json:"files"`
	}{
		Files: []struct {
			HashedName string `json:"hashed_name"`
			TargetDate string `json:"target_date"`
		}{
			{
				HashedName: file.HashedName,
				TargetDate: file.Timestamp.Format("2006-01-02T15:04:05"),
			},
		},
	}
	fmt.Printf("[RestoreFile] Using file.Timestamp=%v for file %s (targetDate was %v)\n", file.Timestamp, file.HashedName, targetDate)

	jsonBody, _ := json.Marshal(requestBody)
	url := fmt.Sprintf("%s/api/v1/devices/%d/restore-files", s.config.ServerURL, s.config.DeviceID)

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.config.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sunucuya bağlanılamadı: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sunucu hatası: %s", string(body))
	}

	s.emitProgress(Progress{
		Phase:   "extracting",
		Message: "Dosya çıkarılıyor ve şifre çözülüyor...",
	})

	// Response is a tar.gz file
	tempDir := filepath.Join(s.config.DataDir, "restore_temp")
	os.MkdirAll(tempDir, 0700)
	defer os.RemoveAll(tempDir)

	// Decompress gzip
	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip açılamadı: %w", err)
	}
	defer gzReader.Close()

	// Extract tar
	tarReader := tar.NewReader(gzReader)
	key := crypto.DeriveKey(s.config.EncryptionKey)

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar okuma hatası: %w", err)
		}

		// Save encrypted file temporarily
		encPath := filepath.Join(tempDir, hdr.Name)
		encFile, err := os.Create(encPath)
		if err != nil {
			continue
		}
		io.Copy(encFile, tarReader)
		encFile.Close()

		// Determine destination path
		var destPath string
		if targetDir != "" {
			destPath = filepath.Join(targetDir, filepath.Base(origPath))
		} else {
			destPath = origPath
		}

		// Create directory structure
		os.MkdirAll(filepath.Dir(destPath), 0755)

		// Decrypt file
		if err := crypto.DecryptFile(encPath, destPath, key); err != nil {
			os.Remove(encPath)
			return fmt.Errorf("şifre çözme hatası: %w", err)
		}
		os.Remove(encPath)
	}

	s.emitProgress(Progress{
		Phase:      "complete",
		Message:    "Dosya başarıyla geri yüklendi",
		TotalFiles: 1,
		DoneFiles:  1,
		Percent:    100,
	})

	return nil
}

// RestoreDirectory restores all files in a directory at a specific point in time
func (s *Service) RestoreDirectory(dirPath string, targetDate time.Time, targetDir string) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("operation already running")
	}
	s.isRunning = true
	s.shouldStop = false
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

	// Get all files in directory at this point in time
	files, err := s.catalog.GetFilesInDirAtTime(dirPath, targetDate)
	if err != nil {
		return fmt.Errorf("dizin dosyaları alınamadı: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("bu dizinde geri yüklenecek dosya bulunamadı")
	}

	totalFiles := len(files)
	s.emitProgress(Progress{
		Phase:      "downloading",
		Message:    fmt.Sprintf("%d dosya geri yükleniyor...", totalFiles),
		TotalFiles: totalFiles,
	})

	// Build request with all files
	type fileRequest struct {
		HashedName string `json:"hashed_name"`
		TargetDate string `json:"target_date"`
	}
	requestFiles := make([]fileRequest, len(files))
	for i, f := range files {
		requestFiles[i] = fileRequest{
			HashedName: f.HashedName,
			TargetDate: f.Timestamp.Format("2006-01-02T15:04:05"),
		}
	}

	requestBody := struct {
		Files []fileRequest `json:"files"`
	}{Files: requestFiles}

	jsonBody, _ := json.Marshal(requestBody)
	fmt.Printf("[RestoreDirectory] Request body: %s\n", string(jsonBody))
	url := fmt.Sprintf("%s/api/v1/devices/%d/restore-files", s.config.ServerURL, s.config.DeviceID)
	fmt.Printf("[RestoreDirectory] URL: %s\n", url)

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.config.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sunucuya bağlanılamadı: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("[RestoreDirectory] Response status: %d\n", resp.StatusCode)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sunucu hatası: %s", string(body))
	}

	s.emitProgress(Progress{
		Phase:   "extracting",
		Message: "Dosyalar çıkarılıyor ve şifre çözülüyor...",
	})

	// Response is a tar.gz file
	tempDir := filepath.Join(s.config.DataDir, "restore_temp")
	os.MkdirAll(tempDir, 0700)
	defer os.RemoveAll(tempDir)

	// Decompress gzip
	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip açılamadı: %w", err)
	}
	defer gzReader.Close()

	// Extract tar
	tarReader := tar.NewReader(gzReader)
	key := crypto.DeriveKey(s.config.EncryptionKey)

	// Create a map of hashed name -> original path for quick lookup
	hashToPath := make(map[string]string)
	for _, f := range files {
		hashToPath[f.HashedName] = f.OrigPath
		fmt.Printf("[RestoreDirectory] Hash map: %s -> %s\n", f.HashedName, f.OrigPath)
	}

	doneFiles := 0
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			fmt.Printf("[RestoreDirectory] Tar EOF reached, processed %d files\n", doneFiles)
			break
		}
		if err != nil {
			return fmt.Errorf("tar okuma hatası: %w", err)
		}

		fmt.Printf("[RestoreDirectory] Tar entry: %s (size: %d)\n", hdr.Name, hdr.Size)

		// Save encrypted file temporarily
		encPath := filepath.Join(tempDir, hdr.Name)
		encFile, err := os.Create(encPath)
		if err != nil {
			fmt.Printf("[RestoreDirectory] Failed to create temp file: %v\n", err)
			continue
		}
		written, _ := io.Copy(encFile, tarReader)
		encFile.Close()
		fmt.Printf("[RestoreDirectory] Wrote %d bytes to temp file\n", written)

		// Find original path from hashed name (strip .enc extension if present)
		hashName := strings.TrimSuffix(hdr.Name, ".enc")
		origPath, ok := hashToPath[hashName]
		if !ok {
			fmt.Printf("[RestoreDirectory] Hash not found in map: %s (tried: %s)\n", hdr.Name, hashName)
			os.Remove(encPath)
			continue
		}
		fmt.Printf("[RestoreDirectory] Matched to orig path: %s\n", origPath)

		// Determine destination path - preserve directory structure relative to dirPath
		var destPath string
		if targetDir != "" {
			relPath := strings.TrimPrefix(origPath, dirPath)
			destPath = filepath.Join(targetDir, relPath)
		} else {
			destPath = origPath
		}

		// Create directory structure
		os.MkdirAll(filepath.Dir(destPath), 0755)

		// Decrypt file
		if err := crypto.DecryptFile(encPath, destPath, key); err != nil {
			os.Remove(encPath)
			fmt.Printf("[RestoreDirectory] Decrypt error for %s: %v\n", origPath, err)
			continue
		}
		os.Remove(encPath)
		doneFiles++

		s.emitProgress(Progress{
			Phase:       "extracting",
			Message:     fmt.Sprintf("%d/%d dosya geri yüklendi", doneFiles, totalFiles),
			TotalFiles:  totalFiles,
			DoneFiles:   doneFiles,
			Percent:     float64(doneFiles) / float64(totalFiles) * 100,
			CurrentFile: filepath.Base(origPath),
		})
	}

	s.emitProgress(Progress{
		Phase:      "complete",
		Message:    fmt.Sprintf("%d dosya başarıyla geri yüklendi", doneFiles),
		TotalFiles: totalFiles,
		DoneFiles:  doneFiles,
		Percent:    100,
	})

	return nil
}

// Restore restores a specific backup (legacy method for single tar file)
func (s *Service) Restore(backupID uint, targetDir string) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("operation already running")
	}
	s.isRunning = true
	s.shouldStop = false
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

	key := crypto.DeriveKey(s.config.EncryptionKey)

	// Download backup tar
	s.emitProgress(Progress{Phase: "downloading"})

	url := fmt.Sprintf("%s/api/v1/devices/%d/backups/%d/download",
		s.config.ServerURL, s.config.DeviceID, backupID)

	tmpTar := filepath.Join(s.config.DataDir, "restore.tar")
	if err := s.downloadFile(url, tmpTar); err != nil {
		return err
	}
	defer os.Remove(tmpTar)

	// Extract and decrypt files
	s.emitProgress(Progress{Phase: "extracting"})

	tarFile, err := os.Open(tmpTar)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	tarReader := tar.NewReader(tarFile)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Get original path from catalog
		hashedName := strings.TrimSuffix(header.Name, ".enc")
		origPath, err := s.catalog.GetOriginalPath(hashedName)
		if err != nil {
			continue // Skip unknown files
		}

		// Extract encrypted file
		encPath := filepath.Join(s.config.DataDir, header.Name)
		encFile, err := os.Create(encPath)
		if err != nil {
			continue
		}
		io.Copy(encFile, tarReader)
		encFile.Close()

		// Decrypt to target
		var destPath string
		if targetDir != "" {
			// Restore to alternate location
			destPath = filepath.Join(targetDir, origPath)
		} else {
			destPath = origPath
		}

		os.MkdirAll(filepath.Dir(destPath), 0755)
		if err := crypto.DecryptFile(encPath, destPath, key); err != nil {
			os.Remove(encPath)
			continue
		}
		os.Remove(encPath)

		s.emitProgress(Progress{
			Phase:       "extracting",
			CurrentFile: filepath.Base(origPath),
		})
	}

	s.emitProgress(Progress{Phase: "complete", Percent: 100})
	return nil
}

// GetBackupDates returns all available backup dates for Time Machine UI
func (s *Service) GetBackupDates() ([]time.Time, error) {
	if s.catalog == nil {
		return []time.Time{}, nil
	}
	return s.catalog.GetBackupDates()
}

// GetFileHistory returns version history of a file
func (s *Service) GetFileHistory(filePath string) ([]catalog.FileVersion, error) {
	if s.catalog == nil {
		return []catalog.FileVersion{}, nil
	}
	return s.catalog.GetFileHistory(filePath)
}

// GetCatalogFiles returns all files with version info for Time Machine UI
func (s *Service) GetCatalogFiles() ([]catalog.CatalogFileInfo, error) {
	if s.catalog == nil {
		return []catalog.CatalogFileInfo{}, nil
	}
	return s.catalog.GetAllFilesWithInfo()
}

// GetCatalogFilesAtTimestamp returns files as they were at a specific timestamp
func (s *Service) GetCatalogFilesAtTimestamp(ts time.Time) ([]catalog.CatalogFileInfo, error) {
	if s.catalog == nil {
		return []catalog.CatalogFileInfo{}, nil
	}
	return s.catalog.GetFilesAtTimestamp(ts)
}

// GetCatalogDirectories returns all directories in the catalog
func (s *Service) GetCatalogDirectories() ([]string, error) {
	if s.catalog == nil {
		return []string{}, nil
	}
	return s.catalog.GetDirectories()
}

// GetFilesInDirectory returns files in a specific directory
func (s *Service) GetFilesInDirectory(directory string) ([]catalog.CatalogFileInfo, error) {
	if s.catalog == nil {
		return []catalog.CatalogFileInfo{}, nil
	}
	return s.catalog.GetFilesInDirectory(directory)
}

func (s *Service) GetQuota() (*QuotaInfo, error) {
	resp, err := s.get("/api/v1/account/quota")
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool      `json:"success"`
		Data    QuotaInfo `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (s *Service) GetUsage() (*UsageInfo, error) {
	resp, err := s.get("/api/v1/account/usage")
	if err != nil {
		return nil, err
	}

	var result struct {
		Success bool      `json:"success"`
		Data    UsageInfo `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// Private methods

// scanDirectoryIncrementalWithProgress scans a directory and returns files that need backup
// Uses content hash comparison - only backs up files with changed content (Time Machine style)
// seenPaths prevents duplicate backups when backup directories overlap
// Returns: files to backup, total scanned count, skipped (unchanged) count, error
func (s *Service) scanDirectoryIncrementalWithProgress(dir string, seenPaths map[string]bool) ([]fileToBackup, int, int, error) {
	var files []fileToBackup
	var scannedCount int
	var skippedCount int // Files skipped because unchanged

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if info.Size() == 0 {
			return nil
		}

		// Skip if already seen (handles overlapping directories)
		absPath, _ := filepath.Abs(path)
		fmt.Printf("[SCAN] path=%s absPath=%s seen=%v\n", path, absPath, seenPaths[absPath])
		if seenPaths[absPath] {
			fmt.Printf("[SKIP] Already seen: %s\n", absPath)
			return nil
		}
		seenPaths[absPath] = true

		scannedCount++

		// Emit progress every 100 files
		if scannedCount%100 == 0 {
			s.emitProgress(Progress{
				Phase:      "scanning",
				Message:    fmt.Sprintf("%d dosya tarandı, %d değişmemiş...", scannedCount, skippedCount),
				CurrentDir: dir,
				DoneFiles:  scannedCount,
			})
		}

		// Check blacklist
		ext := strings.ToLower(filepath.Ext(path))
		for _, blocked := range s.config.Blacklist {
			if ext == blocked || ext == "."+blocked {
				return nil
			}
		}

		// Calculate content hash
		contentHash, err := crypto.HashFileContent(path)
		if err != nil {
			return nil // Skip files we can't read
		}

		// Check if file needs backup (new or content changed)
		needsBackup, err := s.catalog.NeedsBackup(path, contentHash, info.Size())
		if err != nil {
			needsBackup = true // Backup if we can't determine
		}

		if needsBackup {
			files = append(files, fileToBackup{
				path:        path,
				size:        info.Size(),
				modTime:     info.ModTime(),
				hashedName:  crypto.HashPath(path),
				contentHash: contentHash,
			})
		} else {
			skippedCount++ // File unchanged, skip
		}

		return nil
	})

	return files, scannedCount, skippedCount, err
}

// scanDirectoryIncremental scans a directory and returns files that need backup (legacy method)
func (s *Service) scanDirectoryIncremental(dir string) ([]fileToBackup, error) {
	seenPaths := make(map[string]bool)
	files, _, _, err := s.scanDirectoryIncrementalWithProgress(dir, seenPaths)
	return files, err
}

func (s *Service) uploadTar(tarPath, sessionID string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(tarPath))
	if err != nil {
		return err
	}
	io.Copy(part, file)

	writer.WriteField("session_id", sessionID)
	writer.Close()

	url := fmt.Sprintf("%s/api/v1/devices/%d/backups", s.config.ServerURL, s.config.DeviceID)
	req, _ := http.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+s.config.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: %s", string(body))
	}

	return nil
}

func (s *Service) uploadCatalog(catalogPath, sessionID string) error {
	file, err := os.Open(catalogPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("catalog", sessionID+".katalog.enc")
	if err != nil {
		return err
	}
	io.Copy(part, file)

	writer.WriteField("session_id", sessionID)
	writer.Close()

	url := fmt.Sprintf("%s/api/v1/devices/%d/catalogs", s.config.ServerURL, s.config.DeviceID)
	req, _ := http.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+s.config.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("catalog upload failed: %s", string(body))
	}

	return nil
}

func (s *Service) downloadFile(url, destPath string) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+s.config.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (s *Service) get(path string) ([]byte, error) {
	url := s.config.ServerURL + path
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+s.config.Token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (s *Service) post(path string, body map[string]string, token string) ([]byte, error) {
	jsonBody, _ := json.Marshal(body)
	url := s.config.ServerURL + path

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (s *Service) emitProgress(p Progress) {
	if s.OnProgress != nil {
		s.OnProgress(p)
	}
}
