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
) error {
	if err := s.repo.ExpirePendingOffers(ctx); err != nil {
		return err
	}

	req, err := s.repo.GetRequestForBooking(ctx, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrRequestNotFound) {
			return ErrRequestNotFound
		}
		return err
	}

	if req.CustomerProfileID != customerProfileID {
		return ErrRequestNotOwned
	}

	if req.Status != "pending" {
		return ErrRequestUnavailable
	}

	ok, err := s.repo.IsWorkerEligible(ctx, workerProfileID, req.CategoryID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrWorkerNotSelectable
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
