package repository

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ServiceCategory struct {
	CategoryID  int     `json:"category_id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type CategoryRepository struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(db *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) List(ctx context.Context) ([]ServiceCategory, error) {
	rows, err := r.db.Query(ctx, `
		SELECT category_id, name, description
		FROM service_categories
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]ServiceCategory, 0)
	for rows.Next() {
		var item ServiceCategory
		var description sql.NullString
		if err := rows.Scan(&item.CategoryID, &item.Name, &description); err != nil {
			return nil, err
		}

		if description.Valid {
			item.Description = &description.String
		}

		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *CategoryRepository) EnsureDefaults(ctx context.Context) error {
	var count int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM service_categories`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	defaultCategories := []ServiceCategory{
		{Name: "Plumbing", Description: ptr("Pipes, leaks, bathroom and kitchen fixes.")},
		{Name: "Electrical", Description: ptr("Wiring, sockets, lights and diagnostics.")},
		{Name: "Cleaning", Description: ptr("Home and office cleaning services.")},
		{Name: "Carpentry", Description: ptr("Furniture assembly and wood repairs.")},
		{Name: "Painting", Description: ptr("Walls, ceilings and decorative painting.")},
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, category := range defaultCategories {
		if _, err := tx.Exec(ctx, `
			INSERT INTO service_categories (name, description)
			VALUES ($1, $2)
		`, category.Name, category.Description); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func ptr(value string) *string {
	return &value
}
