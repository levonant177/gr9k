package application

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/ups-eco-system/backend/core/internal/domain/bom"
	"github.com/ups-eco-system/backend/core/internal/domain/item"
	"github.com/ups-eco-system/backend/core/internal/infrastructure/postgres"
	"github.com/xuri/excelize/v2"
)

// ExcelImportService handles import of items, components and BOM from Excel (ТЗ 2.8)
type ExcelImportService struct {
	items *postgres.ItemRepository
	boms  *postgres.BomRepository
}

func NewExcelImportService(items *postgres.ItemRepository, boms *postgres.BomRepository) *ExcelImportService {
	return &ExcelImportService{items: items, boms: boms}
}

type ImportResult struct {
	Created int      `json:"created"`
	Updated int      `json:"updated"`
	Errors  []string `json:"errors,omitempty"`
	Total   int      `json:"total"`
}

// ImportItems imports products/components from Excel.
// Expected columns: article, name, type, family/line, execution, weight, dimensions, status
func (s *ExcelImportService) ImportItems(ctx context.Context, r io.Reader) (*ImportResult, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("open excel: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets in workbook")
	}

	result := &ImportResult{}

	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("sheet %s: %v", sheet, err))
			continue
		}
		if len(rows) < 2 {
			continue
		}

		header := normalizeHeader(rows[0])
		col := mapColumns(header)

		for i, row := range rows[1:] {
			result.Total++
			lineNum := i + 2

			article := getCell(row, col["article"])
			name := getCell(row, col["name"])
			if article == "" || name == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("%s:%d empty article or name", sheet, lineNum))
				continue
			}

			itemType := parseItemType(getCell(row, col["type"]))
			weight := parseFloat(getCell(row, col["weight"]))
			status := strings.ToLower(getCell(row, col["status"]))
			isActive := status != "inactive" && status != "архив" && status != "неактивен"

			attrs := map[string]interface{}{}
			if line := getCell(row, col["line"]); line != "" {
				attrs["line"] = line
			}
			if exec := getCell(row, col["execution"]); exec != "" {
				attrs["execution"] = exec
			}
			if family := getCell(row, col["family"]); family != "" {
				attrs["family"] = family
			}

			it := &item.Item{
				Article:          strings.ToUpper(strings.TrimSpace(article)),
				Name:             strings.TrimSpace(name),
				ItemType:         itemType,
				Attributes:       attrs,
				UOM:              "шт",
				WeightKg:         weight,
				IsActive:         isActive,
				IsPurchasable:    itemType == item.TypeComponent || itemType == item.TypeRaw || itemType == item.TypeReplaceable,
				IsSellable:       itemType == item.TypeProduct || itemType == item.TypeAssembly,
				IsManufacturable: itemType == item.TypeProduct || itemType == item.TypeAssembly,
			}

			existing, err := s.items.GetByArticle(ctx, it.Article)
			if err == nil && existing != nil {
				it.ID = existing.ID
				if err := s.items.UpsertByArticle(ctx, it); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s:%d update %s: %v", sheet, lineNum, article, err))
					continue
				}
				result.Updated++
			} else {
				if err := s.items.UpsertByArticle(ctx, it); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s:%d create %s: %v", sheet, lineNum, article, err))
					continue
				}
				result.Created++
			}
		}
	}

	return result, nil
}

