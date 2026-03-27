package service

import (
	"context"
	"errors"

	"diploma/booking-service/internal/repository"
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
) error {

	ok, err := s.repo.IsRequestAvailable(ctx, requestID)
	if err != nil {
		return err
	}

	if !ok {
		return errors.New("request already assigned")
	}

	return s.repo.Create(ctx, requestID, workerProfileID)
}
