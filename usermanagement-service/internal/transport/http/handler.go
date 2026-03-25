package transporthttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"diploma/usermanagement-service/internal/model"
	"diploma/usermanagement-service/internal/repository"
	"diploma/usermanagement-service/internal/service"
)

type AuthService interface {
	Register(ctx context.Context, input service.RegisterInput) (service.RegisterResult, error)
}

type Handler struct {
	auth AuthService
}

func NewHandler(auth AuthService) *Handler {
	return &Handler{auth: auth}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.handleHealth)
	mux.HandleFunc("POST /register", h.handleRegister)
	return mux
}

type registerRequest struct {
	FullName string  `json:"full_name"`
	Email    string  `json:"email"`
	Phone    *string `json:"phone"`
	Password string  `json:"password"`
	Role     string  `json:"role"`
}

type registerResponse struct {
	UserID      int       `json:"user_id"`
	FullName    string    `json:"full_name"`
	Email       string    `json:"email"`
	Phone       *string   `json:"phone,omitempty"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	result, err := h.auth.Register(r.Context(), service.RegisterInput{
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		Password: req.Password,
		Role:     model.Role(req.Role),
	})
	if err != nil {
		h.writeRegisterError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, registerResponse{
		UserID:      result.User.ID,
		FullName:    result.User.FullName,
		Email:       result.User.Email,
		Phone:       result.User.Phone,
		Role:        string(result.User.Role),
		Status:      string(result.User.Status),
		CreatedAt:   result.User.CreatedAt,
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   result.ExpiresAt,
	})
}

func (h *Handler) writeRegisterError(w http.ResponseWriter, err error) {
	if service.IsValidationError(err) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	switch {
	case errors.Is(err, repository.ErrEmailAlreadyExists):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "email already exists"})
	case errors.Is(err, repository.ErrPhoneAlreadyExists):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "phone already exists"})
	default:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
