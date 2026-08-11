package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ups-eco-system/backend/core/internal/domain/warehouse"
)

type WarehouseRepository struct {
	db *pgxpool.Pool
}

func NewWarehouseRepository(db *pgxpool.Pool) *WarehouseRepository {
	return &WarehouseRepository{db: db}
}

func (r *WarehouseRepository) ListZones(ctx context.Context) ([]warehouse.Zone, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, name, description, is_fefo FROM warehouse_zones ORDER BY code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []warehouse.Zone
	for rows.Next() {
		var z warehouse.Zone
		if err := rows.Scan(&z.ID, &z.Code, &z.Name, &z.Description, &z.IsFEFO); err != nil {
			return nil, err
		}
		zones = append(zones, z)
	}
	return zones, rows.Err()
}

func (r *WarehouseRepository) ListLocations(ctx context.Context, zoneCode string) ([]warehouse.Location, error) {
	q := `
		SELECT l.id, l.zone_id, l.code, l.aisle, l.rack, l.shelf, l.bin,
		       l.max_weight_kg, l.is_active, z.code
		FROM warehouse_locations l
		JOIN warehouse_zones z ON z.id = l.zone_id
		WHERE l.is_active = true
	`
	args := []interface{}{}
	if zoneCode != "" {
		q += ` AND z.code = $1`
		args = append(args, zoneCode)
	}
	q += ` ORDER BY l.code`

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []warehouse.Location
	for rows.Next() {
		var loc warehouse.Location
		if err := rows.Scan(&loc.ID, &loc.ZoneID, &loc.Code, &loc.Aisle, &loc.Rack, &loc.Shelf, &loc.Bin,
			&loc.MaxWeightKg, &loc.IsActive, &loc.ZoneCode); err != nil {
			return nil, err
		}
		list = append(list, loc)
	}
	return list, rows.Err()
}

func (r *WarehouseRepository) CreateLocation(ctx context.Context, loc *warehouse.Location) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO warehouse_locations (zone_id, code, aisle, rack, shelf, bin, max_weight_kg)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, loc.ZoneID, loc.Code, loc.Aisle, loc.Rack, loc.Shelf, loc.Bin, loc.MaxWeightKg).Scan(&loc.ID)
}

// CreateWave — создаёт волну комплектации за 72ч до production_start
func (r *WarehouseRepository) CreateWave(ctx context.Context, productionStart time.Time, orderIDs []uuid.UUID) (*warehouse.PickWave, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	number := fmt.Sprintf("WAVE-%s-%04d", time.Now().Format("2006"), time.Now().Unix()%10000)

	var wave warehouse.PickWave
	err = tx.QueryRow(ctx, `
		INSERT INTO pick_waves (number, status, production_start_at)
		VALUES ($1, 'planned', $2)
		RETURNING id, number, status, production_start_at, release_at, created_at
	`, number, productionStart).Scan(
		&wave.ID, &wave.Number, &wave.Status, &wave.ProductionStartAt, &wave.ReleaseAt, &wave.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Собираем потребности по заказам через BOM
	for _, orderID := range orderIDs {
		rows, err := tx.Query(ctx, `
			SELECT ol.id, ol.item_id, ol.quantity, i.article
			FROM order_lines ol
			JOIN items i ON i.id = ol.item_id
			WHERE ol.order_id = $1
		`, orderID)
		if err != nil {
			return nil, err
		}

		type ol struct {
			ID, ItemID uuid.UUID
			Qty        float64
			Article    string
		}
		var ols []ol
		for rows.Next() {
			var o ol
			if err := rows.Scan(&o.ID, &o.ItemID, &o.Qty, &o.Article); err != nil {
				rows.Close()
				return nil, err
			}
			ols = append(ols, o)
		}
		rows.Close()

		pos := 10
		for _, o := range ols {
			reqRows, err := tx.Query(ctx, `
				SELECT item_id, total_qty, uom FROM get_bom_requirements($1, $2)
			`, o.ItemID, o.Qty)
			if err != nil {
				return nil, err
			}

			for reqRows.Next() {
				var itemID uuid.UUID
				var qty float64
				var uom string
				if err := reqRows.Scan(&itemID, &qty, &uom); err != nil {
					reqRows.Close()
					return nil, err
				}

				// FEFO: выбираем ячейку с ближайшим expiry
				var locID *uuid.UUID
				var batch *string
				_ = tx.QueryRow(ctx, `
					SELECT sb.location_id, sb.batch_number
					FROM stock_balances sb
					LEFT JOIN warehouse_locations wl ON wl.id = sb.location_id
					LEFT JOIN warehouse_zones wz ON wz.id = wl.zone_id
					WHERE sb.item_id = $1 AND (sb.quantity - sb.reserved_qty) >= $2
					ORDER BY
						CASE WHEN wz.is_fefo THEN sb.expiry_date END ASC NULLS LAST,
						sb.quantity DESC
					LIMIT 1
				`, itemID, qty).Scan(&locID, &batch)

				_, err = tx.Exec(ctx, `
					INSERT INTO pick_wave_lines (
						wave_id, order_id, order_line_id, item_id, quantity, uom,
						from_location_id, batch_number, position
					) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				`, wave.ID, orderID, o.ID, itemID, qty, uom, locID, batch, pos)
				if err != nil {
					reqRows.Close()
					return nil, err
				}
				pos += 10
			}
			reqRows.Close()
		}
	}

	// Проверка дефицита — блокировка волны
	var shortCount int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM pick_wave_lines
		WHERE wave_id = $1 AND from_location_id IS NULL
	`, wave.ID).Scan(&shortCount)
	if err != nil {
		return nil, err
	}

	if shortCount > 0 {
		reason := fmt.Sprintf("Дефицит по %d позициям — волна заблокирована", shortCount)
		_, err = tx.Exec(ctx, `
			UPDATE pick_waves SET status = 'blocked', blocked_reason = $2, updated_at = now()
			WHERE id = $1
		`, wave.ID, reason)
		if err != nil {
			return nil, err
		}
		wave.Status = warehouse.WaveBlocked
		wave.BlockedReason = &reason
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &wave, nil
}

func (r *WarehouseRepository) GetWave(ctx context.Context, id uuid.UUID) (*warehouse.PickWave, error) {
	var w warehouse.PickWave
	err := r.db.QueryRow(ctx, `
		SELECT id, number, status, production_start_at, release_at, blocked_reason,
		       released_at, completed_at, created_at
		FROM pick_waves WHERE id = $1
	`, id).Scan(
		&w.ID, &w.Number, &w.Status, &w.ProductionStartAt, &w.ReleaseAt, &w.BlockedReason,
		&w.ReleasedAt, &w.CompletedAt, &w.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT pwl.id, pwl.wave_id, pwl.order_id, pwl.item_id, pwl.quantity, pwl.uom,
		       pwl.from_location_id, pwl.batch_number, pwl.status, pwl.picked_qty, pwl.position,
		       i.article, i.name, COALESCE(wl.code, '')
		FROM pick_wave_lines pwl
		JOIN items i ON i.id = pwl.item_id
		LEFT JOIN warehouse_locations wl ON wl.id = pwl.from_location_id
		WHERE pwl.wave_id = $1
		ORDER BY pwl.position
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var l warehouse.WaveLine
		if err := rows.Scan(
			&l.ID, &l.WaveID, &l.OrderID, &l.ItemID, &l.Quantity, &l.UOM,
			&l.FromLocationID, &l.BatchNumber, &l.Status, &l.PickedQty, &l.Position,
			&l.Article, &l.ItemName, &l.LocationCode,
		); err != nil {
			return nil, err
		}
		w.Lines = append(w.Lines, l)
	}
	return &w, rows.Err()
}

func (r *WarehouseRepository) ReleaseWave(ctx context.Context, id uuid.UUID) error {
	// Только если не blocked и release_at наступил (или принудительно)
	tag, err := r.db.Exec(ctx, `
		UPDATE pick_waves
		SET status = 'released', released_at = now(), updated_at = now()
		WHERE id = $1 AND status = 'planned'
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("wave cannot be released (not planned or blocked)")
	}
	return nil
}

