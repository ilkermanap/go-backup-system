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

type UserListResponse struct {
	ID         uint        `json:"id"`
	Name       string      `json:"name"`
	Email      string      `json:"email"`
	Role       models.Role `json:"role"`
	Plan       int         `json:"plan"`
	IsApproved bool        `json:"is_approved"`
	CreatedAt  string      `json:"created_at"`
}

// GET /api/v1/users
func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	var total int64
	h.db.Model(&models.User{}).Count(&total)

	var users []models.User
	if err := h.db.Offset(offset).Limit(perPage).Find(&users).Error; err != nil {
		InternalError(c, "Failed to fetch users")
		return
	}

	response := make([]UserListResponse, len(users))
	for i, u := range users {
		response[i] = UserListResponse{
			ID:         u.ID,
			Name:       u.Name,
			Email:      u.Email,
			Role:       u.Role,
			Plan:       u.Plan,
			IsApproved: u.IsApproved,
			CreatedAt:  u.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	SuccessWithMeta(c, response, &Meta{
		Page:    page,
		PerPage: perPage,
		Total:   total,
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

	Success(c, UserListResponse{
		ID:         user.ID,
		Name:       user.Name,
		Email:      user.Email,
		Role:       user.Role,
		Plan:       user.Plan,
		IsApproved: user.IsApproved,
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

	Success(c, UserListResponse{
		ID:         user.ID,
		Name:       user.Name,
		Email:      user.Email,
		Role:       user.Role,
		Plan:       user.Plan,
		IsApproved: user.IsApproved,
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
		IsApproved: user.IsApproved,
		CreatedAt:  user.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}
