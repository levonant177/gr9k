package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ups-eco-system/backend/core/internal/infrastructure/postgres"
)

// Статусная модель из ТЗ (2.2)
var allowedTransitions = map[string][]string{
	"request":       {"quote", "cancelled"},
	"quote":         {"negotiation", "confirmed", "cancelled"},
	"negotiation":   {"confirmed", "quote", "cancelled"},
	"confirmed":     {"in_production", "cancelled"},
	"in_production": {"ready", "cancelled"},
	"ready":         {"shipped", "cancelled"},
	"shipped":       {"commissioning", "completed"},
	"commissioning": {"completed"},
	"completed":     {},
	"cancelled":     {},
}

// Статусы, при переходе в которые создаются резервы и «ПЗ»
var reserveOnStatuses = map[string]bool{
	"confirmed":     true,
	"in_production": true,
}

type OrderService struct {
	orders *postgres.OrderRepository
}

func NewOrderService(orders *postgres.OrderRepository) *OrderService {
	return &OrderService{orders: orders}
}

type ChangeStatusResult struct {
	OrderID            uuid.UUID `json:"order_id"`
	OldStatus          string    `json:"old_status"`
	NewStatus          string    `json:"new_status"`
	ReservationsCreated int      `json:"reservations_created,omitempty"`
	ClientToken        string    `json:"client_token,omitempty"`
	Message            string    `json:"message,omitempty"`
}

func (s *OrderService) ChangeStatus(ctx context.Context, orderID uuid.UUID, newStatus string) (*ChangeStatusResult, error) {
	order, err := s.orders.GetByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	allowed, ok := allowedTransitions[order.Status]
	if !ok {
		return nil, fmt.Errorf("unknown current status: %s", order.Status)
	}

	valid := false
	for _, a := range allowed {
		if a == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("transition %s → %s not allowed", order.Status, newStatus)
	}

	result := &ChangeStatusResult{
		OrderID:   orderID,
		OldStatus: order.Status,
		NewStatus: newStatus,
	}

	if err := s.orders.UpdateStatus(ctx, orderID, newStatus); err != nil {
		return nil, err
	}

	// При confirmed / in_production — создаём резервы (критерий приёмки №2)
	if reserveOnStatuses[newStatus] && !reserveOnStatuses[order.Status] {
		n, err := s.orders.CreateReservationsForOrder(ctx, orderID)
		if err != nil {
			return nil, fmt.Errorf("create reservations: %w", err)
		}
		result.ReservationsCreated = n
		result.Message = fmt.Sprintf("Создано резервов: %d (ПЗ сформирован)", n)
	}

	// Генерируем токен портала при подтверждении
	if newStatus == "confirmed" || newStatus == "in_production" {
		token, err := s.orders.EnsureClientToken(ctx, orderID)
		if err == nil {
			result.ClientToken = token
		}
	}

	return result, nil
}

// Ship — попытка отгрузки с проверкой оплаты (критерии №4, №8)
func (s *OrderService) Ship(ctx context.Context, orderID uuid.UUID) error {
	ok, reason, err := s.orders.CanShip(ctx, orderID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s", reason)
	}

	return s.orders.UpdateStatus(ctx, orderID, "shipped")
}
