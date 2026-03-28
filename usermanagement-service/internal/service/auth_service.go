package service

import (
	"context"
	"errors"
	"log"
	"net/mail"
	"strings"
	"time"

	"diploma/usermanagement-service/internal/auth"
	"diploma/usermanagement-service/internal/email"
	"diploma/usermanagement-service/internal/model"
	"diploma/usermanagement-service/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type TokenGenerator interface {
	GenerateAccessToken(user model.User) (string, time.Time, error)
	GenerateRefreshToken(user model.User) (string, time.Time, error)
	Parse(token string) (*auth.Claims, error)
}

type AuthService struct {
	users            *repository.UserRepository
	tokens           TokenGenerator
	verifications    *repository.EmailVerificationRepository
	emailSender      *email.Sender
	passwordResets   *repository.PasswordResetRepository
	customerProfiles *repository.CustomerProfileRepository
	workerProfiles   *repository.WorkerProfileRepository
	workerSkills     *repository.WorkerSkillRepository
	categories       *repository.CategoryRepository
	admin            *repository.AdminRepository
}

func NewAuthService(
	users *repository.UserRepository,
	tokens TokenGenerator,
	verifications *repository.EmailVerificationRepository,
	emailSender *email.Sender,
	passwordResets *repository.PasswordResetRepository,
	customerProfiles *repository.CustomerProfileRepository,
	workerProfiles *repository.WorkerProfileRepository,
	workerSkills *repository.WorkerSkillRepository,
	categories *repository.CategoryRepository,
	admin *repository.AdminRepository,
) *AuthService {
	return &AuthService{
		users:            users,
		tokens:           tokens,
		verifications:    verifications,
		emailSender:      emailSender,
		passwordResets:   passwordResets,
		customerProfiles: customerProfiles,
		workerProfiles:   workerProfiles,
		workerSkills:     workerSkills,
		categories:       categories,
		admin:            admin,
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
	User model.User
}
type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
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
		Status:       model.StatusInactive,
	})
	if err != nil {
		return RegisterResult{}, err
	}

	verifyToken, err := auth.GenerateVerificationToken()
	if err != nil {
		return RegisterResult{}, err
	}

	expiresAt := time.Now().Add(24 * time.Hour)

	err = s.verifications.Create(ctx, user.ID, verifyToken, expiresAt)
	if err != nil {
		_ = s.users.DeleteUser(ctx, user.ID)
		return RegisterResult{}, err
	}

	err = s.emailSender.SendVerificationEmail(user.Email, verifyToken)
	if err != nil {
		_ = s.users.DeleteUser(ctx, user.ID)

		return RegisterResult{}, err
	}
	return RegisterResult{
		User: user,
	}, nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	userID, expiresAt, err := s.verifications.GetByToken(ctx, token)
	if err != nil {
		return err
	}

	if time.Now().After(expiresAt) {
		return errors.New("token expired")
	}

	err = s.users.ActivateUser(ctx, userID)
	if err != nil {
		return err
	}

	return s.verifications.Delete(ctx, token)
}

