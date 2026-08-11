package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ups-eco-system/backend/core/internal/application"
	"github.com/ups-eco-system/backend/core/internal/infrastructure/postgres"
)

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	repo := postgres.NewOrderRepository(h.db)
	order, err := repo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	lines, _ := repo.GetLines(r.Context(), id)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"order": order,
		"lines": lines,
	})
}

func (h *Handler) changeOrderStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Status == "" {
		writeError(w, http.StatusBadRequest, "status required")
		return
	}

	svc := application.NewOrderService(postgres.NewOrderRepository(h.db))
	result, err := svc.ChangeStatus(r.Context(), id, body.Status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) shipOrder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	svc := application.NewOrderService(postgres.NewOrderRepository(h.db))
	if err := svc.Ship(r.Context(), id); err != nil {
		// Критерий приёмки №8 — баннер/ошибка при неоплаченном заказе
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"error":   err.Error(),
			"blocked": true,
			"banner":  "Отгрузка заблокирована: заказ не оплачен",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "shipped",
		"message": "Заказ отгружен",
	})
}

// Портал клиента по токену (ТЗ 2.2, TTL 30 дней)
func (h *Handler) clientPortal(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token required")
		return
	}

	repo := postgres.NewOrderRepository(h.db)
	order, err := repo.GetByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusNotFound, "invalid or expired token")
		return
	}

	lines, _ := repo.GetLines(r.Context(), order.ID)

	// Не отдаём чувствительные данные
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"number":            order.Number,
		"status":            order.Status,
		"payment_status":    order.PaymentStatus,
		"planned_ship_date": order.PlannedShipDate,
		"lines":             lines,
		// photo reports / gantt — заглушки до MES
		"photo_reports": []interface{}{},
		"gantt":         nil,
	})
}
