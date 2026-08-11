package warehouse

import (
	"time"

	"github.com/google/uuid"
)

type ZoneCode string

const (
	ZoneReceiving ZoneCode = "receiving"
	ZoneRacks     ZoneCode = "racks"
	ZoneBulk      ZoneCode = "bulk"
	ZoneClimate   ZoneCode = "climate"
)

type WaveStatus string

const (
	WavePlanned    WaveStatus = "planned"
	WaveReleased   WaveStatus = "released"
	WaveInProgress WaveStatus = "in_progress"
	WaveCompleted  WaveStatus = "completed"
	WaveCancelled  WaveStatus = "cancelled"
	WaveBlocked    WaveStatus = "blocked"
)

type Zone struct {
	ID          uuid.UUID `json:"id"`
	Code        ZoneCode  `json:"code"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsFEFO      bool      `json:"is_fefo"`
}

type Location struct {
	ID          uuid.UUID  `json:"id"`
	ZoneID      uuid.UUID  `json:"zone_id"`
	Code        string     `json:"code"`
	Aisle       *string    `json:"aisle,omitempty"`
	Rack        *string    `json:"rack,omitempty"`
	Shelf       *string    `json:"shelf,omitempty"`
	Bin         *string    `json:"bin,omitempty"`
	MaxWeightKg *float64   `json:"max_weight_kg,omitempty"`
	IsActive    bool       `json:"is_active"`
	ZoneCode    string     `json:"zone_code,omitempty"`
}

type PickWave struct {
	ID                 uuid.UUID  `json:"id"`
	Number             string     `json:"number"`
	Status             WaveStatus `json:"status"`
	ProductionStartAt  time.Time  `json:"production_start_at"`
	ReleaseAt          *time.Time `json:"release_at,omitempty"`
	BlockedReason      *string    `json:"blocked_reason,omitempty"`
	ReleasedAt         *time.Time `json:"released_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	Lines              []WaveLine `json:"lines,omitempty"`
}

type WaveLine struct {
	ID             uuid.UUID  `json:"id"`
	WaveID         uuid.UUID  `json:"wave_id"`
	OrderID        *uuid.UUID `json:"order_id,omitempty"`
	ItemID         uuid.UUID  `json:"item_id"`
	Quantity       float64    `json:"quantity"`
	UOM            string     `json:"uom"`
	FromLocationID *uuid.UUID `json:"from_location_id,omitempty"`
	BatchNumber    *string    `json:"batch_number,omitempty"`
	Status         string     `json:"status"`
	PickedQty      float64    `json:"picked_qty"`
	Position       int        `json:"position"`
	Article        string     `json:"article,omitempty"`
	ItemName       string     `json:"item_name,omitempty"`
	LocationCode   string     `json:"location_code,omitempty"`
}

type InventorySession struct {
	ID           uuid.UUID  `json:"id"`
	Number       string     `json:"number"`
	Status       string     `json:"status"`
	LocationID   *uuid.UUID `json:"location_id,omitempty"`
	ZoneID       *uuid.UUID `json:"zone_id,omitempty"`
	Mode         string     `json:"mode"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	DurationSec  *int       `json:"duration_sec,omitempty"`
	LocationCode string     `json:"location_code,omitempty"`
}

type InventoryCount struct {
	ID          uuid.UUID  `json:"id"`
	SessionID   uuid.UUID  `json:"session_id"`
	ItemID      uuid.UUID  `json:"item_id"`
	SystemQty   float64    `json:"system_qty"`
	CountedQty  *float64   `json:"counted_qty,omitempty"`
	Variance    *float64   `json:"variance,omitempty"`
	BatchNumber *string    `json:"batch_number,omitempty"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	Article     string     `json:"article,omitempty"`
	ItemName    string     `json:"item_name,omitempty"`
}
