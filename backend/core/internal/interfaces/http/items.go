package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ups-eco-system/backend/core/internal/domain/item"
)

func (h *Handler) listItems(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT id, family_id, article, name, item_type, attributes, uom,
		       weight_kg, dimensions, is_active, is_purchasable, is_sellable,
		       is_manufacturable, shelf_life_days, created_at, updated_at
		FROM items
		WHERE is_active = true
		ORDER BY article
		LIMIT 500
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var items []item.Item
	for rows.Next() {
		var it item.Item
		var attrs, dims []byte
		err := rows.Scan(
			&it.ID, &it.FamilyID, &it.Article, &it.Name, &it.ItemType,
			&attrs, &it.UOM, &it.WeightKg, &dims,
			&it.IsActive, &it.IsPurchasable, &it.IsSellable, &it.IsManufacturable,
			&it.ShelfLifeDays, &it.CreatedAt, &it.UpdatedAt,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = json.Unmarshal(attrs, &it.Attributes)
		_ = json.Unmarshal(dims, &it.Dimensions)
		items = append(items, it)
	}

	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) getItem(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var it item.Item
	var attrs, dims []byte
	err = h.db.QueryRow(r.Context(), `
		SELECT id, family_id, article, name, item_type, attributes, uom,
		       weight_kg, dimensions, is_active, is_purchasable, is_sellable,
		       is_manufacturable, shelf_life_days, created_at, updated_at
		FROM items WHERE id = $1
	`, id).Scan(
		&it.ID, &it.FamilyID, &it.Article, &it.Name, &it.ItemType,
		&attrs, &it.UOM, &it.WeightKg, &dims,
		&it.IsActive, &it.IsPurchasable, &it.IsSellable, &it.IsManufacturable,
		&it.ShelfLifeDays, &it.CreatedAt, &it.UpdatedAt,
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	_ = json.Unmarshal(attrs, &it.Attributes)
	_ = json.Unmarshal(dims, &it.Dimensions)

	writeJSON(w, http.StatusOK, it)
}

func (h *Handler) createItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Article          string                 `json:"article"`
		Name             string                 `json:"name"`
		ItemType         item.ItemType          `json:"item_type"`
		Attributes       map[string]interface{} `json:"attributes"`
		UOM              string                 `json:"uom"`
		IsPurchasable    bool                   `json:"is_purchasable"`
		IsSellable       bool                   `json:"is_sellable"`
		IsManufacturable bool                   `json:"is_manufacturable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if req.Article == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "article and name required")
		return
	}
	if req.UOM == "" {
		req.UOM = "шт"
	}

	attrs, _ := json.Marshal(req.Attributes)

	var id uuid.UUID
	err := h.db.QueryRow(r.Context(), `
		INSERT INTO items (article, name, item_type, attributes, uom,
		                   is_purchasable, is_sellable, is_manufacturable)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, req.Article, req.Name, req.ItemType, attrs, req.UOM,
		req.IsPurchasable, req.IsSellable, req.IsManufacturable).Scan(&id)

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}
