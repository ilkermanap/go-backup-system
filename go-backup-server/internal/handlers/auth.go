package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/ilker/backup-server/internal/middleware"
	"github.com/ilker/backup-server/internal/models"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db      *gorm.DB
	jwtAuth *middleware.JWTAuth
}

func NewAuthHandler(db *gorm.DB, jwtAuth *middleware.JWTAuth) *AuthHandler {
	return &AuthHandler{
		db:      db,
		jwtAuth: jwtAuth,
	}
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=60"`
	Email    string `json:"email" binding:"required,email,max=60"`
	Password string `json:"password" binding:"required,min=6"`
	Plan     int    `json:"plan" binding:"omitempty,min=1,max=200"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type UserResponse struct {
	ID         uint        `json:"id"`
	Name       string      `json:"name"`
	Email      string      `json:"email"`
	Role       models.Role `json:"role"`
	Plan       int         `json:"plan"`
	IsApproved bool        `json:"is_approved"`
}

// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	// Check if email exists
	var existing models.User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		Conflict(c, "Email already registered")
		return
	}

	// Check if this is the first user (make admin)
	var count int64
	h.db.Model(&models.User{}).Count(&count)

	user := models.User{
		Name:  req.Name,
		Email: req.Email,
		Plan:  req.Plan,
		Role:  models.RoleUser,
	}

	// First user becomes admin and auto-approved
	if count == 0 {
		user.Role = models.RoleAdmin
		user.Approve()
	}

	if user.Plan == 0 {
		user.Plan = 1 // Default 1GB
	}

	if err := user.SetPassword(req.Password); err != nil {
		InternalError(c, "Failed to process password")
		return
	}

	if err := h.db.Create(&user).Error; err != nil {
		InternalError(c, "Failed to create user")
		return
	}

	token, err := h.jwtAuth.GenerateToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		InternalError(c, "Failed to generate token")
		return
	}

	Created(c, AuthResponse{
		Token: token,
		User: UserResponse{
			ID:         user.ID,
			Name:       user.Name,
			Email:      user.Email,
			Role:       user.Role,
			Plan:       user.Plan,
			IsApproved: user.IsApproved,
		},
	})
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		Unauthorized(c, "Invalid email or password")
		return
	}

	if !user.CheckPassword(req.Password) {
		Unauthorized(c, "Invalid email or password")
		return
	}

	if !user.IsApproved {
		Forbidden(c, "Account not approved yet")
		return
	}

	token, err := h.jwtAuth.GenerateToken(user.ID, user.Email, string(user.Role))
	if err != nil {
		InternalError(c, "Failed to generate token")
		return
	}

	Success(c, AuthResponse{
		Token: token,
		User: UserResponse{
			ID:         user.ID,
			Name:       user.Name,
			Email:      user.Email,
			Role:       user.Role,
			Plan:       user.Plan,
			IsApproved: user.IsApproved,
		},
	})
}

// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// JWT is stateless, client should discard token
	Success(c, gin.H{"message": "Logged out successfully"})
}

// GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		NotFound(c, "User not found")
		return
	}

	Success(c, UserResponse{
		ID:         user.ID,
		Name:       user.Name,
		Email:      user.Email,
		Role:       user.Role,
		Plan:       user.Plan,
		IsApproved: user.IsApproved,
	})
}
