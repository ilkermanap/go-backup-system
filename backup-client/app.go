package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ilker/backup-client/internal/backup"
	"github.com/ilker/backup-client/internal/catalog"
	"github.com/ilker/backup-client/internal/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx     context.Context
	config  *config.Config
	catalog *catalog.Catalog
	backup  *backup.Service
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load or create config
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("[startup] Config load error, using default:", err)
		cfg = config.NewDefault()
	}
	a.config = cfg
	fmt.Println("[startup] DataDir:", cfg.DataDir)

	// Initialize catalog
	cat, catErr := catalog.New(cfg.DataDir)
	if catErr != nil {
		fmt.Println("[startup] Catalog init error:", catErr)
	} else {
		fmt.Println("[startup] Catalog initialized successfully")
	}
	a.catalog = cat

	// Initialize backup service
	a.backup = backup.NewService(cfg, a.catalog)
	fmt.Println("[startup] Backup service initialized")
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	if a.catalog != nil {
		a.catalog.Close()
	}
	if a.config != nil {
		a.config.Save()
	}
}

// Login authenticates with the server
func (a *App) Login(email, password string) (*LoginResult, error) {
	result, err := a.backup.Login(email, password)
	if err != nil {
		return nil, err
	}

	// Save credentials
	a.config.Email = email
	a.config.Password = password
	a.config.Token = result.Token
	a.config.Save()

	return &LoginResult{
		Success: true,
		User:    result.User,
		Token:   result.Token,
	}, nil
}

// Logout clears the session
func (a *App) Logout() {
	a.config.Token = ""
	a.config.Save()
}

// GetConfig returns current configuration
func (a *App) GetConfig() *config.Config {
	return a.config
}

// SaveConfig saves configuration
func (a *App) SaveConfig(cfg *config.Config) error {
	// Preserve internal fields that aren't in JSON
	cfg.DataDir = a.config.DataDir
	a.config = cfg
	return a.config.Save()
}

// SetEncryptionKey sets the encryption key and saves config
func (a *App) SetEncryptionKey(key string) error {
	a.config.EncryptionKey = key
	return a.config.Save()
}

// SelectDirectory opens a directory picker
func (a *App) SelectDirectory() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Yedeklenecek Dizin Seçin",
	})
	return dir, err
}

// AddBackupDirectory adds a directory to backup list
func (a *App) AddBackupDirectory(dir string) error {
	// Check if exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("dizin bulunamadı: %s", dir)
	}

	// Check if already added
	for _, d := range a.config.BackupDirs {
		if d == dir {
			return fmt.Errorf("dizin zaten ekli: %s", dir)
		}
	}

	a.config.BackupDirs = append(a.config.BackupDirs, dir)
	return a.config.Save()
}

// RemoveBackupDirectory removes a directory from backup list
func (a *App) RemoveBackupDirectory(dir string) error {
	newDirs := make([]string, 0)
	for _, d := range a.config.BackupDirs {
		if d != dir {
			newDirs = append(newDirs, d)
		}
	}
	a.config.BackupDirs = newDirs
	return a.config.Save()
}

// GetDevices returns user's devices
func (a *App) GetDevices() ([]backup.Device, error) {
	return a.backup.GetDevices()
}

// RegisterDevice registers this device
func (a *App) RegisterDevice(name string) (*backup.Device, error) {
	return a.backup.RegisterDevice(name)
}

// StartBackup initiates backup process
func (a *App) StartBackup() error {
	// Emit initial progress immediately
	runtime.EventsEmit(a.ctx, "backup:progress", backup.Progress{
		Phase:   "starting",
		Message: "Yedekleme başlatılıyor...",
		Percent: 0,
	})

	go func() {
		a.backup.OnProgress = func(progress backup.Progress) {
			runtime.EventsEmit(a.ctx, "backup:progress", progress)
		}

		err := a.backup.Run()
		if err != nil {
			runtime.EventsEmit(a.ctx, "backup:error", err.Error())
		}
	}()
	return nil
}

// StopBackup stops backup process
func (a *App) StopBackup() {
	a.backup.Stop()
}

