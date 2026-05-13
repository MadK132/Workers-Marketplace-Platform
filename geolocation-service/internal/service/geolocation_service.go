package service

import (
	"context"
	"errors"

	"diploma/geolocation-service/internal/repository"
)

type GeolocationRepository interface {
	FindNearbyWorkers(ctx context.Context, categoryID int, latitude float64, longitude float64, radiusMeters int) ([]repository.NearbyWorker, error)
	UpdateWorkerLocation(ctx context.Context, userID int, latitude float64, longitude float64) error
	UpdateCustomerLocation(ctx context.Context, userID int, latitude float64, longitude float64) error
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

func (s *GeolocationService) UpdateWorkerLocation(
	ctx context.Context,
	userID int,
	latitude float64,
	longitude float64,
) error {
	if err := validateLocationInput(userID, latitude, longitude); err != nil {
		return err
	}

	return s.repo.UpdateWorkerLocation(ctx, userID, latitude, longitude)
}

func (s *GeolocationService) UpdateCustomerLocation(
	ctx context.Context,
	userID int,
	latitude float64,
	longitude float64,
) error {
	if err := validateLocationInput(userID, latitude, longitude); err != nil {
		return err
	}

	return s.repo.UpdateCustomerLocation(ctx, userID, latitude, longitude)
}

func validateLocationInput(userID int, latitude float64, longitude float64) error {
	if userID <= 0 {
		return errors.New("user_id must be positive")
	}
	if latitude < -90 || latitude > 90 {
		return errors.New("invalid latitude")
	}
	if longitude < -180 || longitude > 180 {
		return errors.New("invalid longitude")
	}
	return nil
}
