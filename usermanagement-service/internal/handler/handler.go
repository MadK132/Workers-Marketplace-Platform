package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"diploma/usermanagement-service/internal/filestorage"
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
	UpsertCustomerProfile(ctx context.Context, userID int, address string, latitude *float64, longitude *float64, bio string, profilePhotoURL *string) (*model.CustomerProfile, error)
	CreateWorkerProfile(ctx context.Context, userID int) error
	UpsertWorkerProfile(ctx context.Context, userID int, bio string, latitude *float64, longitude *float64, profilePhotoURL *string) (*model.WorkerProfile, error)
	AddWorkerSkill(ctx context.Context, userID int, categoryID int, experience string, price int) (int, error)
	AddWorkerSkillEvidence(ctx context.Context, workerSkillID int, fileName string, filePath string, contentType string, note string) error
	VerifyWorkerSkill(ctx context.Context, skillID int) error
	SetAvailability(ctx context.Context, userID int, available bool) error
	FindWorkers(ctx context.Context, categoryID int) ([]repository.WorkerSearchResult, error)
	GetCustomerProfile(ctx context.Context, userID int) (*model.CustomerProfile, error)
	GetWorkerProfile(ctx context.Context, userID int) (*model.WorkerProfile, error)
	ListVerifiedWorkerSkills(ctx context.Context, userID int) ([]repository.VerifiedWorkerSkill, error)
	GetCategories(ctx context.Context) ([]repository.ServiceCategory, error)
	GetAdminOverview(ctx context.Context) (repository.AdminOverview, error)
	ListUsers(ctx context.Context) ([]repository.UserSummary, error)
	DeleteUser(ctx context.Context, userID int) error
	ActivateUser(ctx context.Context, userID int) error
	CreateAdmin(ctx context.Context, input service.RegisterInput) (model.User, error)
	CreateManager(ctx context.Context, input service.RegisterInput) (model.User, error)
	UpsertPaymentMethod(ctx context.Context, userID int, provider string, last4 string) (repository.PaymentMethod, error)
	GetPaymentMethod(ctx context.Context, userID int) (repository.PaymentMethod, error)
	HasPaymentMethod(ctx context.Context, userID int) (bool, error)
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

func (h *AuthHandler) ResetPasswordPage(c *gin.Context) {
	redirectToApp(c, "/auth/reset")
}

func redirectToApp(c *gin.Context, path string) {
	baseURL := os.Getenv("APP_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5173"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	target := baseURL + path
	if c.Request.URL.RawQuery != "" {
		target += "?" + c.Request.URL.RawQuery
	}
	c.Redirect(http.StatusFound, target)
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

	address, latitude, longitude, bio, photoURL, ok := h.parseCustomerProfileRequest(c)
	if !ok {
		return
	}

	profile, err := h.auth.UpsertCustomerProfile(c.Request.Context(), userID, address, latitude, longitude, bio, photoURL)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"customer_profile_id": profile.ID,
		"user_id":             profile.UserID,
		"address":             profile.Address,
		"latitude":            profile.Latitude,
		"longitude":           profile.Longitude,
		"bio":                 profile.Bio,
		"profile_photo_url":   profile.ProfilePhotoURL,
	})
}

