package repository

import (
	"context"

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
