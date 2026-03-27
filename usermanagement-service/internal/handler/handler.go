package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"diploma/usermanagement-service/internal/model"
	"diploma/usermanagement-service/internal/repository"
	"diploma/usermanagement-service/internal/service"
)

type AuthService interface {
	Register(ctx context.Context, input service.RegisterInput) (service.RegisterResult, error)
	VerifyEmail(ctx context.Context, token string) error
	Login(ctx context.Context, input service.LoginInput) (service.LoginResult, error)
	Refresh(ctx context.Context, refreshToken string) (service.LoginResult, error)
	ResendVerification(ctx context.Context, email string) error
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	SelectRole(ctx context.Context, userID int, role model.Role) error
	CreateCustomerProfile(ctx context.Context, userID int) error
	CreateWorkerProfile(ctx context.Context, userID int) error
	AddWorkerSkill(ctx context.Context, userID int, categoryID int, experience string, price int) error
	VerifyWorkerSkill(ctx context.Context, skillID int) error
	SetAvailability(ctx context.Context, userID int, available bool) error
	FindWorkers(ctx context.Context, categoryID int) ([]repository.WorkerSearchResult, error)
	VerifyWorker(ctx context.Context, workerID int) error
}

type AuthHandler struct {
	auth AuthService
}

func NewAuthHandler(auth AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type registerRequest struct {
	FullName string  `json:"full_name"`
	Email    string  `json:"email"`
	Phone    *string `json:"phone"`
	Password string  `json:"password"`
	Role     string  `json:"role"`
}

type registerResponse struct {
	UserID    int       `json:"user_id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone,omitempty"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Message   string    `json:"message"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	result, err := h.auth.Register(c.Request.Context(), service.RegisterInput{
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		Password: req.Password,
		Role:     model.Role(req.Role),
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, registerResponse{
		UserID:    result.User.ID,
		FullName:  result.User.FullName,
		Email:     result.User.Email,
		Phone:     result.User.Phone,
		Role:      string(result.User.Role),
		Status:    string(result.User.Status),
		CreatedAt: result.User.CreatedAt,
		Message:   "check your email to verify account",
	})
}
func (h *AuthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AuthHandler) handleError(c *gin.Context, err error) {
	if service.IsValidationError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch {
	case errors.Is(err, repository.ErrEmailAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "email already exists"})
	case errors.Is(err, repository.ErrPhoneAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "phone already exists"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
func (h *AuthHandler) Verify(c *gin.Context) {
	token := c.Query("token")

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token"})
		return
	}

	err := h.auth.VerifyEmail(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "email verified successfully",
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	result, err := h.auth.Login(c.Request.Context(), service.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"token_type":    "Bearer",
		"expires_at":    result.ExpiresAt,
	})
}
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	result, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"access_token": result.AccessToken,
		"expires_at":   result.ExpiresAt,
	})
}
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	err := h.auth.ResendVerification(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message": "verification email sent",
	})
}
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}

	_ = c.ShouldBindJSON(&req)

	_ = h.auth.ForgotPassword(c.Request.Context(), req.Email)

	c.JSON(200, gin.H{"message": "if email exists, reset link sent"})
}
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	err := h.auth.ResetPassword(c.Request.Context(), req.Token, req.NewPassword)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "password updated"})
}
func (h *AuthHandler) SelectRole(c *gin.Context) {
	var req struct {
		Role string `json:"role"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	userID := c.GetInt("user_id")

	err := h.auth.SelectRole(
		c.Request.Context(),
		userID,
		model.Role(req.Role),
	)

	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "role selected"})
}
func (h *AuthHandler) CreateCustomerProfile(c *gin.Context) {
	userID := c.GetInt("user_id")

	err := h.auth.CreateCustomerProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "customer profile created"})
}
func (h *AuthHandler) CreateWorkerProfile(c *gin.Context) {
	userID := c.GetInt("user_id")

	err := h.auth.CreateWorkerProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "worker profile created"})
}
func (h *AuthHandler) AddWorkerSkill(c *gin.Context) {
	var req struct {
		CategoryID int    `json:"category_id"`
		Experience string `json:"experience_level"`
		Price      int    `json:"price_base"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	userID := c.GetInt("user_id")

	err := h.auth.AddWorkerSkill(
		c.Request.Context(),
		userID,
		req.CategoryID,
		req.Experience,
		req.Price,
	)

	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "skill added"})
}
func (h *AuthHandler) VerifyWorkerSkill(c *gin.Context) {
	var req struct {
		SkillID int `json:"worker_skill_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	err := h.auth.VerifyWorkerSkill(c.Request.Context(), req.SkillID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "skill verified"})
}
func (h *AuthHandler) SetAvailability(c *gin.Context) {
	var req struct {
		IsAvailable bool `json:"is_available"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	userID := c.GetInt("user_id")

	err := h.auth.SetAvailability(c.Request.Context(), userID, req.IsAvailable)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "availability updated"})
}
func (h *AuthHandler) FindWorkers(c *gin.Context) {
	categoryIDStr := c.Query("category_id")
	if categoryIDStr == "" {
		c.JSON(400, gin.H{"error": "category_id is required"})
		return
	}

	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid category_id"})
		return
	}

	result, err := h.auth.FindWorkers(c.Request.Context(), categoryID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, result)
}
func (h *AuthHandler) VerifyWorker(c *gin.Context) {
	var req struct {
		WorkerID int `json:"worker_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	err := h.auth.VerifyWorker(c.Request.Context(), req.WorkerID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "worker verified"})
}