func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}
func (s *AuthService) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return LoginResult{}, errors.New("invalid credentials")
	}

	if user.Status != model.StatusActive {
		return LoginResult{}, errors.New("email not verified")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return LoginResult{}, errors.New("invalid credentials")
	}

	accessToken, expiresAt, err := s.tokens.GenerateAccessToken(user)
	if err != nil {
		return LoginResult{}, err
	}

	refreshToken, _, err := s.tokens.GenerateRefreshToken(user)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (LoginResult, error) {
	claims, err := s.tokens.Parse(refreshToken)
	if err != nil {
		return LoginResult{}, errors.New("invalid refresh token")
	}

	userID := claims.UserID
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return LoginResult{}, err
	}

	token, expiresAt, err := s.tokens.GenerateAccessToken(user)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken: token,
		ExpiresAt:   expiresAt,
	}, nil
}
func (s *AuthService) ResendVerification(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return errors.New("user not found")
	}

	if user.Status == model.StatusActive {
		return errors.New("user already verified")
	}

	_ = s.verifications.DeleteByUserID(ctx, user.ID)

	verifyToken, err := auth.GenerateVerificationToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(24 * time.Hour)

	err = s.verifications.Create(ctx, user.ID, verifyToken, expiresAt)
	if err != nil {
		return err
	}

	return s.emailSender.SendVerificationEmail(user.Email, verifyToken)
}
func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil
	}

	token, _ := auth.GenerateVerificationToken()
	expiresAt := time.Now().Add(1 * time.Hour)
	log.Println("FORGOT PASSWORD CALLED:", email)
	err = s.passwordResets.Create(ctx, user.ID, token, expiresAt)
	if err != nil {
		return err
	}
	log.Println("SENDING RESET EMAIL TO:", user.Email)
	err = s.emailSender.SendResetEmail(user.Email, token)
	if err != nil {
		log.Println("EMAIL ERROR:", err)
		return err
	}
	return nil
}
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	userID, expiresAt, err := s.passwordResets.GetByToken(ctx, token)
	if err != nil {
		return err
	}

	if time.Now().After(expiresAt) {
		return errors.New("token expired")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)

	err = s.users.UpdatePassword(ctx, userID, string(hash))
	if err != nil {
		return err
	}

	return s.passwordResets.Delete(ctx, token)
}
func (s *AuthService) SelectRole(ctx context.Context, userID int, role model.Role) error {
	if role != model.RoleCustomer && role != model.RoleWorker {
		return errors.New("invalid role")
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.Role != "" {
		return errors.New("role already selected")
	}
	return s.users.UpdateRole(ctx, userID, role)
}
func (s *AuthService) CreateCustomerProfile(ctx context.Context, userID int) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.Role != model.RoleCustomer {
		return errors.New("not a customer")
	}

	return s.customerProfiles.Create(ctx, userID)
}
func (s *AuthService) CreateWorkerProfile(ctx context.Context, userID int) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.Role != model.RoleWorker {
		return errors.New("not a worker")
	}

	return s.workerProfiles.Create(ctx, userID)
}
func (s *AuthService) AddWorkerSkill(
	ctx context.Context,
	userID int,
	categoryID int,
	experience string,
	price int,
) error {
	if !isValidExperience(experience) {
		return errors.New("invalid experience level")
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.Role != model.RoleWorker {
		return errors.New("not a worker")
	}

	workerID, err := s.workerProfiles.GetByUserID(ctx, userID)
	if err != nil {
		return errors.New("worker profile not found")
	}

	return s.workerSkills.Create(ctx, workerID, categoryID, experience, price)
}
func (s *AuthService) VerifyWorkerSkill(ctx context.Context, skillID int) error {
	return s.workerSkills.Verify(ctx, skillID)
}
func (s *AuthService) SetAvailability(ctx context.Context, userID int, available bool) error {
	worker, err := s.workerProfiles.GetByUserIDFull(ctx, userID)
	if err != nil {
		return err
	}

	if worker.VerificationStatus != "verified" {
		return errors.New("worker not verified")
	}

	return s.workerProfiles.UpdateAvailability(ctx, worker.ID, available)
}
func isValidExperience(level string) bool {
	switch level {
	case "junior", "middle", "senior":
		return true
	default:
		return false
	}
}
func (s *AuthService) FindWorkers(
	ctx context.Context,
	categoryID int,
) ([]repository.WorkerSearchResult, error) {
	return s.workerSkills.FindWorkersByCategory(ctx, categoryID)
}
func (s *AuthService) VerifyWorker(ctx context.Context, workerID int) error {
	return s.workerProfiles.Verify(ctx, workerID)
}
func (s *AuthService) GetCustomerProfile(
	ctx context.Context,
	userID int,
) (*model.CustomerProfile, error) {

	return s.customerProfiles.GetByUserID(ctx, userID)
}
func (s *AuthService) GetWorkerProfile(
	ctx context.Context,
	userID int,
) (*model.WorkerProfile, error) {

	profile, err := s.workerProfiles.GetByUserIDFull(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &profile, nil
}

func (s *AuthService) GetCategories(ctx context.Context) ([]repository.ServiceCategory, error) {
	return s.categories.List(ctx)
}

func (s *AuthService) GetAdminOverview(ctx context.Context) (repository.AdminOverview, error) {
	return s.admin.GetOverview(ctx)
}