// --- Inventory (voice) ---

func (r *WarehouseRepository) StartInventory(ctx context.Context, locationID uuid.UUID, mode string) (*warehouse.InventorySession, error) {
	if mode == "" {
		mode = "voice"
	}
	number := fmt.Sprintf("INV-%s-%04d", time.Now().Format("20060102"), time.Now().Unix()%10000)

	var s warehouse.InventorySession
	err := r.db.QueryRow(ctx, `
		INSERT INTO inventory_sessions (number, status, location_id, mode, started_at)
		VALUES ($1, 'in_progress', $2, $3, now())
		RETURNING id, number, status, location_id, mode, started_at
	`, number, locationID, mode).Scan(
		&s.ID, &s.Number, &s.Status, &s.LocationID, &s.Mode, &s.StartedAt,
	)
	if err != nil {
		return nil, err
	}

	// Подтягиваем системные остатки по ячейке
	_, err = r.db.Exec(ctx, `
		INSERT INTO inventory_counts (session_id, item_id, location_id, system_qty, batch_number)
		SELECT $1, sb.item_id, sb.location_id, sb.quantity, sb.batch_number
		FROM stock_balances sb
		WHERE sb.location_id = $2 AND sb.quantity > 0
	`, s.ID, locationID)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (r *WarehouseRepository) ConfirmCount(ctx context.Context, sessionID, itemID uuid.UUID, countedQty float64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE inventory_counts
		SET counted_qty = $3, confirmed_at = now()
		WHERE session_id = $1 AND item_id = $2
	`, sessionID, itemID, countedQty)
	return err
}

func (r *WarehouseRepository) CompleteInventory(ctx context.Context, sessionID uuid.UUID) (*warehouse.InventorySession, error) {
	var s warehouse.InventorySession
	err := r.db.QueryRow(ctx, `
		UPDATE inventory_sessions
		SET status = 'completed',
		    completed_at = now(),
		    duration_sec = EXTRACT(EPOCH FROM (now() - started_at))::int
		WHERE id = $1 AND status = 'in_progress'
		RETURNING id, number, status, location_id, mode, started_at, completed_at, duration_sec
	`, sessionID).Scan(
		&s.ID, &s.Number, &s.Status, &s.LocationID, &s.Mode,
		&s.StartedAt, &s.CompletedAt, &s.DurationSec,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *WarehouseRepository) GetInventoryCounts(ctx context.Context, sessionID uuid.UUID) ([]warehouse.InventoryCount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ic.id, ic.session_id, ic.item_id, ic.system_qty, ic.counted_qty, ic.variance,
		       ic.batch_number, ic.confirmed_at, i.article, i.name
		FROM inventory_counts ic
		JOIN items i ON i.id = ic.item_id
		WHERE ic.session_id = $1
		ORDER BY i.article
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []warehouse.InventoryCount
	for rows.Next() {
		var c warehouse.InventoryCount
		if err := rows.Scan(
			&c.ID, &c.SessionID, &c.ItemID, &c.SystemQty, &c.CountedQty, &c.Variance,
			&c.BatchNumber, &c.ConfirmedAt, &c.Article, &c.ItemName,
		); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}