func (h *AuthHandler) parseCustomerProfileRequest(c *gin.Context) (string, *float64, *float64, string, *string, bool) {
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		latitude, err := optionalFloat(c.PostForm("latitude"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid latitude"})
			return "", nil, nil, "", nil, false
		}
		longitude, err := optionalFloat(c.PostForm("longitude"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid longitude"})
			return "", nil, nil, "", nil, false
		}
		var photoURL *string
		file, err := c.FormFile("profile_photo")
		if err == nil && file != nil {
			storedPath, err := saveProfilePhoto(c.Request.Context(), file)
			if err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return "", nil, nil, "", nil, false
			}
			photoURL = &storedPath
		}
		return c.PostForm("address"), latitude, longitude, c.PostForm("bio"), photoURL, true
	}

	var req struct {
		Address         string   `json:"address"`
		Latitude        *float64 `json:"latitude"`
		Longitude       *float64 `json:"longitude"`
		Bio             string   `json:"bio"`
		ProfilePhotoURL *string  `json:"profile_photo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return "", nil, nil, "", nil, false
	}
	return req.Address, req.Latitude, req.Longitude, req.Bio, req.ProfilePhotoURL, true
}
func (h *AuthHandler) CreateWorkerProfile(c *gin.Context) {
	userID := c.GetInt("user_id")

	bio, latitude, longitude, photoURL, ok := h.parseWorkerProfileRequest(c)
	if !ok {
		return
	}

	profile, err := h.auth.UpsertWorkerProfile(c.Request.Context(), userID, bio, latitude, longitude, photoURL)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"worker_profile_id":   profile.ID,
		"user_id":             profile.UserID,
		"bio":                 profile.Bio,
		"rating":              profile.Rating,
		"verification_status": profile.VerificationStatus,
		"is_available":        profile.IsAvailable,
		"current_latitude":    profile.CurrentLatitude,
		"current_longitude":   profile.CurrentLongitude,
		"profile_photo_url":   profile.ProfilePhotoURL,
	})
}

func (h *AuthHandler) parseWorkerProfileRequest(c *gin.Context) (string, *float64, *float64, *string, bool) {
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		bio := c.PostForm("bio")
		latitude, err := optionalFloat(c.PostForm("current_latitude"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid current_latitude"})
			return "", nil, nil, nil, false
		}
		longitude, err := optionalFloat(c.PostForm("current_longitude"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid current_longitude"})
			return "", nil, nil, nil, false
		}
		var photoURL *string
		file, err := c.FormFile("profile_photo")
		if err == nil && file != nil {
			storedPath, err := saveProfilePhoto(c.Request.Context(), file)
			if err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return "", nil, nil, nil, false
			}
			photoURL = &storedPath
		}
		return bio, latitude, longitude, photoURL, true
	}

	var req struct {
		Bio              string   `json:"bio"`
		CurrentLatitude  *float64 `json:"current_latitude"`
		CurrentLongitude *float64 `json:"current_longitude"`
		ProfilePhotoURL  *string  `json:"profile_photo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return "", nil, nil, nil, false
	}
	return req.Bio, req.CurrentLatitude, req.CurrentLongitude, req.ProfilePhotoURL, true
}
func (h *AuthHandler) AddWorkerSkill(c *gin.Context) {
	categoryID, experience, price, note, files, ok := h.parseWorkerSkillRequest(c)
	if !ok {
		return
	}

	userID := c.GetInt("user_id")

	skillID, err := h.auth.AddWorkerSkill(
		c.Request.Context(),
		userID,
		categoryID,
		experience,
		price,
	)

	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	for _, file := range files {
		storedPath, err := saveEvidenceFile(c.Request.Context(), file)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := h.auth.AddWorkerSkillEvidence(
			c.Request.Context(),
			skillID,
			file.Filename,
			storedPath,
			file.Header.Get("Content-Type"),
			note,
		); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(200, gin.H{"message": "skill added", "worker_skill_id": skillID})
}

func (h *AuthHandler) parseWorkerSkillRequest(c *gin.Context) (int, string, int, string, []*multipart.FileHeader, bool) {
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		categoryID, err := strconv.Atoi(c.PostForm("category_id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid category_id"})
			return 0, "", 0, "", nil, false
		}
		price, err := strconv.Atoi(firstNonEmpty(c.PostForm("price_base"), c.PostForm("price")))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid price"})
			return 0, "", 0, "", nil, false
		}
		form, _ := c.MultipartForm()
		var files []*multipart.FileHeader
		if form != nil {
			files = form.File["evidence_files"]
		}
		return categoryID, c.PostForm("experience_level"), price, c.PostForm("evidence_note"), files, true
	}

	var req struct {
		CategoryID int    `json:"category_id"`
		Experience string `json:"experience_level"`
		PriceBase  int    `json:"price_base"`
		Price      int    `json:"price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return 0, "", 0, "", nil, false
	}
	price := req.PriceBase
	if price == 0 {
		price = req.Price
	}
	return req.CategoryID, req.Experience, price, "", nil, true
}

func saveEvidenceFile(ctx context.Context, file *multipart.FileHeader) (string, error) {
	return filestorage.SaveUploadedFile(ctx, file, filestorage.SaveOptions{
		Prefix:      "verification",
		MaxSize:     10 * 1024 * 1024,
		AllowedExts: map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".pdf": true},
	})
}

func saveProfilePhoto(ctx context.Context, file *multipart.FileHeader) (string, error) {
	return filestorage.SaveUploadedFile(ctx, file, filestorage.SaveOptions{
		Prefix:      "profiles",
		MaxSize:     5 * 1024 * 1024,
		AllowedExts: map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true},
	})
}

func optionalFloat(value string) (*float64, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func isAdminRole(c *gin.Context) bool {
	return c.GetString("role") == string(model.RoleAdmin)
}

func isStaffRole(c *gin.Context) bool {
	role := c.GetString("role")
	return role == string(model.RoleAdmin) || role == string(model.RoleManager)
}

func (h *AuthHandler) VerifyWorkerSkill(c *gin.Context) {
	if !isStaffRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only staff allowed"})
		return
	}

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

	c.JSON(200, gin.H{"message": "skill and worker verified"})
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

func (h *AuthHandler) UpsertPaymentMethod(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
		Last4    string `json:"last4"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	method, err := h.auth.UpsertPaymentMethod(c.Request.Context(), c.GetInt("user_id"), req.Provider, req.Last4)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, method)
}

