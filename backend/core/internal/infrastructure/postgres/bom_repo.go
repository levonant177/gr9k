package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ups-eco-system/backend/core/internal/domain/bom"
)

type BomRepository struct {
	db *pgxpool.Pool
}

func NewBomRepository(db *pgxpool.Pool) *BomRepository {
	return &BomRepository{db: db}
}

func (r *BomRepository) GetActiveHeader(ctx context.Context, parentItemID uuid.UUID) (*bom.Header, error) {
	var h bom.Header
	err := r.db.QueryRow(ctx, `
		SELECT id, parent_item_id, version, revision, name, description, status,
		       effective_from, effective_to, source, source_file,
		       created_by, approved_by, approved_at, created_at, updated_at
		FROM bom_headers
		WHERE parent_item_id = $1 AND status = 'active'
		ORDER BY revision DESC LIMIT 1
	`, parentItemID).Scan(
		&h.ID, &h.ParentItemID, &h.Version, &h.Revision,
		&h.Name, &h.Description, &h.Status,
		&h.EffectiveFrom, &h.EffectiveTo, &h.Source, &h.SourceFile,
		&h.CreatedBy, &h.ApprovedBy, &h.ApprovedAt,
		&h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *BomRepository) CreateHeader(ctx context.Context, h *bom.Header) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO bom_headers (parent_item_id, version, revision, name, status, source, source_file)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`, h.ParentItemID, h.Version, h.Revision, h.Name, h.Status, h.Source, h.SourceFile).
		Scan(&h.ID, &h.CreatedAt, &h.UpdatedAt)
}

func (r *BomRepository) AddLine(ctx context.Context, line *bom.Line) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO bom_lines (
			bom_header_id, parent_line_id, child_item_id,
			quantity, uom, node_type, position, scrap_percent, is_optional, notes, replace_group
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, path::text, created_at, updated_at
	`,
		line.BomHeaderID, line.ParentLineID, line.ChildItemID,
		line.Quantity, line.UOM, line.NodeType, line.Position,
		line.ScrapPercent, line.IsOptional, line.Notes, line.ReplaceGroup,
	).Scan(&line.ID, &line.Path, &line.CreatedAt, &line.UpdatedAt)
}

func (r *BomRepository) AddLineTx(ctx context.Context, tx pgx.Tx, line *bom.Line) error {
	return tx.QueryRow(ctx, `
		INSERT INTO bom_lines (
			bom_header_id, parent_line_id, child_item_id,
			quantity, uom, node_type, position
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, path::text, created_at, updated_at
	`,
		line.BomHeaderID, line.ParentLineID, line.ChildItemID,
		line.Quantity, line.UOM, line.NodeType, line.Position,
	).Scan(&line.ID, &line.Path, &line.CreatedAt, &line.UpdatedAt)
}

func (r *BomRepository) Activate(ctx context.Context, headerID uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// obsolete previous active
	var parentID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT parent_item_id FROM bom_headers WHERE id = $1`, headerID).Scan(&parentID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE bom_headers SET status = 'obsolete', updated_at = now()
		WHERE parent_item_id = $1 AND status = 'active' AND id <> $2
	`, parentID, headerID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE bom_headers SET status = 'active', updated_at = now() WHERE id = $1
	`, headerID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *BomRepository) GetRequirements(ctx context.Context, parentItemID uuid.UUID, qty float64) ([]bom.ExplodedRequirement, error) {
	rows, err := r.db.Query(ctx, `
		SELECT item_id, article, name, total_qty, uom, level
		FROM get_bom_requirements($1, $2)
	`, parentItemID, qty)
	if err != nil {
		return nil, fmt.Errorf("requirements: %w", err)
	}
	defer rows.Close()

	var list []bom.ExplodedRequirement
	for rows.Next() {
		var req bom.ExplodedRequirement
		if err := rows.Scan(&req.ItemID, &req.Article, &req.Name, &req.TotalQty, &req.UOM, &req.Level); err != nil {
			return nil, err
		}
		list = append(list, req)
	}
	return list, rows.Err()
}

func (r *BomRepository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}