// GetBackupStatus returns current backup status
func (a *App) GetBackupStatus() *backup.BackupStatus {
	return a.backup.GetStatus()
}

// GetBackupHistory returns backup history for a device
func (a *App) GetBackupHistory(deviceID uint) ([]backup.BackupEntry, error) {
	return a.backup.GetHistory(deviceID)
}

// StartRestore initiates restore process
func (a *App) StartRestore(backupID uint, targetDir string) error {
	go func() {
		a.backup.OnProgress = func(progress backup.Progress) {
			runtime.EventsEmit(a.ctx, "restore:progress", progress)
		}

		err := a.backup.Restore(backupID, targetDir)
		if err != nil {
			runtime.EventsEmit(a.ctx, "restore:error", err.Error())
		} else {
			runtime.EventsEmit(a.ctx, "restore:complete", nil)
		}
	}()
	return nil
}

// GetQuota returns user's quota info
func (a *App) GetQuota() (*backup.QuotaInfo, error) {
	return a.backup.GetQuota()
}

// GetUsage returns user's usage info
func (a *App) GetUsage() (*backup.UsageInfo, error) {
	return a.backup.GetUsage()
}

// HasLocalCatalog checks if local catalog has any entries
func (a *App) HasLocalCatalog() bool {
	if a.catalog == nil {
		fmt.Println("[HasLocalCatalog] catalog is nil!")
		return false
	}
	count, _, _, err := a.catalog.GetStats()
	fmt.Printf("[HasLocalCatalog] count=%d, err=%v\n", count, err)
	return count > 0
}

// RecoverCatalog downloads and restores catalog from server
func (a *App) RecoverCatalog() error {
	if a.config.EncryptionKey == "" {
		return fmt.Errorf("şifreleme anahtarı gerekli")
	}
	if a.config.DeviceID == 0 {
		return fmt.Errorf("cihaz kaydı gerekli")
	}

	go func() {
		runtime.EventsEmit(a.ctx, "catalog:recovering", nil)
		err := a.backup.RecoverCatalog()
		if err != nil {
			runtime.EventsEmit(a.ctx, "catalog:error", err.Error())
		} else {
			runtime.EventsEmit(a.ctx, "catalog:recovered", nil)
		}
	}()
	return nil
}

// GetBackupDates returns all backup dates for Time Machine UI
func (a *App) GetBackupDates() ([]string, error) {
	fmt.Println("[GetBackupDates] called")
	if a.catalog == nil {
		fmt.Println("[GetBackupDates] catalog is nil!")
		return []string{}, nil
	}
	dates, err := a.backup.GetBackupDates()
	if err != nil {
		fmt.Println("[GetBackupDates] error:", err)
		return nil, err
	}
	fmt.Printf("[GetBackupDates] found %d dates\n", len(dates))
	result := make([]string, len(dates))
	for i, d := range dates {
		result[i] = d.Format("2006-01-02 15:04:05")
	}
	return result, nil
}

// GetFileHistory returns version history for a file
func (a *App) GetFileHistory(filePath string) ([]FileVersionInfo, error) {
	versions, err := a.backup.GetFileHistory(filePath)
	if err != nil {
		return nil, err
	}
	result := make([]FileVersionInfo, len(versions))
	for i, v := range versions {
		result[i] = FileVersionInfo{
			Timestamp: v.Timestamp.Format("2006-01-02 15:04:05"),
			Size:      v.Size,
			Hash:      v.ContentHash[:8],
		}
	}
	return result, nil
}

// RestoreToDate restores files to their state at a specific date (Time Machine style)
func (a *App) RestoreToDate(dateStr string, targetDir string) error {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("geçersiz tarih formatı: %v", err)
	}

	go func() {
		a.backup.OnProgress = func(progress backup.Progress) {
			runtime.EventsEmit(a.ctx, "restore:progress", progress)
		}

		err := a.backup.RestoreToTime(t, targetDir)
		if err != nil {
			runtime.EventsEmit(a.ctx, "restore:error", err.Error())
		} else {
			runtime.EventsEmit(a.ctx, "restore:complete", nil)
		}
	}()
	return nil
}

