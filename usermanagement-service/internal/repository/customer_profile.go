package repository

import (
	"context"
	"diploma/usermanagement-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerProfileRepository struct {
	db *pgxpool.Pool
}

func NewCustomerProfileRepository(db *pgxpool.Pool) *CustomerProfileRepository {
	return &CustomerProfileRepository{db: db}
}

func (r *CustomerProfileRepository) Create(ctx context.Context, userID int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO customer_profiles (user_id) VALUES ($1)`,
		userID,
	)
	return err
}
func (r *CustomerProfileRepository) GetByUserID(
	ctx context.Context,
	userID int,
) (*model.CustomerProfile, error) {

	var profile model.CustomerProfile

	err := r.db.QueryRow(ctx, `
		SELECT customer_profile_id, user_id, address, latitude, longitude
		FROM customer_profiles
		WHERE user_id = $1
	`, userID).Scan(
		&profile.ID,
		&profile.UserID,
		&profile.Address,
		&profile.Latitude,
		&profile.Longitude,
	)

	if err != nil {
		return nil, err
	}

	return &profile, nil
}
