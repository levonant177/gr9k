package item

import (
	"time"

	"github.com/google/uuid"
)

type ItemType string

const (
	TypeProduct      ItemType = "product"
	TypeAssembly     ItemType = "assembly"
	TypeComponent    ItemType = "component"
	TypeReplaceable  ItemType = "replaceable"
	TypeRaw          ItemType = "raw"
)

type Item struct {
	ID               uuid.UUID              `json:"id"`
	FamilyID         *uuid.UUID             `json:"family_id,omitempty"`
	Article          string                 `json:"article"`
	Name             string                 `json:"name"`
	ItemType         ItemType               `json:"item_type"`
	Attributes       map[string]interface{} `json:"attributes"`
	UOM              string                 `json:"uom"`
	WeightKg         *float64               `json:"weight_kg,omitempty"`
	Dimensions       map[string]interface{} `json:"dimensions,omitempty"`
	IsActive         bool                   `json:"is_active"`
	IsPurchasable    bool                   `json:"is_purchasable"`
	IsSellable       bool                   `json:"is_sellable"`
	IsManufacturable bool                   `json:"is_manufacturable"`
	ShelfLifeDays    *int                   `json:"shelf_life_days,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type Family struct {
	ID              uuid.UUID              `json:"id"`
	Code            string                 `json:"code"`
	Name            string                 `json:"name"`
	Description     *string                `json:"description,omitempty"`
	AttributeSchema map[string]interface{} `json:"attribute_schema,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}
