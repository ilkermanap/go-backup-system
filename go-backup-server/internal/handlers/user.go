package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ilker/backup-server/internal/models"
	"gorm.io/gorm"
)

type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

type UpdateUserRequest struct {
	Name string `json:"name" binding:"omitempty,min=2,max=60"`
	Plan int    `json:"plan" binding:"omitempty,min=1,max=200"`
}

type CreateUserRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=60"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Plan     int    `json:"plan" binding:"required,min=1,max=200"`
	Role     string `json:"role" binding:"omitempty,oneof=admin user"`
}

type UserListResponse struct {
	ID         uint        `json:"id"`
	Name       string      `json:"name"`
	Email      string      `json:"email"`
	Role       models.Role `json:"role"`
	Plan       int         `json:"plan"`
	UsedSpace  int64       `json:"used_space"`
	IsApproved bool        `json:"is_approved"`
	IsActive   bool        `json:"is_active"`
	CreatedAt  string      `json:"created_at"`
}

type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6"`
}

type BulkDeleteRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}

// GET /api/v1/users
func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	search := c.Query("search")
	status := c.Query("status")       // approved, pending, active, inactive
	sortBy := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	query := h.db.Model(&models.User{})

	// Search filter
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("name LIKE ? OR email LIKE ?", searchPattern, searchPattern)
	}

	// Status filter
	switch status {
	case "approved":
		query = query.Where("is_approved = ?", true)
	case "pending":
		query = query.Where("is_approved = ?", false)
	case "active":
		query = query.Where("is_active = ?", true)
	case "inactive":
		query = query.Where("is_active = ?", false)
	}

	var total int64
	query.Count(&total)

	// Sorting
	validSortFields := map[string]bool{"id": true, "name": true, "email": true, "created_at": true, "plan": true}
	if !validSortFields[sortBy] {
		sortBy = "created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	query = query.Order(sortBy + " " + sortOrder)

	var users []models.User
	if err := query.Offset(offset).Limit(perPage).Find(&users).Error; err != nil {
		InternalError(c, "Failed to fetch users")
		return
	}

	response := make([]UserListResponse, len(users))
	for i, u := range users {
		// Calculate used space for each user
		var usedSpace int64
		h.db.Model(&models.Backup{}).
			Joins("JOIN devices ON devices.id = backups.device_id").
			Where("devices.user_id = ?", u.ID).
			Select("COALESCE(SUM(backups.size), 0)").
			Scan(&usedSpace)

		response[i] = UserListResponse{
			ID:         u.ID,
			Name:       u.Name,
			Email:      u.Email,
			Role:       u.Role,
			Plan:       u.Plan,
			UsedSpace:  usedSpace,
			IsApproved: u.IsApproved,
			IsActive:   u.IsActive,
			CreatedAt:  u.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	SuccessWithMeta(c, response, &Meta{
		Page:    page,
		PerPage: perPage,
		Total:   total,
	})
}

// POST /api/v1/users
func (h *UserHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	// Check if email already exists
	var existing models.User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		BadRequest(c, "Email already exists")
		return
	}

	role := models.RoleUser
	if req.Role == "admin" {
		role = models.RoleAdmin
	}

	user := models.User{
		Name:       req.Name,
		Email:      req.Email,
		Role:       role,
		Plan:       req.Plan,
		IsApproved: true, // Admin tarafından oluşturulan kullanıcılar otomatik onaylı
		IsActive:   true,
	}

	if err := user.SetPassword(req.Password); err != nil {
		InternalError(c, "Failed to set password")
		return
	}

	if err := h.db.Create(&user).Error; err != nil {
		InternalError(c, "Failed to create user")
		return
	}

	Created(c, UserListResponse{
		ID:         user.ID,
		Name:       user.Name,
		Email:      user.Email,
		Role:       user.Role,
		Plan:       user.Plan,
		UsedSpace:  0,
		IsApproved: user.IsApproved,
		IsActive:   user.IsActive,
		CreatedAt:  user.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// GET /api/v1/users/:id
func (h *UserHandler) Get(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	var usedSpace int64
	h.db.Model(&models.Backup{}).
		Joins("JOIN devices ON devices.id = backups.device_id").
		Where("devices.user_id = ?", user.ID).
		Select("COALESCE(SUM(backups.size), 0)").
		Scan(&usedSpace)

	Success(c, UserListResponse{
		ID:         user.ID,
		Name:       user.Name,
		Email:      user.Email,
		Role:       user.Role,
		Plan:       user.Plan,
		UsedSpace:  usedSpace,
		IsApproved: user.IsApproved,
		IsActive:   user.IsActive,
		CreatedAt:  user.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// PATCH /api/v1/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Plan > 0 {
		user.Plan = req.Plan
	}

	if err := h.db.Save(&user).Error; err != nil {
		InternalError(c, "Failed to update user")
		return
	}

	var usedSpace int64
	h.db.Model(&models.Backup{}).
		Joins("JOIN devices ON devices.id = backups.device_id").
		Where("devices.user_id = ?", user.ID).
		Select("COALESCE(SUM(backups.size), 0)").
		Scan(&usedSpace)

	Success(c, UserListResponse{
		ID:         user.ID,
		Name:       user.Name,
		Email:      user.Email,
		Role:       user.Role,
		Plan:       user.Plan,
		UsedSpace:  usedSpace,
		IsApproved: user.IsApproved,
		IsActive:   user.IsActive,
		CreatedAt:  user.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// DELETE /api/v1/users/:id
func (h *UserHandler) Delete(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	// Delete user's devices and backups
	var devices []models.Device
	h.db.Where("user_id = ?", userID).Find(&devices)
	for _, device := range devices {
		h.db.Where("device_id = ?", device.ID).Delete(&models.Backup{})
	}
	h.db.Where("user_id = ?", userID).Delete(&models.Device{})
	h.db.Where("user_id = ?", userID).Delete(&models.Payment{})

	if err := h.db.Delete(&user).Error; err != nil {
		InternalError(c, "Failed to delete user")
		return
	}

	NoContent(c)
}

// POST /api/v1/users/:id/approve
func (h *UserHandler) Approve(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	if user.IsApproved {
		BadRequest(c, "User already approved")
		return
	}

	user.Approve()
	if err := h.db.Save(&user).Error; err != nil {
		InternalError(c, "Failed to approve user")
		return
	}

	Success(c, UserListResponse{
		ID:         user.ID,
		Name:       user.Name,
		Email:      user.Email,
		Role:       user.Role,
		Plan:       user.Plan,
		UsedSpace:  0,
		IsApproved: user.IsApproved,
		IsActive:   user.IsActive,
		CreatedAt:  user.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// POST /api/v1/users/:id/reset-password
func (h *UserHandler) ResetPassword(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	if err := user.SetPassword(req.Password); err != nil {
		InternalError(c, "Failed to set password")
		return
	}

	if err := h.db.Save(&user).Error; err != nil {
		InternalError(c, "Failed to update password")
		return
	}

	Success(c, gin.H{"message": "Password updated successfully"})
}

// POST /api/v1/users/:id/toggle-status
func (h *UserHandler) ToggleStatus(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "Invalid user ID")
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	// Don't allow disabling admin users
	if user.Role == models.RoleAdmin && user.IsActive {
		BadRequest(c, "Cannot disable admin users")
		return
	}

	user.IsActive = !user.IsActive
	if err := h.db.Save(&user).Error; err != nil {
		InternalError(c, "Failed to update user status")
		return
	}

	var usedSpace int64
	h.db.Model(&models.Backup{}).
		Joins("JOIN devices ON devices.id = backups.device_id").
		Where("devices.user_id = ?", user.ID).
		Select("COALESCE(SUM(backups.size), 0)").
		Scan(&usedSpace)

	Success(c, UserListResponse{
		ID:         user.ID,
		Name:       user.Name,
		Email:      user.Email,
		Role:       user.Role,
		Plan:       user.Plan,
		UsedSpace:  usedSpace,
		IsApproved: user.IsApproved,
		IsActive:   user.IsActive,
		CreatedAt:  user.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// POST /api/v1/users/bulk-delete
func (h *UserHandler) BulkDelete(c *gin.Context) {
	var req BulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	// Don't allow deleting admin users
	var adminCount int64
	h.db.Model(&models.User{}).Where("id IN ? AND role = ?", req.IDs, models.RoleAdmin).Count(&adminCount)
	if adminCount > 0 {
		BadRequest(c, "Cannot delete admin users")
		return
	}

	// Delete devices and backups for all users
	for _, userID := range req.IDs {
		var devices []models.Device
		h.db.Where("user_id = ?", userID).Find(&devices)
		for _, device := range devices {
			h.db.Where("device_id = ?", device.ID).Delete(&models.Backup{})
		}
		h.db.Where("user_id = ?", userID).Delete(&models.Device{})
		h.db.Where("user_id = ?", userID).Delete(&models.Payment{})
	}

	if err := h.db.Where("id IN ?", req.IDs).Delete(&models.User{}).Error; err != nil {
		InternalError(c, "Failed to delete users")
		return
	}

	Success(c, gin.H{"message": "Users deleted successfully", "count": len(req.IDs)})
}