// ImportBOM imports BOM from Excel (one sheet = one product).
// Columns: level (optional), article, quantity, uom, node_type, position
// First data row or parentArticle param defines the parent product.
func (s *ExcelImportService) ImportBOM(ctx context.Context, r io.Reader, parentArticle string) (*ImportResult, error) {
	if s.boms == nil {
		return nil, fmt.Errorf("bom repository not configured")
	}

	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("open excel: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets")
	}

	result := &ImportResult{}
	sheet := sheets[0]
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) < 2 {
		return nil, fmt.Errorf("empty sheet")
	}

	header := normalizeHeader(rows[0])
	col := mapColumns(header)

	// Resolve parent
	if parentArticle == "" {
		// try first column named parent or use sheet name
		parentArticle = strings.TrimSpace(sheet)
	}
	parent, err := s.items.GetByArticle(ctx, strings.ToUpper(parentArticle))
	if err != nil {
		return nil, fmt.Errorf("parent item %s not found: %w", parentArticle, err)
	}

	version := "1.0"
	name := "Импорт BOM " + parent.Article
	h := &bom.Header{
		ParentItemID: parent.ID,
		Version:      version,
		Revision:     1,
		Name:         &name,
		Status:       bom.StatusDraft,
		Source:       strPtr("excel"),
	}
	if err := s.boms.CreateHeader(ctx, h); err != nil {
		return nil, fmt.Errorf("create header: %w", err)
	}

	tx, err := s.boms.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// level stack for hierarchy: level -> line id
	levelStack := map[int]uuid.UUID{}

	for i, row := range rows[1:] {
		result.Total++
		lineNum := i + 2

		article := getCell(row, col["article"])
		if article == "" {
			continue
		}
		article = strings.ToUpper(strings.TrimSpace(article))
		if article == parent.Article {
			continue // skip self
		}

		child, err := s.items.GetByArticle(ctx, article)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("line %d: item %s not found", lineNum, article))
			continue
		}

		qty := 1.0
		if q := parseFloat(getCell(row, col["qty"])); q != nil {
			qty = *q
		}
		uom := getCell(row, col["uom"])
		if uom == "" {
			uom = "шт"
		}

		nodeType := bom.NodeComponent
		nt := strings.ToLower(getCell(row, col["type"]))
		if nt == "assembly" || nt == "сборка" || nt == "сборочный" {
			nodeType = bom.NodeAssembly
		} else if nt == "replaceable" || nt == "заменяемый" {
			nodeType = bom.NodeReplaceable
		}

		level := 1
		if lv := parseFloat(getCell(row, col["level"])); lv != nil {
			level = int(*lv)
		}
		pos := (i + 1) * 10

		var parentLineID *uuid.UUID
		if level > 1 {
			if pid, ok := levelStack[level-1]; ok {
				parentLineID = &pid
			}
		}

		line := &bom.Line{
			BomHeaderID:  h.ID,
			ParentLineID: parentLineID,
			ChildItemID:  child.ID,
			Quantity:     qty,
			UOM:          uom,
			NodeType:     nodeType,
			Position:     pos,
		}
		if err := s.boms.AddLineTx(ctx, tx, line); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("line %d: %v", lineNum, err))
			continue
		}
		levelStack[level] = line.ID
		result.Created++
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// activate
	_ = s.boms.Activate(ctx, h.ID)
	return result, nil
}

func strPtr(s string) *string { return &s }


func normalizeHeader(row []string) []string {
	out := make([]string, len(row))
	for i, h := range row {
		h = strings.ToLower(strings.TrimSpace(h))
		h = strings.ReplaceAll(h, " ", "_")
		h = strings.ReplaceAll(h, "-", "_")
		out[i] = h
	}
	return out
}

func mapColumns(header []string) map[string]int {
	aliases := map[string][]string{
		"article":   {"article", "артикул", "код", "sku", "code"},
		"name":      {"name", "наименование", "название", "title"},
		"type":      {"type", "тип", "item_type"},
		"family":    {"family", "семейство", "family_code"},
		"line":      {"line", "линейка", "product_line"},
		"execution": {"execution", "исполнение", "variant"},
		"weight":    {"weight", "вес", "weight_kg"},
		"status":    {"status", "статус"},
		"qty":       {"qty", "quantity", "количество", "кол_во", "кол-во"},
		"uom":       {"uom", "ед", "ед_изм", "unit"},
		"level":     {"level", "уровень", "lvl"},
	}

	col := map[string]int{}
	for key, names := range aliases {
		for i, h := range header {
			for _, n := range names {
				if h == n {
					col[key] = i
					break
				}
			}
		}
	}
	return col
}

func getCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func parseItemType(s string) item.ItemType {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "product", "продукт", "изделие", "товар":
		return item.TypeProduct
	case "assembly", "сборка", "сборочный":
		return item.TypeAssembly
	case "component", "комплектующий", "компонент", "деталь":
		return item.TypeComponent
	case "replaceable", "заменяемый":
		return item.TypeReplaceable
	case "raw", "сырьё", "материал":
		return item.TypeRaw
	default:
		return item.TypeComponent
	}
}

func parseFloat(s string) *float64 {
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}
