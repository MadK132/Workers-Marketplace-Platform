package service

import (
	"context"
	"errors"

	"diploma/booking-service/internal/repository"
)

var (
	ErrRequestNotFound       = errors.New("request not found")
	ErrRequestNotOwned       = errors.New("request does not belong to customer")
	ErrRequestUnavailable    = errors.New("request already assigned")
	ErrWorkerNotSelectable   = errors.New("selected worker is not available for this category")
	ErrBookingNotFound       = errors.New("booking not found")
	ErrBookingNotOwned       = errors.New("booking does not belong to worker")
	ErrBookingNotCustomer    = errors.New("booking does not belong to customer")
	ErrInvalidTransition     = errors.New("invalid booking status transition")
	ErrEvidenceRequired      = errors.New("completion evidence is required")
	ErrPaymentMethodRequired = errors.New("payment method is required")
	ErrPaymentAmountInvalid  = errors.New("payment amount is invalid")
	ErrReviewInvalidRating   = errors.New("review rating must be from 1 to 5")
	ErrPriceInvalid          = errors.New("booking price must be positive")
)

type BookingService struct {
	repo *repository.BookingRepository
}

func NewBookingService(repo *repository.BookingRepository) *BookingService {
	return &BookingService{repo: repo}
}

func (s *BookingService) CreateBooking(
	ctx context.Context,
	requestID int,
	workerProfileID int,
	customerProfileID int,
) (int, error) {
	if err := s.repo.ExpirePendingOffers(ctx); err != nil {
		return 0, err
	}

	req, err := s.repo.GetRequestForBooking(ctx, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) {
			return 0, ErrRequestNotFound
		}
		return 0, err
	}

	if req.CustomerProfileID != customerProfileID {
		return 0, ErrRequestNotOwned
	}

	if req.Status != "pending" {
		return 0, ErrRequestUnavailable
	}

	ok, err := s.repo.IsWorkerEligible(ctx, workerProfileID, req.CategoryID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrWorkerNotSelectable
	}

	return s.repo.Create(ctx, requestID, workerProfileID)
}

func (s *BookingService) StartBooking(
	ctx context.Context,
	bookingID int,
	workerProfileID int,
) error {
	if err := s.repo.ExpirePendingOffers(ctx); err != nil {
		return err
	}

	b, err := s.repo.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return ErrBookingNotFound
		}
		return err
	}

	if b.WorkerProfileID != workerProfileID {
		return ErrBookingNotOwned
	}

	if b.Status != "scheduled" {
		return ErrInvalidTransition
	}

	return s.repo.MarkInProgress(ctx, bookingID, b.RequestID)
}

func (s *BookingService) RejectBooking(
	ctx context.Context,
	bookingID int,
	workerProfileID int,
) error {
	if err := s.repo.ExpirePendingOffers(ctx); err != nil {
		return err
	}

	b, err := s.repo.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return ErrBookingNotFound
		}
		return err
	}

	if b.WorkerProfileID != workerProfileID {
		return ErrBookingNotOwned
	}
	if b.Status != "scheduled" {
		return ErrInvalidTransition
	}

	return s.repo.MarkRejected(ctx, bookingID, b.RequestID, b.WorkerProfileID)
}

func (s *BookingService) CompleteBooking(
	ctx context.Context,
	bookingID int,
	workerProfileID int,
	evidence string,
) error {
	b, err := s.repo.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return ErrBookingNotFound
		}
		return err
	}

	if b.WorkerProfileID != workerProfileID {
		return ErrBookingNotOwned
	}

	if b.Status != "in_progress" {
		return ErrInvalidTransition
	}
	if evidence == "" {
		return ErrEvidenceRequired
	}

	return s.repo.MarkAwaitingConfirmation(ctx, bookingID, b.RequestID, evidence)
}

func (s *BookingService) ConfirmCompletion(
	ctx context.Context,
	bookingID int,
	customerProfileID int,
) (float64, error) {
	b, err := s.repo.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return 0, ErrBookingNotFound
		}
		return 0, err
	}

	req, err := s.repo.GetRequestForBooking(ctx, b.RequestID)
	if err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) {
			return 0, ErrRequestNotFound
		}
		return 0, err
	}
	if req.CustomerProfileID != customerProfileID {
		return 0, ErrBookingNotCustomer
	}
	if b.Status != "awaiting_confirmation" {
		return 0, ErrInvalidTransition
	}

	paymentData, err := s.repo.GetPaymentData(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return 0, ErrBookingNotFound
		}
		return 0, err
	}
	if paymentData.Amount <= 0 {
		return 0, ErrPaymentAmountInvalid
	}

	return paymentData.Amount, nil
}

