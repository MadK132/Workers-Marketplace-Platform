package service

import (
	"context"
	"diploma/booking-service/internal/repository"
)

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
) error {
	return s.repo.Create(ctx, customerProfileID, categoryID, description)
}

func (s *RequestService) ListCustomerRequests(
	ctx context.Context,
	customerProfileID int,
) ([]repository.RequestListItem, error) {
	return s.repo.ListByCustomerProfile(ctx, customerProfileID)
}
