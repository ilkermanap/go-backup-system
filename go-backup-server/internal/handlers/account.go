package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/ilker/backup-server/internal/middleware"
	"github.com/ilker/backup-server/internal/models"
	"gorm.io/gorm"
)

type AccountHandler struct {
	db *gorm.DB
}

func NewAccountHandler(db *gorm.DB) *AccountHandler {
	return &AccountHandler{db: db}
}

type QuotaResponse struct {
	PlanGB   int     `json:"plan_gb"`
	UsedMB   float64 `json:"used_mb"`
	UsedGB   float64 `json:"used_gb"`
	FreeMB   float64 `json:"free_mb"`
	FreeGB   float64 `json:"free_gb"`
	UsedPerc float64 `json:"used_percentage"`
}

type UsageResponse struct {
	TotalBackups  int64            `json:"total_backups"`
	TotalDevices  int64            `json:"total_devices"`
	TotalSizeMB   float64          `json:"total_size_mb"`
	DeviceUsage   []DeviceUsage    `json:"device_usage"`
}

type DeviceUsage struct {
	DeviceID    uint    `json:"device_id"`
	DeviceName  string  `json:"device_name"`
	BackupCount int64   `json:"backup_count"`
	SizeMB      float64 `json:"size_mb"`
}

// GET /api/v1/account/quota
func (h *AccountHandler) Quota(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	usedBytes := h.calculateUsage(userID)
	planBytes := int64(user.Plan) * 1024 * 1024 * 1024

	usedMB := float64(usedBytes) / (1024 * 1024)
	usedGB := float64(usedBytes) / (1024 * 1024 * 1024)
	freeMB := float64(planBytes-usedBytes) / (1024 * 1024)
	freeGB := float64(planBytes-usedBytes) / (1024 * 1024 * 1024)
	usedPerc := float64(usedBytes) / float64(planBytes) * 100

	if usedPerc < 0 {
		usedPerc = 0
	}

	Success(c, QuotaResponse{
		PlanGB:   user.Plan,
		UsedMB:   usedMB,
		UsedGB:   usedGB,
		FreeMB:   freeMB,
		FreeGB:   freeGB,
		UsedPerc: usedPerc,
	})
}

// GET /api/v1/account/usage
func (h *AccountHandler) Usage(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var devices []models.Device
	if err := h.db.Where("user_id = ?", userID).Find(&devices).Error; err != nil {
		InternalError(c, "Failed to fetch devices")
		return
	}

	var totalBackups int64
	var totalSize int64
	deviceUsage := make([]DeviceUsage, 0, len(devices))

	for _, device := range devices {
		var backups []models.Backup
		h.db.Where("device_id = ?", device.ID).Find(&backups)

		var deviceSize int64
		for _, b := range backups {
			deviceSize += b.FileSize
			totalSize += b.FileSize
		}
		totalBackups += int64(len(backups))

		deviceUsage = append(deviceUsage, DeviceUsage{
			DeviceID:    device.ID,
			DeviceName:  device.Name,
			BackupCount: int64(len(backups)),
			SizeMB:      float64(deviceSize) / (1024 * 1024),
		})
	}

	Success(c, UsageResponse{
		TotalBackups: totalBackups,
		TotalDevices: int64(len(devices)),
		TotalSizeMB:  float64(totalSize) / (1024 * 1024),
		DeviceUsage:  deviceUsage,
	})
}

func (h *AccountHandler) calculateUsage(userID uint) int64 {
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
