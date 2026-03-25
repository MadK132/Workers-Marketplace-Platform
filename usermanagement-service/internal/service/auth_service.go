package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"diploma/usermanagement-service/internal/model"
	"diploma/usermanagement-service/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type TokenGenerator interface {
	GenerateAccessToken(user model.User) (string, time.Time, error)
}

type AuthService struct {
	users  *repository.UserRepository
	tokens TokenGenerator
}

func NewAuthService(users *repository.UserRepository, tokens TokenGenerator) *AuthService {
	return &AuthService{
		users:  users,
		tokens: tokens,
	}
}

type RegisterInput struct {
	FullName string
	Email    string
	Phone    *string
	Password string
	Role     model.Role
}

type RegisterResult struct {
	User        model.User
	AccessToken string
	ExpiresAt   time.Time
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (RegisterResult, error) {
	fullName := strings.TrimSpace(input.FullName)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)
	role := model.Role(strings.ToLower(strings.TrimSpace(string(input.Role))))

	if fullName == "" {
		return RegisterResult{}, &ValidationError{Field: "full_name", Message: "is required"}
	}
	if len(fullName) > 255 {
		return RegisterResult{}, &ValidationError{Field: "full_name", Message: "must be at most 255 characters"}
	}
	if email == "" {
		return RegisterResult{}, &ValidationError{Field: "email", Message: "is required"}
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return RegisterResult{}, &ValidationError{Field: "email", Message: "must be a valid email address"}
	}
	if len(password) < 8 {
		return RegisterResult{}, &ValidationError{Field: "password", Message: "must be at least 8 characters"}
	}
	if !role.CanRegister() {
		return RegisterResult{}, &ValidationError{Field: "role", Message: "must be one of: customer, worker"}
	}

	var phone *string
	if input.Phone != nil {
		trimmed := strings.TrimSpace(*input.Phone)
		if trimmed != "" {
			phone = &trimmed
		}
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return RegisterResult{}, err
	}

	user, err := s.users.CreateUser(ctx, repository.CreateUserParams{
		FullName:     fullName,
		Email:        email,
		Phone:        phone,
		PasswordHash: string(passwordHash),
		Role:         role,
	})
	if err != nil {
		return RegisterResult{}, err
	}

	accessToken, expiresAt, err := s.tokens.GenerateAccessToken(user)
	if err != nil {
		return RegisterResult{}, err
	}

	return RegisterResult{
		User:        user,
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
	}, nil
}

func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}