func (h *AuthHandler) GetPaymentMethod(c *gin.Context) {
	method, err := h.auth.GetPaymentMethod(c.Request.Context(), c.GetInt("user_id"))
	if err != nil {
		if errors.Is(err, repository.ErrPaymentMethodNotFound) {
			c.JSON(http.StatusOK, gin.H{"has_payment_method": false})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"has_payment_method": true,
		"provider":           method.Provider,
		"last4":              method.Last4,
	})
}

func (h *AuthHandler) CreatePaymentSetupSession(c *gin.Context) {
	stripeSecret := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))
	appBaseURL := strings.TrimRight(os.Getenv("APP_BASE_URL"), "/")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:5173"
	}

	if stripeSecret == "" {
		c.JSON(http.StatusOK, gin.H{
			"payment_setup_url": appBaseURL + "/payment/setup-success?session_id=mock_setup_session",
		})
		return
	}

	checkoutURL := strings.TrimSpace(os.Getenv("STRIPE_CHECKOUT_API_URL"))
	if checkoutURL == "" {
		checkoutURL = "https://api.stripe.com/v1/checkout/sessions"
	}

	userID := c.GetInt("user_id")
	values := url.Values{}
	values.Set("mode", "setup")
	values.Set("success_url", appBaseURL+"/payment/setup-success?session_id={CHECKOUT_SESSION_ID}")
	values.Set("cancel_url", appBaseURL+"/payment/setup-cancel")
	values.Set("client_reference_id", strconv.Itoa(userID))
	values.Set("currency", strings.ToLower(paymentCurrency()))
	values.Set("metadata[user_id]", strconv.Itoa(userID))

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, checkoutURL, strings.NewReader(values.Encode()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(stripeSecret, "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		c.JSON(resp.StatusCode, gin.H{"error": stripeMessage(body)})
		return
	}

	var session struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &session); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if session.URL == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "stripe setup session response has empty url"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"payment_setup_url": session.URL})
}

