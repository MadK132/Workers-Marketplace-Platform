package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RequestRepository struct {
	db *pgxpool.Pool
}

type RequestListItem struct {
	RequestID    int       `json:"request_id"`
	CategoryID   int       `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
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

func (r *RequestRepository) ListByCustomerProfile(
	ctx context.Context,
	customerProfileID int,
) ([]RequestListItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			sr.request_id,
			sr.category_id,
			COALESCE(sc.name, '') AS category_name,
			COALESCE(sr.description, '') AS description,
			sr.status,
			sr.created_at
		FROM service_requests sr
		LEFT JOIN service_categories sc ON sc.category_id = sr.category_id
		WHERE sr.customer_profile_id = $1
		ORDER BY sr.created_at DESC, sr.request_id DESC
	`, customerProfileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]RequestListItem, 0)
	for rows.Next() {
		var item RequestListItem
		if err := rows.Scan(
			&item.RequestID,
			&item.CategoryID,
			&item.CategoryName,
			&item.Description,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
