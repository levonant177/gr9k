package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ups-eco-system/backend/core/internal/domain/warehouse"
	"github.com/ups-eco-system/backend/core/internal/infrastructure/postgres"
)

func (h *Handler) listZones(w http.ResponseWriter, r *http.Request) {
	repo := postgres.NewWarehouseRepository(h.db)
	zones, err := repo.ListZones(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, zones)
}

func (h *Handler) listLocations(w http.ResponseWriter, r *http.Request) {
	zone := r.URL.Query().Get("zone")
	repo := postgres.NewWarehouseRepository(h.db)
	locs, err := repo.ListLocations(r.Context(), zone)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, locs)
}

func (h *Handler) createLocation(w http.ResponseWriter, r *http.Request) {
	var loc warehouse.Location
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if loc.Code == "" || loc.ZoneID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "code and zone_id required")
		return
	}
	repo := postgres.NewWarehouseRepository(h.db)
	if err := repo.CreateLocation(r.Context(), &loc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, loc)
}

func (h *Handler) createWave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProductionStartAt time.Time   `json:"production_start_at"`
		OrderIDs          []uuid.UUID `json:"order_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.ProductionStartAt.IsZero() || len(body.OrderIDs) == 0 {
		writeError(w, http.StatusBadRequest, "production_start_at and order_ids required")
		return
	}

	repo := postgres.NewWarehouseRepository(h.db)
	wave, err := repo.CreateWave(r.Context(), body.ProductionStartAt, body.OrderIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, wave)
}

func (h *Handler) getWave(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	repo := postgres.NewWarehouseRepository(h.db)
	wave, err := repo.GetWave(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "wave not found")
		return
	}
	writeJSON(w, http.StatusOK, wave)
}

func (h *Handler) releaseWave(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	repo := postgres.NewWarehouseRepository(h.db)
	if err := repo.ReleaseWave(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "released"})
}

// --- Voice inventory (критерий №5: ≤15 мин на ячейку) ---

func (h *Handler) startInventory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LocationID uuid.UUID `json:"location_id"`
		Mode       string    `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LocationID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "location_id required")
		return
	}

	repo := postgres.NewWarehouseRepository(h.db)
	session, err := repo.StartInventory(r.Context(), body.LocationID, body.Mode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (h *Handler) confirmInventoryCount(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	var body struct {
		ItemID     uuid.UUID `json:"item_id"`
		CountedQty float64   `json:"counted_qty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	repo := postgres.NewWarehouseRepository(h.db)
	if err := repo.ConfirmCount(r.Context(), sessionID, body.ItemID, body.CountedQty); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "confirmed"})
}

func (h *Handler) completeInventory(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	repo := postgres.NewWarehouseRepository(h.db)
	session, err := repo.CompleteInventory(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Критерий №5: ≤15 минут
	msg := "Инвентаризация завершена"
	if session.DurationSec != nil && *session.DurationSec > 15*60 {
		msg = "Завершено, но превышен лимит 15 минут"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session": session,
		"message": msg,
		"within_sla": session.DurationSec != nil && *session.DurationSec <= 15*60,
	})
}

func (h *Handler) getInventoryCounts(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	repo := postgres.NewWarehouseRepository(h.db)
	counts, err := repo.GetInventoryCounts(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, counts)
}
