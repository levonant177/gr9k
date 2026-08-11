package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{db: db}
}

type Order struct {
	ID                uuid.UUID  `json:"id"`
	Number            string     `json:"number"`
	CounterpartID     *uuid.UUID `json:"counterpart_id,omitempty"`
	OrderType         string     `json:"order_type"`
	Status            string     `json:"status"`
	PaymentStatus     string     `json:"payment_status"`
	TotalAmount       float64    `json:"total_amount"`
	Currency          string     `json:"currency"`
	PlannedShipDate   *time.Time `json:"planned_ship_date,omitempty"`
	Notes             *string    `json:"notes,omitempty"`
	ClientToken       *string    `json:"client_token,omitempty"`
	ClientTokenExpires *time.Time `json:"client_token_expires,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type OrderLine struct {
	ID        uuid.UUID `json:"id"`
	OrderID   uuid.UUID `json:"order_id"`
	ItemID    uuid.UUID `json:"item_id"`
	Quantity  float64   `json:"quantity"`
	UOM       string    `json:"uom"`
	UnitPrice float64   `json:"unit_price"`
	Position  int       `json:"position"`
	Article   string    `json:"article,omitempty"`
	ItemName  string    `json:"item_name,omitempty"`
}

func (r *OrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*Order, error) {
	var o Order
	err := r.db.QueryRow(ctx, `
		SELECT id, number, counterpart_id, order_type, status, payment_status,
		       total_amount, currency, planned_ship_date, notes,
		       client_token, client_token_expires, created_at, updated_at
		FROM orders WHERE id = $1
	`, id).Scan(
		&o.ID, &o.Number, &o.CounterpartID, &o.OrderType, &o.Status, &o.PaymentStatus,
		&o.TotalAmount, &o.Currency, &o.PlannedShipDate, &o.Notes,
		&o.ClientToken, &o.ClientTokenExpires, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepository) GetByToken(ctx context.Context, token string) (*Order, error) {
	var o Order
	err := r.db.QueryRow(ctx, `
		SELECT id, number, counterpart_id, order_type, status, payment_status,
		       total_amount, currency, planned_ship_date, notes,
		       client_token, client_token_expires, created_at, updated_at
		FROM orders
		WHERE client_token = $1 AND client_token_expires > now()
	`, token).Scan(
		&o.ID, &o.Number, &o.CounterpartID, &o.OrderType, &o.Status, &o.PaymentStatus,
		&o.TotalAmount, &o.Currency, &o.PlannedShipDate, &o.Notes,
		&o.ClientToken, &o.ClientTokenExpires, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepository) GetLines(ctx context.Context, orderID uuid.UUID) ([]OrderLine, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ol.id, ol.order_id, ol.item_id, ol.quantity, ol.uom, ol.unit_price, ol.position,
		       i.article, i.name
		FROM order_lines ol
		JOIN items i ON i.id = ol.item_id
		WHERE ol.order_id = $1
		ORDER BY ol.position
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []OrderLine
	for rows.Next() {
		var l OrderLine
		if err := rows.Scan(&l.ID, &l.OrderID, &l.ItemID, &l.Quantity, &l.UOM, &l.UnitPrice, &l.Position, &l.Article, &l.ItemName); err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}
	return lines, rows.Err()
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, newStatus string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE orders SET status = $2, updated_at = now() WHERE id = $1
	`, id, newStatus)
	return err
}

func (r *OrderRepository) EnsureClientToken(ctx context.Context, orderID uuid.UUID) (string, error) {
	token := generateToken(32)
	expires := time.Now().Add(30 * 24 * time.Hour) // TTL 30 дней (ТЗ)

	_, err := r.db.Exec(ctx, `
		UPDATE orders
		SET client_token = $2, client_token_expires = $3, updated_at = now()
		WHERE id = $1 AND (client_token IS NULL OR client_token_expires < now())
	`, orderID, token, expires)
	if err != nil {
		return "", err
	}

	// return existing or new
	var existing string
	err = r.db.QueryRow(ctx, `SELECT client_token FROM orders WHERE id = $1`, orderID).Scan(&existing)
	if err != nil {
		return "", err
	}
	return existing, nil
}

// CreateReservationsForOrder — резервирует материалы по BOM при переходе в confirmed/in_production
func (r *OrderRepository) CreateReservationsForOrder(ctx context.Context, orderID uuid.UUID) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	lines, err := r.GetLines(ctx, orderID)
	if err != nil {
		return 0, err
	}

	created := 0
	for _, line := range lines {
		// explode BOM requirements
		rows, err := tx.Query(ctx, `
			SELECT item_id, total_qty, uom
			FROM get_bom_requirements($1, $2)
		`, line.ItemID, line.Quantity)
		if err != nil {
			return 0, fmt.Errorf("explode bom: %w", err)
		}

		type req struct {
			ItemID uuid.UUID
			Qty    float64
			UOM    string
		}
		var reqs []req
		for rows.Next() {
			var rq req
			if err := rows.Scan(&rq.ItemID, &rq.Qty, &rq.UOM); err != nil {
				rows.Close()
				return 0, err
			}
			reqs = append(reqs, rq)
		}
		rows.Close()

		for _, rq := range reqs {
			_, err = tx.Exec(ctx, `
				INSERT INTO reservations (item_id, order_id, order_line_id, quantity, uom, status, expires_at)
				VALUES ($1, $2, $3, $4, $5, 'active', now() + interval '30 days')
			`, rq.ItemID, orderID, line.ID, rq.Qty, rq.UOM)
			if err != nil {
				return 0, err
			}
			created++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return created, nil
}

// CanShip — блокировка отгрузки при отсутствии оплаты (критерий приёмки №4, №8)
func (r *OrderRepository) CanShip(ctx context.Context, orderID uuid.UUID) (bool, string, error) {
	var paymentStatus, status string
	err := r.db.QueryRow(ctx, `
		SELECT payment_status, status FROM orders WHERE id = $1
	`, orderID).Scan(&paymentStatus, &status)
	if err != nil {
		return false, "", err
	}

	if paymentStatus != "paid" {
		return false, "Отгрузка заблокирована: заказ не оплачен", nil
	}
	if status != "ready" && status != "confirmed" && status != "in_production" {
		return false, "Отгрузка заблокирована: недопустимый статус заказа", nil
	}
	return true, "", nil
}

func (r *OrderRepository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

func generateToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
