package service

import (
	"context"
	"errors"

	"diploma/booking-service/internal/repository"
)

const (
	astanaMinLatitude  = 50.95
	astanaMaxLatitude  = 51.35
	astanaMinLongitude = 71.15
	astanaMaxLongitude = 71.75
)

var ErrOutsideAstana = errors.New("service requests are available only in Astana")

type RequestService struct {
	repo *repository.RequestRepository
}

func NewRequestService(repo *repository.RequestRepository) *RequestService {
	return &RequestService{repo: repo}
}

func (s *RequestService) CreateRequest(
	ctx context.Context,
	customerProfileID int,
	categoryID int,
	description string,
	address string,
	latitude float64,
	longitude float64,
) error {
	if !isInsideAstana(latitude, longitude) {
		return ErrOutsideAstana
	}
	return s.repo.Create(ctx, customerProfileID, categoryID, description, address, latitude, longitude)
}

func (s *RequestService) ListCustomerRequests(
	ctx context.Context,
	customerProfileID int,
) ([]repository.RequestListItem, error) {
	return s.repo.ListByCustomerProfile(ctx, customerProfileID)
}

func isInsideAstana(latitude float64, longitude float64) bool {
	return latitude >= astanaMinLatitude &&
		latitude <= astanaMaxLatitude &&
		longitude >= astanaMinLongitude &&
		longitude <= astanaMaxLongitude
}
