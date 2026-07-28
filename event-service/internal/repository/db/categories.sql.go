package db

import (
	"context"

	"github.com/google/uuid"
)

func (q *Queries) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := q.db.Query(ctx, `SELECT id, name, slug, icon, created_at FROM categories ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Category{}
	for rows.Next() {
		var i Category
		if err := rows.Scan(&i.ID, &i.Name, &i.Slug, &i.Icon, &i.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (q *Queries) GetCategoryByID(ctx context.Context, id uuid.UUID) (Category, error) {
	row := q.db.QueryRow(ctx, `SELECT id, name, slug, icon, created_at FROM categories WHERE id = $1`, id)
	var i Category
	err := row.Scan(&i.ID, &i.Name, &i.Slug, &i.Icon, &i.CreatedAt)
	return i, err
}

func (q *Queries) GetCategoryBySlug(ctx context.Context, slug string) (Category, error) {
	row := q.db.QueryRow(ctx, `SELECT id, name, slug, icon, created_at FROM categories WHERE slug = $1`, slug)
	var i Category
	err := row.Scan(&i.ID, &i.Name, &i.Slug, &i.Icon, &i.CreatedAt)
	return i, err
}
