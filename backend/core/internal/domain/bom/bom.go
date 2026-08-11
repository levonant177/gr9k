package bom

import (
	"time"

	"github.com/google/uuid"
)

type HeaderStatus string

const (
	StatusDraft    HeaderStatus = "draft"
	StatusActive   HeaderStatus = "active"
	StatusObsolete HeaderStatus = "obsolete"
	StatusArchived HeaderStatus = "archived"
)

type NodeType string

const (
	NodeAssembly    NodeType = "assembly"
	NodeComponent   NodeType = "component"
	NodeReplaceable NodeType = "replaceable"
)

type Header struct {
	ID           uuid.UUID    `json:"id"`
	ParentItemID uuid.UUID    `json:"parent_item_id"`
	Version      string       `json:"version"`
	Revision     int          `json:"revision"`
	Name         *string      `json:"name,omitempty"`
	Description  *string      `json:"description,omitempty"`
	Status       HeaderStatus `json:"status"`
	EffectiveFrom *time.Time  `json:"effective_from,omitempty"`
	EffectiveTo   *time.Time  `json:"effective_to,omitempty"`
	Source       *string      `json:"source,omitempty"`
	SourceFile   *string      `json:"source_file,omitempty"`
	CreatedBy    *uuid.UUID   `json:"created_by,omitempty"`
	ApprovedBy   *uuid.UUID   `json:"approved_by,omitempty"`
	ApprovedAt   *time.Time   `json:"approved_at,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type Line struct {
	ID           uuid.UUID  `json:"id"`
	BomHeaderID  uuid.UUID  `json:"bom_header_id"`
	ParentLineID *uuid.UUID `json:"parent_line_id,omitempty"`
	Path         string     `json:"path,omitempty"`
	ChildItemID  uuid.UUID  `json:"child_item_id"`
	Quantity     float64    `json:"quantity"`
	UOM          string     `json:"uom"`
	NodeType     NodeType   `json:"node_type"`
	Position     int        `json:"position"`
	ScrapPercent float64    `json:"scrap_percent"`
	IsOptional   bool       `json:"is_optional"`
	Notes        *string    `json:"notes,omitempty"`
	ReplaceGroup *string    `json:"replace_group,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// Для дерева
	Children []*Line `json:"children,omitempty"`
	// Денормализация для API
	ChildArticle string `json:"child_article,omitempty"`
	ChildName    string `json:"child_name,omitempty"`
}

type ExplodedRequirement struct {
	ItemID   uuid.UUID `json:"item_id"`
	Article  string    `json:"article"`
	Name     string    `json:"name"`
	TotalQty float64   `json:"total_qty"`
	UOM      string    `json:"uom"`
	Level    int       `json:"level"`
}
