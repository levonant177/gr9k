package http

import (
	"net/http"

	"github.com/ups-eco-system/backend/core/internal/application"
	"github.com/ups-eco-system/backend/core/internal/infrastructure/postgres"
)

func (h *Handler) importItems(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32 MB
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required (field name: file)")
		return
	}
	defer file.Close()

	itemRepo := postgres.NewItemRepository(h.db)
	bomRepo := postgres.NewBomRepository(h.db)
	svc := application.NewExcelImportService(itemRepo, bomRepo)

	result, err := svc.ImportItems(r.Context(), file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}


func (h *Handler) importBOM(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required (field name: file)")
		return
	}
	defer file.Close()

	parentArticle := r.FormValue("parent_article")

	itemRepo := postgres.NewItemRepository(h.db)
	bomRepo := postgres.NewBomRepository(h.db)
	svc := application.NewExcelImportService(itemRepo, bomRepo)

	result, err := svc.ImportBOM(r.Context(), file, parentArticle)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
