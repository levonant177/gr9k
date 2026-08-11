package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// getAvailability — 3 индикатора для менеджера (ТЗ 2.7)
// Менеджер НЕ видит BOM, только: на складе / можно собрать / будет доступно
func (h *Handler) getAvailability(w http.ResponseWriter, r *http.Request) {
	article := chi.URLParam(r, "article")
	if article == "" {
		writeError(w, http.StatusBadRequest, "article required")
		return
	}

	var (
		art, name           string
		onHand, canBuild, willBe float64
		availDate           *string
	)

	err := h.db.QueryRow(r.Context(), `
		SELECT article, name, on_hand, can_build_now, will_be_available, available_date::text
		FROM get_availability($1)
	`, article).Scan(&art, &name, &onHand, &canBuild, &willBe, &availDate)

	if err != nil {
		writeError(w, http.StatusNotFound, "item not found or not sellable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"article":            art,
		"name":               name,
		"on_hand":            onHand,            // На складе
		"can_build_now":      canBuild,          // Можно собрать сейчас
		"will_be_available":  willBe,            // Будет доступно
		"available_date":     availDate,         // Дата готовности
	})
}

func (h *Handler) listAvailability(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT article, name, on_hand, can_build_now, will_be_available, available_date
		FROM v_item_availability
		ORDER BY article
		LIMIT 200
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type row struct {
		Article          string   `json:"article"`
		Name             string   `json:"name"`
		OnHand           float64  `json:"on_hand"`
		CanBuildNow      float64  `json:"can_build_now"`
		WillBeAvailable  float64  `json:"will_be_available"`
		AvailableDate    *string  `json:"available_date"`
	}

	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.Article, &r.Name, &r.OnHand, &r.CanBuildNow, &r.WillBeAvailable, &r.AvailableDate); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		list = append(list, r)
	}

	writeJSON(w, http.StatusOK, list)
}
