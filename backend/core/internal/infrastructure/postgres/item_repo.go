package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ups-eco-system/backend/core/internal/domain/item"
)

type ItemRepository struct {
	db *pgxpool.Pool
}

func NewItemRepository(db *pgxpool.Pool) *ItemRepository {
	return &ItemRepository{db: db}
}

func (r *ItemRepository) List(ctx context.Context, limit int) ([]item.Item, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, family_id, article, name, item_type, attributes, uom,
		       weight_kg, dimensions, is_active, is_purchasable, is_sellable,
		       is_manufacturable, shelf_life_days, created_at, updated_at
		FROM items
		WHERE is_active = true
		ORDER BY article
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	var items []item.Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *ItemRepository) GetByID(ctx context.Context, id uuid.UUID) (*item.Item, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, family_id, article, name, item_type, attributes, uom,
		       weight_kg, dimensions, is_active, is_purchasable, is_sellable,
		       is_manufacturable, shelf_life_days, created_at, updated_at
		FROM items WHERE id = $1
	`, id)

	it, err := scanItem(row)
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	return &it, nil
}

func (r *ItemRepository) GetByArticle(ctx context.Context, article string) (*item.Item, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, family_id, article, name, item_type, attributes, uom,
		       weight_kg, dimensions, is_active, is_purchasable, is_sellable,
		       is_manufacturable, shelf_life_days, created_at, updated_at
		FROM items WHERE article = $1
	`, article)

	it, err := scanItem(row)
	if err != nil {
		return nil, fmt.Errorf("get item by article: %w", err)
	}
	return &it, nil
}

func (r *ItemRepository) Create(ctx context.Context, it *item.Item) error {
	attrs, _ := json.Marshal(it.Attributes)
	dims, _ := json.Marshal(it.Dimensions)

	return r.db.QueryRow(ctx, `
		INSERT INTO items (
			article, name, item_type, attributes, uom, weight_kg, dimensions,
			is_active, is_purchasable, is_sellable, is_manufacturable, shelf_life_days, family_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, created_at, updated_at
	`,
		it.Article, it.Name, it.ItemType, attrs, it.UOM, it.WeightKg, dims,
		it.IsActive, it.IsPurchasable, it.IsSellable, it.IsManufacturable,
		it.ShelfLifeDays, it.FamilyID,
	).Scan(&it.ID, &it.CreatedAt, &it.UpdatedAt)
}

func (r *ItemRepository) UpsertByArticle(ctx context.Context, it *item.Item) error {
	attrs, _ := json.Marshal(it.Attributes)
	dims, _ := json.Marshal(it.Dimensions)

	return r.db.QueryRow(ctx, `
		INSERT INTO items (
			article, name, item_type, attributes, uom, weight_kg, dimensions,
			is_active, is_purchasable, is_sellable, is_manufacturable, shelf_life_days, family_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (article) DO UPDATE SET
			name = EXCLUDED.name,
			item_type = EXCLUDED.item_type,
			attributes = EXCLUDED.attributes,
			uom = EXCLUDED.uom,
			weight_kg = EXCLUDED.weight_kg,
			dimensions = EXCLUDED.dimensions,
			is_purchasable = EXCLUDED.is_purchasable,
			is_sellable = EXCLUDED.is_sellable,
			is_manufacturable = EXCLUDED.is_manufacturable,
			shelf_life_days = EXCLUDED.shelf_life_days,
			family_id = EXCLUDED.family_id,
			updated_at = now()
		RETURNING id, created_at, updated_at
	`,
		it.Article, it.Name, it.ItemType, attrs, it.UOM, it.WeightKg, dims,
		it.IsActive, it.IsPurchasable, it.IsSellable, it.IsManufacturable,
		it.ShelfLifeDays, it.FamilyID,
	).Scan(&it.ID, &it.CreatedAt, &it.UpdatedAt)
}

// scanner interface for both Row and Rows
type scanner interface {
	Scan(dest ...any) error
}

func scanItem(s scanner) (item.Item, error) {
	var it item.Item
	var attrs, dims []byte

	err := s.Scan(
		&it.ID, &it.FamilyID, &it.Article, &it.Name, &it.ItemType,
		&attrs, &it.UOM, &it.WeightKg, &dims,
		&it.IsActive, &it.IsPurchasable, &it.IsSellable, &it.IsManufacturable,
		&it.ShelfLifeDays, &it.CreatedAt, &it.UpdatedAt,
	)
	if err != nil {
		return it, err
	}

	if len(attrs) > 0 {
		_ = json.Unmarshal(attrs, &it.Attributes)
	}
	if it.Attributes == nil {
		it.Attributes = map[string]interface{}{}
	}
	if len(dims) > 0 {
		_ = json.Unmarshal(dims, &it.Dimensions)
	}
	return it, nil
}