func (s *BookingService) MarkCompletionPaid(
	ctx context.Context,
	bookingID int,
	customerProfileID int,
) error {
	b, err := s.repo.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return ErrBookingNotFound
		}
		return err
	}

	req, err := s.repo.GetRequestForBooking(ctx, b.RequestID)
	if err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	if req.CustomerProfileID != customerProfileID {
		return ErrBookingNotCustomer
	}
	if b.Status != "awaiting_confirmation" {
		return ErrInvalidTransition
	}

	return s.repo.MarkCompleted(ctx, bookingID, b.RequestID, b.WorkerProfileID)
}

func (s *BookingService) SetFinalPriceForCustomer(
	ctx context.Context,
	bookingID int,
	customerProfileID int,
	amount float64,
) error {
	if amount <= 0 {
		return ErrPriceInvalid
	}
	b, err := s.repo.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return ErrBookingNotFound
		}
		return err
	}
	req, err := s.repo.GetRequestForBooking(ctx, b.RequestID)
	if err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	if req.CustomerProfileID != customerProfileID {
		return ErrBookingNotCustomer
	}
	if b.Status != "price_pending" && b.Status != "scheduled" && b.Status != "in_progress" {
		return ErrInvalidTransition
	}
	return s.repo.SetFinalPrice(ctx, bookingID, amount)
}

func (s *BookingService) SetFinalPriceForWorker(
	ctx context.Context,
	bookingID int,
	workerProfileID int,
	amount float64,
) error {
	if amount <= 0 {
		return ErrPriceInvalid
	}
	b, err := s.repo.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return ErrBookingNotFound
		}
		return err
	}
	if b.WorkerProfileID != workerProfileID {
		return ErrBookingNotOwned
	}
	if b.Status != "price_pending" && b.Status != "scheduled" && b.Status != "in_progress" {
		return ErrInvalidTransition
	}
	return s.repo.SetFinalPrice(ctx, bookingID, amount)
}

func (s *BookingService) AcceptBookingPrice(
	ctx context.Context,
	bookingID int,
	customerProfileID int,
) error {
	b, err := s.repo.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return ErrBookingNotFound
		}
		return err
	}
	req, err := s.repo.GetRequestForBooking(ctx, b.RequestID)
	if err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	if req.CustomerProfileID != customerProfileID {
		return ErrBookingNotCustomer
	}
	if b.Status != "price_pending" {
		return ErrInvalidTransition
	}
	if b.FinalPrice == nil || *b.FinalPrice <= 0 {
		return ErrPaymentAmountInvalid
	}
	return s.repo.MarkPriceAccepted(ctx, bookingID, b.RequestID)
}

func (s *BookingService) RejectBookingPrice(
	ctx context.Context,
	bookingID int,
	customerProfileID int,
) error {
	b, err := s.repo.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return ErrBookingNotFound
		}
		return err
	}
	req, err := s.repo.GetRequestForBooking(ctx, b.RequestID)
	if err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) {
			return ErrRequestNotFound
		}
		return err
	}
	if req.CustomerProfileID != customerProfileID {
		return ErrBookingNotCustomer
	}
	if b.Status != "price_pending" {
		return ErrInvalidTransition
	}
	return s.repo.MarkPriceRejected(ctx, bookingID)
}

func (s *BookingService) ListCustomerBookings(
	ctx context.Context,
	customerProfileID int,
) ([]repository.BookingListItem, error) {
	if err := s.repo.ExpirePendingOffers(ctx); err != nil {
		return nil, err
	}
	return s.repo.ListByCustomerProfile(ctx, customerProfileID)
}

func (s *BookingService) ListWorkerBookings(
	ctx context.Context,
	workerProfileID int,
) ([]repository.BookingListItem, error) {
	if err := s.repo.ExpirePendingOffers(ctx); err != nil {
		return nil, err
	}
	return s.repo.ListByWorkerProfile(ctx, workerProfileID)
}

func (s *BookingService) CreateReview(
	ctx context.Context,
	bookingID int,
	customerProfileID int,
	rating int,
	comment string,
) (int, error) {
	if rating < 1 || rating > 5 {
		return 0, ErrReviewInvalidRating
	}

	b, err := s.repo.GetBookingDetails(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrBookingNotFound) {
			return 0, ErrBookingNotFound
		}
		return 0, err
	}
	if b.Status != "completed" {
		return 0, ErrInvalidTransition
	}

	req, err := s.repo.GetRequestForBooking(ctx, b.RequestID)
	if err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) {
			return 0, ErrRequestNotFound
		}
		return 0, err
	}
	if req.CustomerProfileID != customerProfileID {
		return 0, ErrBookingNotCustomer
	}

	return s.repo.SaveReview(ctx, bookingID, customerProfileID, b.WorkerProfileID, rating, comment)
}

func (s *BookingService) ListWorkerReviews(
	ctx context.Context,
	workerProfileID int,
) (repository.WorkerReviewSummary, error) {
	return s.repo.ListWorkerReviews(ctx, workerProfileID)
}

func (s *BookingService) BookingUsers(ctx context.Context, bookingID int) (repository.BookingUsers, error) {
	return s.repo.GetBookingUsers(ctx, bookingID)
}
