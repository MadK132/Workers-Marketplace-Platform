package service

import (
	"context"
	"errors"

	"diploma/booking-service/internal/repository"
)

var (
	ErrRequestNotFound     = errors.New("request not found")
	ErrRequestNotOwned     = errors.New("request does not belong to customer")
	ErrRequestUnavailable  = errors.New("request already assigned")
	ErrWorkerNotSelectable = errors.New("selected worker is not available for this category")
	ErrBookingNotFound     = errors.New("booking not found")
	ErrBookingNotOwned     = errors.New("booking does not belong to worker")
	ErrInvalidTransition   = errors.New("invalid booking status transition")
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

func (s *BookingService) CompleteBooking(
	ctx context.Context,
	bookingID int,
	workerProfileID int,
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

	return s.repo.MarkCompleted(ctx, bookingID, b.RequestID, b.WorkerProfileID)
}
