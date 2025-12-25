package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ilker/backup-server/internal/middleware"
	"github.com/ilker/backup-server/internal/models"
	"gorm.io/gorm"
)

type DeviceHandler struct {
	db *gorm.DB
}

func NewDeviceHandler(db *gorm.DB) *DeviceHandler {
	return &DeviceHandler{db: db}
}

type CreateDeviceRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
}

type UpdateDeviceRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
}

type DeviceResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// GET /api/v1/devices
func (h *DeviceHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var devices []models.Device
	if err := h.db.Where("user_id = ?", userID).Find(&devices).Error; err != nil {
		InternalError(c, "Failed to fetch devices")
		return
	}

	response := make([]DeviceResponse, len(devices))
	for i, d := range devices {
		response[i] = DeviceResponse{
			ID:        d.ID,
			Name:      d.Name,
			CreatedAt: d.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	Success(c, response)
}

// POST /api/v1/devices
func (h *DeviceHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	device := models.Device{
		Name:   req.Name,
		UserID: userID,
	}

	if err := h.db.Create(&device).Error; err != nil {
		InternalError(c, "Failed to create device")
		return
	}

	Created(c, DeviceResponse{
		ID:        device.ID,
		Name:      device.Name,
		CreatedAt: device.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// GET /api/v1/devices/:id
func (h *DeviceHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}

	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	Success(c, DeviceResponse{
		ID:        device.ID,
		Name:      device.Name,
		CreatedAt: device.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// PATCH /api/v1/devices/:id
func (h *DeviceHandler) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}

	var req UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	device.Name = req.Name
	if err := h.db.Save(&device).Error; err != nil {
		InternalError(c, "Failed to update device")
		return
	}

	Success(c, DeviceResponse{
		ID:        device.ID,
		Name:      device.Name,
		CreatedAt: device.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// DELETE /api/v1/devices/:id
func (h *DeviceHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	deviceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid device ID")
		return
	}

	var device models.Device
	if err := h.db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		NotFound(c, "Device not found")
		return
	}

	// Also delete associated backups
	if err := h.db.Where("device_id = ?", deviceID).Delete(&models.Backup{}).Error; err != nil {
		InternalError(c, "Failed to delete device backups")
		return
	}

	if err := h.db.Delete(&device).Error; err != nil {
		InternalError(c, "Failed to delete device")
		return
	}

	NoContent(c)
}
