package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	// Server settings
	ServerURL string `json:"server_url"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Token     string `json:"token"`

	// Device settings
	DeviceID   uint   `json:"device_id"`
	DeviceName string `json:"device_name"`

	// Encryption key (user-defined)
	EncryptionKey string `json:"encryption_key"`

	// Backup directories
	BackupDirs []string `json:"backup_dirs"`

	// Blacklist extensions
	Blacklist []string `json:"blacklist"`

	// Schedule settings
	ScheduleEnabled bool   `json:"schedule_enabled"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	IntervalMinutes int    `json:"interval_minutes"`
	SkipWeekends    bool   `json:"skip_weekends"`

	// Chunk size for uploads (bytes)
	ChunkSize int64 `json:"chunk_size"`

	// Data directory
	DataDir    string `json:"-"`
	configPath string `json:"-"`
}

func NewDefault() *Config {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".backup-client")
	os.MkdirAll(dataDir, 0700)

	return &Config{
		ServerURL:       "", // Empty - user must set this
		Blacklist:       []string{".mp3", ".mp4", ".wav", ".m4a", ".iso", ".vmdk", ".vdi"},
		StartTime:       "09:00",
		EndTime:         "19:00",
		IntervalMinutes: 60,
		ChunkSize:       25 * 1024 * 1024, // 25MB
		DataDir:         dataDir,
		configPath:      filepath.Join(dataDir, "config.json"),
	}
}

func Load() (*Config, error) {
	cfg := NewDefault()

	data, err := os.ReadFile(cfg.configPath)
	if err != nil {
		// Config file doesn't exist, save default and return
		cfg.Save()
		return cfg, nil
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		// Invalid JSON, return default
		return cfg, nil
	}

	return cfg, nil
}

func (c *Config) Save() error {
	// Ensure we have valid paths
	if c.DataDir == "" || c.configPath == "" {
		homeDir, _ := os.UserHomeDir()
		c.DataDir = filepath.Join(homeDir, ".backup-client")
		c.configPath = filepath.Join(c.DataDir, "config.json")
		os.MkdirAll(c.DataDir, 0700)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.configPath, data, 0600)
}
