package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ups-eco-system/backend/core/internal/domain/bom"
)

func (h *Handler) getBOM(w http.ResponseWriter, r *http.Request) {
	itemIDStr := chi.URLParam(r, "itemID")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	var header bom.Header
	err = h.db.QueryRow(r.Context(), `
		SELECT id, parent_item_id, version, revision, name, description, status,
		       effective_from, effective_to, source, source_file,
		       created_by, approved_by, approved_at, created_at, updated_at
		FROM bom_headers
		WHERE parent_item_id = $1 AND status = 'active'
		ORDER BY revision DESC
		LIMIT 1
	`, itemID).Scan(
		&header.ID, &header.ParentItemID, &header.Version, &header.Revision,
		&header.Name, &header.Description, &header.Status,
		&header.EffectiveFrom, &header.EffectiveTo, &header.Source, &header.SourceFile,
		&header.CreatedBy, &header.ApprovedBy, &header.ApprovedAt,
		&header.CreatedAt, &header.UpdatedAt,
	)
	if err != nil {
		writeError(w, http.StatusNotFound, "active BOM not found")
		return
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT bl.id, bl.bom_header_id, bl.parent_line_id, bl.path::text,
		       bl.child_item_id, bl.quantity, bl.uom, bl.node_type, bl.position,
		       bl.scrap_percent, bl.is_optional, bl.notes, bl.replace_group,
		       bl.created_at, bl.updated_at,
		       i.article, i.name
		FROM bom_lines bl
		JOIN items i ON i.id = bl.child_item_id
		WHERE bl.bom_header_id = $1
		ORDER BY bl.path
	`, header.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var lines []bom.Line
	for rows.Next() {
		var l bom.Line
		err := rows.Scan(
			&l.ID, &l.BomHeaderID, &l.ParentLineID, &l.Path,
			&l.ChildItemID, &l.Quantity, &l.UOM, &l.NodeType, &l.Position,
			&l.ScrapPercent, &l.IsOptional, &l.Notes, &l.ReplaceGroup,
			&l.CreatedAt, &l.UpdatedAt,
			&l.ChildArticle, &l.ChildName,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		lines = append(lines, l)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"header": header,
		"lines":  lines,
	})
}

func (h *Handler) getBOMRequirements(w http.ResponseWriter, r *http.Request) {
	itemIDStr := chi.URLParam(r, "itemID")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	qty := 1.0
	if q := r.URL.Query().Get("qty"); q != "" {
		if parsed, err := json.Number(q).Float64(); err == nil {
			qty = parsed
		}
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT item_id, article, name, total_qty, uom, level
		FROM get_bom_requirements($1, $2)
	`, itemID, qty)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var reqs []bom.ExplodedRequirement
	for rows.Next() {
		var req bom.ExplodedRequirement
		if err := rows.Scan(&req.ItemID, &req.Article, &req.Name, &req.TotalQty, &req.UOM, &req.Level); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		reqs = append(reqs, req)
	}

	writeJSON(w, http.StatusOK, reqs)
}

func (h *Handler) createBOM(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentItemID uuid.UUID `json:"parent_item_id"`
		Version      string    `json:"version"`
		Name         string    `json:"name"`
		Lines        []struct {
			ChildItemID  uuid.UUID   `json:"child_item_id"`
			Quantity     float64     `json:"quantity"`
			UOM          string      `json:"uom"`
			NodeType     bom.NodeType `json:"node_type"`
			Position     int         `json:"position"`
			ParentLineID *uuid.UUID  `json:"parent_line_id"`
		} `json:"lines"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(r.Context())

	var headerID uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO bom_headers (parent_item_id, version, name, status, source)
		VALUES ($1, $2, $3, 'draft', 'api')
		RETURNING id
	`, req.ParentItemID, req.Version, req.Name).Scan(&headerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, line := range req.Lines {
		uom := line.UOM
		if uom == "" {
			uom = "шт"
		}
		pos := line.Position
		if pos == 0 {
			pos = 10
		}
		_, err = tx.Exec(r.Context(), `
			INSERT INTO bom_lines (bom_header_id, parent_line_id, child_item_id,
			                       quantity, uom, node_type, position)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, headerID, line.ParentLineID, line.ChildItemID, line.Quantity, uom, line.NodeType, pos)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": headerID.String()})
}