// RestoreFile restores a single file to a specific version (Time Machine style)
func (a *App) RestoreFile(origPath string, dateStr string, targetDir string) error {
	fmt.Printf("[RESTORE] Starting restore: origPath=%s, dateStr=%s, targetDir=%s\n", origPath, dateStr, targetDir)

	// Parse in local timezone (not UTC) to match database storage
	loc := time.Local
	t, err := time.ParseInLocation("2006-01-02 15:04:05", dateStr, loc)
	if err != nil {
		// Try without seconds
		t, err = time.ParseInLocation("2006-01-02 15:04", dateStr, loc)
		if err != nil {
			// Try date only
			t, err = time.ParseInLocation("2006-01-02", dateStr, loc)
			if err != nil {
				fmt.Printf("[RESTORE] Date parse error: %v\n", err)
				return fmt.Errorf("geçersiz tarih formatı: %v", err)
			}
			// Set to end of day
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
	}
	fmt.Printf("[RESTORE] Parsed time: %v\n", t)

	go func() {
		a.backup.OnProgress = func(progress backup.Progress) {
			fmt.Printf("[RESTORE] Progress: %+v\n", progress)
			runtime.EventsEmit(a.ctx, "restore:progress", progress)
		}

		err := a.backup.RestoreFile(origPath, t, targetDir)
		if err != nil {
			fmt.Printf("[RESTORE] Error: %v\n", err)
			runtime.EventsEmit(a.ctx, "restore:error", err.Error())
		} else {
			fmt.Printf("[RESTORE] Complete!\n")
			runtime.EventsEmit(a.ctx, "restore:complete", nil)
		}
	}()
	return nil
}

// RestoreDirectory restores all files in a directory at a specific point in time
func (a *App) RestoreDirectory(dirPath string, dateStr string, targetDir string) error {
	fmt.Printf("[RESTORE_DIR] Starting restore: dirPath=%s, dateStr=%s, targetDir=%s\n", dirPath, dateStr, targetDir)

	// Parse in local timezone (not UTC) to match database storage
	loc := time.Local
	t, err := time.ParseInLocation("2006-01-02 15:04:05", dateStr, loc)
	if err != nil {
		// Try without seconds
		t, err = time.ParseInLocation("2006-01-02 15:04", dateStr, loc)
		if err != nil {
			// Try date only
			t, err = time.ParseInLocation("2006-01-02", dateStr, loc)
			if err != nil {
				fmt.Printf("[RESTORE_DIR] Date parse error: %v\n", err)
				return fmt.Errorf("geçersiz tarih formatı: %v", err)
			}
			// Set to end of day
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
	}
	fmt.Printf("[RESTORE_DIR] Parsed time: %v\n", t)

	go func() {
		a.backup.OnProgress = func(progress backup.Progress) {
			fmt.Printf("[RESTORE_DIR] Progress: %+v\n", progress)
			runtime.EventsEmit(a.ctx, "restore:progress", progress)
		}

		err := a.backup.RestoreDirectory(dirPath, t, targetDir)
		if err != nil {
			fmt.Printf("[RESTORE_DIR] Error: %v\n", err)
			runtime.EventsEmit(a.ctx, "restore:error", err.Error())
		} else {
			fmt.Printf("[RESTORE_DIR] Complete!\n")
			runtime.EventsEmit(a.ctx, "restore:complete", nil)
		}
	}()
	return nil
}

// FileVersionInfo for frontend
type FileVersionInfo struct {
	Timestamp string `json:"timestamp"`
	Size      int64  `json:"size"`
	Hash      string `json:"hash"`
}

// CatalogFileInfo for frontend (Time Machine UI)
type CatalogFileInfo struct {
	OrigPath      string `json:"orig_path"`
	Directory     string `json:"directory"`
	FileName      string `json:"file_name"`
	LatestVersion string `json:"latest_version"`
	VersionCount  int    `json:"version_count"`
	Size          int64  `json:"size"`
}

// GetCatalogFiles returns all files in catalog with version info
func (a *App) GetCatalogFiles() ([]CatalogFileInfo, error) {
	fmt.Println("[GetCatalogFiles] called")
	if a.catalog == nil {
		fmt.Println("[GetCatalogFiles] catalog is nil!")
		return []CatalogFileInfo{}, nil
	}
	files, err := a.backup.GetCatalogFiles()
	if err != nil {
		fmt.Println("[GetCatalogFiles] error:", err)
		return nil, err
	}
	fmt.Printf("[GetCatalogFiles] found %d files\n", len(files))
	result := make([]CatalogFileInfo, len(files))
	for i, f := range files {
		result[i] = CatalogFileInfo{
			OrigPath:      f.OrigPath,
			Directory:     f.Directory,
			FileName:      f.FileName,
			LatestVersion: f.LatestVersion.Format("2006-01-02 15:04"),
			VersionCount:  f.VersionCount,
			Size:          f.Size,
		}
	}
	return result, nil
}

// GetCatalogFilesAtDate returns files as they were at a specific date/time
func (a *App) GetCatalogFilesAtDate(dateStr string) ([]CatalogFileInfo, error) {
	fmt.Printf("[GetCatalogFilesAtDate] called with date: %s\n", dateStr)
	if a.catalog == nil {
		fmt.Println("[GetCatalogFilesAtDate] catalog is nil!")
		return []CatalogFileInfo{}, nil
	}

	// Parse the date string
	ts, err := time.Parse("2006-01-02 15:04:05", dateStr)
	if err != nil {
		fmt.Printf("[GetCatalogFilesAtDate] parse error: %v\n", err)
		return nil, err
	}

	files, err := a.backup.GetCatalogFilesAtTimestamp(ts)
	if err != nil {
		fmt.Printf("[GetCatalogFilesAtDate] error: %v\n", err)
		return nil, err
	}

	fmt.Printf("[GetCatalogFilesAtDate] found %d files\n", len(files))
	result := make([]CatalogFileInfo, len(files))
	for i, f := range files {
		result[i] = CatalogFileInfo{
			OrigPath:      f.OrigPath,
			Directory:     f.Directory,
			FileName:      f.FileName,
			LatestVersion: f.LatestVersion.Format("2006-01-02 15:04:05"),
			VersionCount:  f.VersionCount,
			Size:          f.Size,
		}
	}
	return result, nil
}

// GetCatalogDirectories returns all directories in catalog
func (a *App) GetCatalogDirectories() ([]string, error) {
	return a.backup.GetCatalogDirectories()
}

// GetFilesInDirectory returns files in a specific directory
func (a *App) GetFilesInDirectory(directory string) ([]CatalogFileInfo, error) {
	files, err := a.backup.GetFilesInDirectory(directory)
	if err != nil {
		return nil, err
	}
	result := make([]CatalogFileInfo, len(files))
	for i, f := range files {
		result[i] = CatalogFileInfo{
			OrigPath:      f.OrigPath,
			Directory:     f.Directory,
			FileName:      f.FileName,
			LatestVersion: f.LatestVersion.Format("2006-01-02 15:04"),
			VersionCount:  f.VersionCount,
			Size:          f.Size,
		}
	}
	return result, nil
}

// IsLoggedIn checks if user has valid token
func (a *App) IsLoggedIn() bool {
	return a.config.Token != ""
}

// GetDataDir returns the data directory path
func (a *App) GetDataDir() string {
	return a.config.DataDir
}

// LoginResult for frontend
type LoginResult struct {
	Success bool        `json:"success"`
	User    interface{} `json:"user"`
	Token   string      `json:"token"`
}

// ClearLocalCatalog clears all entries from local catalog
func (a *App) ClearLocalCatalog() error {
	if a.catalog == nil {
		return fmt.Errorf("catalog not initialized")
	}
	return a.catalog.ClearAll()
}

// GetServerBackups returns list of backups on server
func (a *App) GetServerBackups() ([]backup.BackupEntry, error) {
	return a.backup.GetHistory(a.config.DeviceID)
}

// DeleteServerBackup deletes a backup from server
func (a *App) DeleteServerBackup(backupID uint) error {
	return a.backup.DeleteBackup(backupID)
}

// DeleteAllServerBackups deletes all backups from server
func (a *App) DeleteAllServerBackups() error {
	return a.backup.DeleteAllBackups()
}
