package service

import (
	"context"
	"errors"

	"diploma/geolocation-service/internal/repository"
)

type GeolocationRepository interface {
	FindNearbyWorkers(ctx context.Context, categoryID int, latitude float64, longitude float64, radiusMeters int) ([]repository.NearbyWorker, error)
}

type GeolocationService struct {
	repo GeolocationRepository
}

func NewGeolocationService(repo GeolocationRepository) *GeolocationService {
	return &GeolocationService{repo: repo}
}

func (s *GeolocationService) FindNearbyWorkers(
	ctx context.Context,
	categoryID int,
	latitude float64,
	longitude float64,
	radiusMeters int,
) ([]repository.NearbyWorker, error) {
	if categoryID <= 0 {
		return nil, errors.New("category_id must be positive")
	}
	if latitude < -90 || latitude > 90 {
		return nil, errors.New("invalid latitude")
	}
	if longitude < -180 || longitude > 180 {
		return nil, errors.New("invalid longitude")
	}
	if radiusMeters <= 0 {
		radiusMeters = 5000
	}

	return s.repo.FindNearbyWorkers(ctx, categoryID, latitude, longitude, radiusMeters)
}
