package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/items", h.listItems)
	r.Get("/items/{id}", h.getItem)
	r.Post("/items", h.createItem)
	r.Post("/import/items", h.importItems)
	r.Post("/import/bom", h.importBOM)

	r.Get("/bom/{itemID}", h.getBOM)
	r.Get("/bom/{itemID}/requirements", h.getBOMRequirements)
	r.Post("/bom", h.createBOM)

	// Витрина менеджера (ТЗ 2.7) — 3 индикатора без раскрытия BOM
	r.Get("/availability", h.listAvailability)
	r.Get("/availability/{article}", h.getAvailability)

	// Заказы + статусы + отгрузка + портал
	r.Get("/orders/{id}", h.getOrder)
	r.Post("/orders/{id}/status", h.changeOrderStatus)
	r.Post("/orders/{id}/ship", h.shipOrder)
	r.Get("/portal/{token}", h.clientPortal)

	// WMS Core (Этап 2)
	r.Get("/warehouse/zones", h.listZones)
	r.Get("/warehouse/locations", h.listLocations)
	r.Post("/warehouse/locations", h.createLocation)
	r.Post("/warehouse/waves", h.createWave)
	r.Get("/warehouse/waves/{id}", h.getWave)
	r.Post("/warehouse/waves/{id}/release", h.releaseWave)
	r.Post("/warehouse/inventory", h.startInventory)
	r.Get("/warehouse/inventory/{id}/counts", h.getInventoryCounts)
	r.Post("/warehouse/inventory/{id}/confirm", h.confirmInventoryCount)
	r.Post("/warehouse/inventory/{id}/complete", h.completeInventory)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	return r
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
