package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RequestRepository struct {
	db *pgxpool.Pool
}

func NewRequestRepository(db *pgxpool.Pool) *RequestRepository {
	return &RequestRepository{db: db}
}

func (r *RequestRepository) Create(
	ctx context.Context,
	customerProfileID int,
	categoryID int,
	description string,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO service_requests (customer_profile_id, category_id, description)
		VALUES ($1, $2, $3)
	`, customerProfileID, categoryID, description)

	return err
}
