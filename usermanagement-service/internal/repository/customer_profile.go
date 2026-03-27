package repository

import (
	"context"
	"database/sql"
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
	var address sql.NullString
	var latitude sql.NullFloat64
	var longitude sql.NullFloat64

	err := r.db.QueryRow(ctx, `
		SELECT customer_profile_id, user_id, address, latitude, longitude
		FROM customer_profiles
		WHERE user_id = $1
	`, userID).Scan(
		&profile.ID,
		&profile.UserID,
		&address,
		&latitude,
		&longitude,
	)

	if err != nil {
		return nil, err
	}

	if address.Valid {
		profile.Address = &address.String
	}
	if latitude.Valid {
		profile.Latitude = &latitude.Float64
	}
	if longitude.Valid {
		profile.Longitude = &longitude.Float64
	}

	return &profile, nil
}
