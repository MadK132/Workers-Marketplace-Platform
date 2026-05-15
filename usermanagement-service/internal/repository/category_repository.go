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
	defaultCategories := []ServiceCategory{
		{Name: "appliance_installation", Description: ptr("Appliance setup and home device installation.")},
		{Name: "carpenter", Description: ptr("Furniture assembly, doors and small wood repairs.")},
		{Name: "cleaner", Description: ptr("Apartment, house or office cleaning.")},
		{Name: "electrician", Description: ptr("Sockets, lighting, wiring and diagnostics.")},
		{Name: "gardener", Description: ptr("Garden and plant care.")},
		{Name: "mover", Description: ptr("Loading, carrying and moving help.")},
		{Name: "plumber", Description: ptr("Pipes, leaks, mixers and plumbing.")},
		{Name: "renovation", Description: ptr("Finishing and renovation work.")},
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, category := range defaultCategories {
		if _, err := tx.Exec(ctx, `
			INSERT INTO service_categories (name, description)
			SELECT $1, $2
			WHERE NOT EXISTS (
				SELECT 1 FROM service_categories WHERE name = $1
			)
		`, category.Name, category.Description); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func ptr(value string) *string {
	return &value
}
