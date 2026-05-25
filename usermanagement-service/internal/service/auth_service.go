package service

import (
	"context"
	"errors"
	"log"
	"net/mail"
	"os"
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
	ParseRefreshToken(token string) (*auth.Claims, error)
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
	paymentMethods   *repository.PaymentMethodRepository
	reports          *repository.ReportRepository
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
	paymentMethods *repository.PaymentMethodRepository,
	reports *repository.ReportRepository,
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
		paymentMethods:   paymentMethods,
		reports:          reports,
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
			if !strings.HasPrefix(trimmed, "+") {
				return RegisterResult{}, &ValidationError{Field: "phone", Message: "must start with +"}
			}
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
		log.Printf("warning: failed to send verification email to %s: %v", user.Email, err)
		if isEmailRequired() {
			_ = s.users.DeleteUser(ctx, user.ID)
			return RegisterResult{}, err
		}
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

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	switch user.Role {
	case model.RoleCustomer:
		if err := s.customerProfiles.Create(ctx, userID); err != nil {
			return err
		}
	case model.RoleWorker:
		if err := s.workerProfiles.Create(ctx, userID); err != nil {
			return err
		}
	}

	return s.verifications.Delete(ctx, token)
}

func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}

func isEmailRequired() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_REQUIRED")))
	switch v {
	case "0", "false", "no":
		return false
	default:
		return true
	}
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
	claims, err := s.tokens.ParseRefreshToken(refreshToken)
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
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 8 {
		return &ValidationError{Field: "new_password", Message: "must be at least 8 characters"}
	}

	userID, expiresAt, err := s.passwordResets.GetByToken(ctx, token)
	if err != nil {
		return err
	}

	if time.Now().After(expiresAt) {
		return errors.New("token expired")
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(newPassword)); err == nil {
		return &ValidationError{Field: "new_password", Message: "must be different from old password"}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

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

func (s *AuthService) UpsertCustomerProfile(
	ctx context.Context,
	userID int,
	address string,
	latitude *float64,
	longitude *float64,
	bio string,
	profilePhotoURL *string,
) (*model.CustomerProfile, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Role != model.RoleCustomer {
		return nil, errors.New("not a customer")
	}
	return s.customerProfiles.Upsert(ctx, userID, address, latitude, longitude, bio, profilePhotoURL)
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

func (s *AuthService) UpsertWorkerProfile(
	ctx context.Context,
	userID int,
	bio string,
	latitude *float64,
	longitude *float64,
	profilePhotoURL *string,
) (*model.WorkerProfile, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Role != model.RoleWorker {
		return nil, errors.New("not a worker")
	}

	profile, err := s.workerProfiles.UpsertDetails(ctx, userID, bio, latitude, longitude, profilePhotoURL)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}
func (s *AuthService) AddWorkerSkill(
	ctx context.Context,
	userID int,
	categoryID int,
	experience string,
	price int,
) (int, error) {
	if !isValidExperience(experience) {
		return 0, errors.New("invalid experience level")
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	if user.Role != model.RoleWorker {
		return 0, errors.New("not a worker")
	}

	workerID, err := s.workerProfiles.GetByUserID(ctx, userID)
	if err != nil {
		return 0, errors.New("worker profile not found")
	}

	return s.workerSkills.Create(ctx, workerID, categoryID, experience, price)
}

func (s *AuthService) AddWorkerSkillEvidence(
	ctx context.Context,
	workerSkillID int,
	fileName string,
	filePath string,
	contentType string,
	note string,
) error {
	return s.workerSkills.AddEvidence(ctx, workerSkillID, fileName, filePath, contentType, note)
}

func (s *AuthService) RequestWorkerSkillUpgrade(
	ctx context.Context,
	userID int,
	workerSkillID int,
	requestedLevel string,
	evidenceFiles string,
	note string,
) (int, error) {
	if !isValidExperience(requestedLevel) {
		return 0, errors.New("invalid experience level")
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	if user.Role != model.RoleWorker {
		return 0, errors.New("not a worker")
	}
	return s.workerSkills.CreateUpgradeRequest(ctx, userID, workerSkillID, requestedLevel, evidenceFiles, note)
}

func (s *AuthService) VerifyWorkerSkill(ctx context.Context, skillID int) error {
	return s.workerSkills.Verify(ctx, skillID)
}

func (s *AuthService) VerifyWorkerSkillUpgrade(ctx context.Context, requestID int, reviewerUserID int) error {
	return s.workerSkills.ApproveUpgradeRequest(ctx, requestID, reviewerUserID)
}

func (s *AuthService) AddWorkerIdentityDocument(
	ctx context.Context,
	userID int,
	fileName string,
	filePath string,
	contentType string,
) (repository.WorkerIdentityDocument, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return repository.WorkerIdentityDocument{}, err
	}
	if user.Role != model.RoleWorker {
		return repository.WorkerIdentityDocument{}, errors.New("not a worker")
	}
	if _, err := s.workerProfiles.GetByUserID(ctx, userID); err != nil {
		if createErr := s.workerProfiles.Create(ctx, userID); createErr != nil {
			return repository.WorkerIdentityDocument{}, createErr
		}
	}
	return s.workerProfiles.AddIdentityDocument(ctx, userID, fileName, filePath, contentType)
}

func (s *AuthService) VerifyWorkerIdentityDocument(ctx context.Context, documentID int, reviewerUserID int) error {
	return s.workerProfiles.VerifyIdentityDocument(ctx, documentID, reviewerUserID)
}

func (s *AuthService) RejectWorkerIdentityDocument(ctx context.Context, documentID int, reviewerUserID int, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Document was rejected. Upload a clear ID card or passport."
	}
	return s.workerProfiles.RejectIdentityDocument(ctx, documentID, reviewerUserID, reason)
}

func (s *AuthService) AssignWorkerIdentityDocument(ctx context.Context, documentID int, managerUserID int) error {
	return s.workerProfiles.AssignIdentityDocument(ctx, documentID, managerUserID)
}

func (s *AuthService) GetWorkerIdentityDocument(ctx context.Context, userID int) (*repository.WorkerIdentityDocument, error) {
	return s.workerProfiles.GetLatestIdentityDocumentByUserID(ctx, userID)
}

func (s *AuthService) SetAvailability(ctx context.Context, userID int, available bool) error {
	worker, err := s.workerProfiles.GetByUserIDFull(ctx, userID)
	if err != nil {
		return err
	}

	if available {
		readiness, err := s.workerProfiles.GetReadiness(ctx, worker.ID)
		if err != nil {
			return err
		}
		if !readiness.IdentityVerified {
			return errors.New("identity document is not verified")
		}
		if !readiness.HasVerifiedSkill {
			return errors.New("verified skill is required")
		}
		if worker.VerificationStatus != "verified" {
			if err := s.workerProfiles.RecalculateVerificationStatus(ctx, worker.ID); err != nil {
				return err
			}
			worker, err = s.workerProfiles.GetByUserIDFull(ctx, userID)
			if err != nil {
				return err
			}
			if worker.VerificationStatus != "verified" {
				return errors.New("worker not verified")
			}
		}

		blocked, err := s.reports.HasActiveOnlineBlock(ctx, userID)
		if err != nil {
			return err
		}
		if blocked {
			return errors.New("worker is suspended by support")
		}

		hasPaymentMethod, err := s.paymentMethods.Exists(ctx, userID)
		if err != nil {
			return err
		}
		if !hasPaymentMethod {
			return errors.New("payment method is required")
		}

		hasActiveJob, err := s.workerProfiles.HasInProgressBooking(ctx, worker.ID)
		if err != nil {
			return err
		}
		if hasActiveJob {
			return errors.New("worker has a job in progress")
		}
	}

	return s.workerProfiles.UpdateAvailability(ctx, worker.ID, available)
}

func (s *AuthService) UpsertPaymentMethod(ctx context.Context, userID int, provider string, last4 string) (repository.PaymentMethod, error) {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "stripe"
	}
	last4 = strings.TrimSpace(last4)
	if len(last4) != 4 {
		return repository.PaymentMethod{}, errors.New("card last4 must contain 4 digits")
	}
	for _, char := range last4 {
		if char < '0' || char > '9' {
			return repository.PaymentMethod{}, errors.New("card last4 must contain only digits")
		}
	}
	return s.paymentMethods.Upsert(ctx, userID, provider, last4)
}

func (s *AuthService) GetPaymentMethod(ctx context.Context, userID int) (repository.PaymentMethod, error) {
	return s.paymentMethods.Get(ctx, userID)
}

func (s *AuthService) ListPaymentMethods(ctx context.Context, userID int) ([]repository.PaymentMethod, error) {
	return s.paymentMethods.List(ctx, userID)
}

func (s *AuthService) SelectPaymentMethod(ctx context.Context, userID int, paymentMethodID int) (repository.PaymentMethod, error) {
	if paymentMethodID <= 0 {
		return repository.PaymentMethod{}, errors.New("payment_method_id must be positive")
	}
	return s.paymentMethods.SetActive(ctx, userID, paymentMethodID)
}

func (s *AuthService) HasPaymentMethod(ctx context.Context, userID int) (bool, error) {
	return s.paymentMethods.Exists(ctx, userID)
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

	profile, err := s.customerProfiles.GetByUserID(ctx, userID)
	if err == nil {
		return profile, nil
	}

	user, userErr := s.users.GetByID(ctx, userID)
	if userErr != nil || user.Role != model.RoleCustomer {
		return nil, err
	}
	if createErr := s.customerProfiles.Create(ctx, userID); createErr != nil {
		return nil, createErr
	}
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

func (s *AuthService) ListVerifiedWorkerSkills(
	ctx context.Context,
	userID int,
) ([]repository.VerifiedWorkerSkill, error) {
	profile, err := s.workerProfiles.GetByUserIDFull(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.workerProfiles.ListVerifiedSkills(ctx, profile.ID)
}

func (s *AuthService) GetCategories(ctx context.Context) ([]repository.ServiceCategory, error) {
	return s.categories.List(ctx)
}

func (s *AuthService) GetAdminOverview(ctx context.Context, userID int, role string) (repository.AdminOverview, error) {
	return s.admin.GetOverview(ctx, userID, role)
}

func (s *AuthService) ListUsers(ctx context.Context) ([]repository.UserSummary, error) {
	return s.users.ListUsers(ctx)
}

func (s *AuthService) GetStaffUserProfile(ctx context.Context, userID int) (repository.StaffUserProfile, error) {
	profile, err := s.users.GetStaffProfile(ctx, userID)
	if err != nil {
		return repository.StaffUserProfile{}, err
	}
	if profile.User.Role == model.RoleWorker {
		skills, err := s.ListVerifiedWorkerSkills(ctx, userID)
		if err == nil {
			profile.VerifiedSkills = skills
		}
	}
	return profile, nil
}

func (s *AuthService) EnsureDefaultAdmin(ctx context.Context) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.users.EnsureDefaultAdmin(ctx, "sa@sa.sa", string(passwordHash))
}

func (s *AuthService) DeleteUser(ctx context.Context, userID int) error {
	return s.users.DeleteUser(ctx, userID)
}

func (s *AuthService) ActivateUser(ctx context.Context, userID int) error {
	return s.users.ActivateUser(ctx, userID)
}

func (s *AuthService) CreateAdmin(ctx context.Context, input RegisterInput) (model.User, error) {
	return s.createPrivilegedUser(ctx, input, model.RoleAdmin)
}

func (s *AuthService) CreateManager(ctx context.Context, input RegisterInput) (model.User, error) {
	return s.createPrivilegedUser(ctx, input, model.RoleManager)
}

func (s *AuthService) CreateReport(ctx context.Context, reporterUserID int, role string, bookingID *int, reportedUserID int, reason string, description string) (repository.Report, error) {
	reason = strings.TrimSpace(reason)
	description = strings.TrimSpace(description)
	if reason == "" {
		return repository.Report{}, errors.New("reason is required")
	}
	if bookingID != nil {
		counterpartyID, err := s.reports.BookingCounterparty(ctx, *bookingID, reporterUserID)
		if err != nil {
			return repository.Report{}, err
		}
		reportedUserID = counterpartyID
	}
	if reportedUserID <= 0 || reportedUserID == reporterUserID {
		return repository.Report{}, errors.New("reported user is required")
	}
	if role != string(model.RoleCustomer) && role != string(model.RoleWorker) {
		return repository.Report{}, errors.New("only customers and workers can create reports")
	}
	report, err := s.reports.Create(ctx, bookingID, reporterUserID, reportedUserID, reason, description)
	if err != nil {
		return repository.Report{}, err
	}
	if err := s.reports.NotifyStaffReportCreated(ctx, report); err != nil {
		log.Printf("report notification skipped for report %d: %v", report.ReportID, err)
	}
	return report, nil
}

func (s *AuthService) AddReportFile(ctx context.Context, reportID int, userID int, fileName string, filePath string, contentType string) error {
	return s.reports.AddFile(ctx, reportID, userID, fileName, filePath, contentType)
}

func (s *AuthService) ListReports(ctx context.Context, userID int, staff bool, role string) ([]repository.Report, error) {
	return s.reports.List(ctx, userID, staff, role)
}

func (s *AuthService) AddReportMessage(ctx context.Context, reportID int, userID int, staff bool, conversationSide string, text string, attachmentURL string, attachmentName string, attachmentType string) (repository.ReportMessage, error) {
	if strings.TrimSpace(text) == "" && strings.TrimSpace(attachmentURL) == "" {
		return repository.ReportMessage{}, errors.New("message or attachment is required")
	}
	msg, err := s.reports.AddMessage(ctx, reportID, userID, staff, conversationSide, text, attachmentURL, attachmentName, attachmentType)
	if err != nil {
		return repository.ReportMessage{}, err
	}
	if err := s.reports.NotifyReportMessage(ctx, reportID, userID, staff, msg.ConversationSide); err != nil {
		log.Printf("report message notification skipped for report %d: %v", reportID, err)
	}
	return msg, nil
}

func (s *AuthService) ListReportMessages(ctx context.Context, reportID int, userID int, staff bool, conversationSide string) ([]repository.ReportMessage, error) {
	messages, err := s.reports.ListMessages(ctx, reportID, userID, staff, conversationSide)
	if err != nil {
		return nil, err
	}
	_ = s.reports.MarkMessagesRead(ctx, reportID, userID, staff, conversationSide)
	return messages, nil
}

func (s *AuthService) ApplyReportPenalty(ctx context.Context, reportID int, targetUserID int, issuedByUserID int, penaltyType string, reason string, expiresAt *time.Time) (repository.Penalty, error) {
	penaltyType = strings.TrimSpace(penaltyType)
	reason = strings.TrimSpace(reason)
	if reportID <= 0 || targetUserID <= 0 || issuedByUserID <= 0 {
		return repository.Penalty{}, errors.New("invalid penalty target")
	}
	if reason == "" {
		reason = penaltyType
	}
	return s.reports.ApplyPenalty(ctx, reportID, targetUserID, issuedByUserID, penaltyType, reason, expiresAt)
}

func (s *AuthService) CancelPenalty(ctx context.Context, penaltyID int) error {
	if penaltyID <= 0 {
		return errors.New("invalid penalty_id")
	}
	return s.reports.CancelPenalty(ctx, penaltyID)
}

func (s *AuthService) CloseReport(ctx context.Context, reportID int, status string, resolution string) error {
	status = strings.TrimSpace(status)
	if status != "resolved" && status != "rejected" {
		return errors.New("invalid report status")
	}
	return s.reports.CloseReport(ctx, reportID, status, strings.TrimSpace(resolution))
}

func (s *AuthService) AssignReport(ctx context.Context, reportID int, managerUserID int) (repository.Report, error) {
	if reportID <= 0 || managerUserID <= 0 {
		return repository.Report{}, errors.New("invalid report assignment")
	}
	return s.reports.Assign(ctx, reportID, managerUserID)
}

func (s *AuthService) createPrivilegedUser(ctx context.Context, input RegisterInput, role model.Role) (model.User, error) {
	fullName := strings.TrimSpace(input.FullName)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)

	if fullName == "" {
		return model.User{}, &ValidationError{Field: "full_name", Message: "is required"}
	}
	if email == "" {
		return model.User{}, &ValidationError{Field: "email", Message: "is required"}
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return model.User{}, &ValidationError{Field: "email", Message: "must be a valid email address"}
	}
	if len(password) < 8 {
		return model.User{}, &ValidationError{Field: "password", Message: "must be at least 8 characters"}
	}

	var phone *string
	if input.Phone != nil {
		trimmed := strings.TrimSpace(*input.Phone)
		if trimmed != "" {
			if !strings.HasPrefix(trimmed, "+") {
				return model.User{}, &ValidationError{Field: "phone", Message: "must start with +"}
			}
			phone = &trimmed
		}
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, err
	}

	return s.users.CreateUser(ctx, repository.CreateUserParams{
		FullName:     fullName,
		Email:        email,
		Phone:        phone,
		PasswordHash: string(passwordHash),
		Role:         role,
		Status:       model.StatusActive,
	})
}