func (h *AuthHandler) ConfirmPaymentSetupSession(c *gin.Context) {
	var reqBody struct {
		SessionID string `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	sessionID := strings.TrimSpace(reqBody.SessionID)
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	last4 := "4242"
	stripeSecret := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))
	if stripeSecret != "" && !strings.HasPrefix(sessionID, "mock_") {
		resolvedLast4, err := fetchStripeSetupLast4(c.Request.Context(), stripeSecret, sessionID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		last4 = resolvedLast4
	}

	method, err := h.auth.UpsertPaymentMethod(c.Request.Context(), c.GetInt("user_id"), "stripe", last4)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_payment_method": true,
		"provider":           method.Provider,
		"last4":              method.Last4,
	})
}

func fetchStripeSetupLast4(ctx context.Context, stripeSecret string, sessionID string) (string, error) {
	apiURL := "https://api.stripe.com/v1/checkout/sessions/" + url.PathEscape(sessionID) + "?expand[]=setup_intent.payment_method"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(stripeSecret, "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("stripe setup session lookup failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var session struct {
		SetupIntent struct {
			PaymentMethod struct {
				Card struct {
					Last4 string `json:"last4"`
				} `json:"card"`
			} `json:"payment_method"`
		} `json:"setup_intent"`
	}
	if err := json.Unmarshal(body, &session); err != nil {
		return "", err
	}

	last4 := strings.TrimSpace(session.SetupIntent.PaymentMethod.Card.Last4)
	if len(last4) != 4 {
		return "", errors.New("stripe setup session does not contain card last4")
	}
	return last4, nil
}

func stripeMessage(body []byte) string {
	var stripeErr struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &stripeErr); err == nil && stripeErr.Error.Message != "" {
		return stripeErr.Error.Message
	}
	return strings.TrimSpace(string(body))
}

func paymentCurrency() string {
	currency := strings.TrimSpace(os.Getenv("PAYMENT_CURRENCY"))
	if currency == "" {
		return "kzt"
	}
	return currency
}

func (h *AuthHandler) HasPaymentMethod(c *gin.Context) {
	hasPaymentMethod, err := h.auth.HasPaymentMethod(c.Request.Context(), c.GetInt("user_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"has_payment_method": hasPaymentMethod})
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
func (h *AuthHandler) GetCustomerProfile(c *gin.Context) {
	userID := c.GetInt("user_id")
	if userID == 0 {
		userIDStr := c.Query("user_id")
		if userIDStr == "" {
			c.JSON(400, gin.H{"error": "missing user_id"})
			return
		}

		var err error
		userID, err = strconv.Atoi(userIDStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid user_id"})
			return
		}
	}

	profile, err := h.auth.GetCustomerProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(404, gin.H{"error": "customer profile not found"})
		return
	}

	c.JSON(200, gin.H{
		"customer_profile_id": profile.ID,
		"user_id":             profile.UserID,
		"address":             profile.Address,
		"latitude":            profile.Latitude,
		"longitude":           profile.Longitude,
		"bio":                 profile.Bio,
		"profile_photo_url":   profile.ProfilePhotoURL,
	})
}
func (h *AuthHandler) GetWorkerProfile(c *gin.Context) {
	userID := c.GetInt("user_id")
	if userID == 0 {
		userIDStr := c.Query("user_id")
		if userIDStr == "" {
			c.JSON(400, gin.H{"error": "missing user_id"})
			return
		}

		var err error
		userID, err = strconv.Atoi(userIDStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid user_id"})
			return
		}
	}

	profile, err := h.auth.GetWorkerProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(404, gin.H{"error": "worker profile not found"})
		return
	}
	skills, err := h.auth.ListVerifiedWorkerSkills(c.Request.Context(), userID)
	if err != nil {
		skills = []repository.VerifiedWorkerSkill{}
	}

	c.JSON(200, gin.H{
		"worker_profile_id":   profile.ID,
		"user_id":             profile.UserID,
		"bio":                 profile.Bio,
		"rating":              profile.Rating,
		"verification_status": profile.VerificationStatus,
		"is_available":        profile.IsAvailable,
		"current_latitude":    profile.CurrentLatitude,
		"current_longitude":   profile.CurrentLongitude,
		"profile_photo_url":   profile.ProfilePhotoURL,
		"verified_skills":     skills,
	})
}

func (h *AuthHandler) GetCategories(c *gin.Context) {
	result, err := h.auth.GetCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *AuthHandler) AdminOverview(c *gin.Context) {
	if !isStaffRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only staff allowed"})
		return
	}

	result, err := h.auth.GetAdminOverview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *AuthHandler) AdminUsers(c *gin.Context) {
	if !isStaffRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only staff allowed"})
		return
	}

	users, err := h.auth.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *AuthHandler) AdminDeleteUser(c *gin.Context) {
	if !isAdminRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins allowed"})
		return
	}

	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}
	if userID == c.GetInt("user_id") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "admin cannot delete own account"})
		return
	}

	if err := h.auth.DeleteUser(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

func (h *AuthHandler) AdminActivateUser(c *gin.Context) {
	if !isStaffRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only staff allowed"})
		return
	}

	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	if err := h.auth.ActivateUser(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user activated"})
}

func (h *AuthHandler) AdminCreateAdmin(c *gin.Context) {
	if !isAdminRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins allowed"})
		return
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	user, err := h.auth.CreateAdmin(c.Request.Context(), service.RegisterInput{
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		Password: req.Password,
		Role:     model.RoleAdmin,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":   user.ID,
		"full_name": user.FullName,
		"email":     user.Email,
		"role":      user.Role,
		"status":    user.Status,
	})
}

func (h *AuthHandler) AdminCreateManager(c *gin.Context) {
	if !isAdminRole(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only admins allowed"})
		return
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	user, err := h.auth.CreateManager(c.Request.Context(), service.RegisterInput{
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		Password: req.Password,
		Role:     model.RoleManager,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":   user.ID,
		"full_name": user.FullName,
		"email":     user.Email,
		"role":      user.Role,
		"status":    user.Status,
	})
}
