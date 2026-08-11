-- ============================================================
-- UPS-ECO-SYSTEM · Migration 004
-- WMS Core: зоны, ячейки, волны, инвентаризация (ТЗ 2.3)
-- ============================================================

-- ------------------------------------------------------------
-- Зоны хранения (4 зоны из ТЗ)
-- ------------------------------------------------------------
CREATE TABLE warehouse_zones (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            VARCHAR(20)  NOT NULL UNIQUE
                    CHECK (code IN ('receiving', 'racks', 'bulk', 'climate')),
    name            VARCHAR(128) NOT NULL,
    description     TEXT,
    is_fefo         BOOLEAN NOT NULL DEFAULT false,  -- racks = FEFO
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO warehouse_zones (code, name, is_fefo) VALUES
  ('receiving', 'Приёмка', false),
  ('racks',     'Стеллажи (FEFO)', true),
  ('bulk',      'Буль (крупногабарит)', false),
  ('climate',   'Климатическая', false)
ON CONFLICT (code) DO NOTHING;

-- ------------------------------------------------------------
-- Ячейки / адреса
-- ------------------------------------------------------------
CREATE TABLE warehouse_locations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    zone_id         UUID NOT NULL REFERENCES warehouse_zones(id),
    code            VARCHAR(64) NOT NULL UNIQUE,   -- A-01-02-03
    aisle           VARCHAR(16),
    rack            VARCHAR(16),
    shelf           VARCHAR(16),
    bin             VARCHAR(16),
    max_weight_kg   NUMERIC(12,2),
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_locations_zone ON warehouse_locations(zone_id);
CREATE INDEX idx_locations_code ON warehouse_locations(code);

-- ------------------------------------------------------------
-- Переопределяем stock_balances с привязкой к location
-- (миграция 003 создала упрощённую версию)
-- ------------------------------------------------------------
ALTER TABLE stock_balances
    ADD COLUMN IF NOT EXISTS location_id UUID REFERENCES warehouse_locations(id);

CREATE INDEX IF NOT EXISTS idx_stock_location ON stock_balances(location_id);

-- ------------------------------------------------------------
-- Волны комплектации (ТЗ 2.3: запуск за 72 часа до сборки)
-- ------------------------------------------------------------
CREATE TABLE pick_waves (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    number          VARCHAR(32) NOT NULL UNIQUE,   -- WAVE-2026-00001
    status          VARCHAR(20) NOT NULL DEFAULT 'planned'
                    CHECK (status IN ('planned', 'released', 'in_progress', 'completed', 'cancelled', 'blocked')),
    
    -- Привязка к производству
    production_start_at TIMESTAMPTZ NOT NULL,      -- старт сборки
    release_at          TIMESTAMPTZ,               -- когда волна должна быть запущена (start - 72h)
    
    blocked_reason  TEXT,                          -- дефицит и т.п.
    
    created_by      UUID,
    released_at     TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_waves_status ON pick_waves(status);
CREATE INDEX idx_waves_release ON pick_waves(release_at) WHERE status = 'planned';

CREATE TABLE pick_wave_lines (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    wave_id         UUID NOT NULL REFERENCES pick_waves(id) ON DELETE CASCADE,
    
    order_id        UUID REFERENCES orders(id),
    order_line_id   UUID REFERENCES order_lines(id),
    item_id         UUID NOT NULL REFERENCES items(id),
    
    quantity        NUMERIC(18,6) NOT NULL CHECK (quantity > 0),
    uom             VARCHAR(16) NOT NULL DEFAULT 'шт',
    
    -- Назначение ячейки (после резервирования / FEFO)
    from_location_id UUID REFERENCES warehouse_locations(id),
    batch_number    VARCHAR(64),
    
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'picked', 'short', 'cancelled')),
    picked_qty      NUMERIC(18,6) DEFAULT 0,
    
    position        INTEGER NOT NULL DEFAULT 10,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wave_lines_wave ON pick_wave_lines(wave_id);
CREATE INDEX idx_wave_lines_item ON pick_wave_lines(item_id);

-- ------------------------------------------------------------
-- Инвентаризация (голосовая — ТЗ 2.3, критерий №5 ≤15 мин/ячейка)
-- ------------------------------------------------------------
CREATE TABLE inventory_sessions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    number          VARCHAR(32) NOT NULL UNIQUE,
    status          VARCHAR(20) NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open', 'in_progress', 'completed', 'cancelled')),
    
    location_id     UUID REFERENCES warehouse_locations(id),  -- одна ячейка
    zone_id         UUID REFERENCES warehouse_zones(id),
    
    mode            VARCHAR(20) NOT NULL DEFAULT 'voice'
                    CHECK (mode IN ('voice', 'manual', 'rfid')),
    
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    duration_sec    INTEGER,                       -- для контроля ≤15 мин
    
    created_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE inventory_counts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id      UUID NOT NULL REFERENCES inventory_sessions(id) ON DELETE CASCADE,
    
    item_id         UUID NOT NULL REFERENCES items(id),
    location_id     UUID REFERENCES warehouse_locations(id),
    
    system_qty      NUMERIC(18,6) NOT NULL DEFAULT 0,   -- по учёту
    counted_qty     NUMERIC(18,6),                      -- факт (голос/ручной)
    variance        NUMERIC(18,6) GENERATED ALWAYS AS (counted_qty - system_qty) STORED,
    
    batch_number    VARCHAR(64),
    confirmed_at    TIMESTAMPTZ,                   -- момент голосового подтверждения
    confirmed_by    UUID,
    
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_inv_counts_session ON inventory_counts(session_id);

-- ------------------------------------------------------------
-- Движения склада (для аудита)
-- ------------------------------------------------------------
CREATE TABLE stock_movements (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    item_id         UUID NOT NULL REFERENCES items(id),
    
    from_location_id UUID REFERENCES warehouse_locations(id),
    to_location_id   UUID REFERENCES warehouse_locations(id),
    
    quantity        NUMERIC(18,6) NOT NULL,
    uom             VARCHAR(16) NOT NULL DEFAULT 'шт',
    
    movement_type   VARCHAR(20) NOT NULL
                    CHECK (movement_type IN (
                        'receipt', 'putaway', 'pick', 'transfer',
                        'adjustment', 'reserve', 'release', 'ship'
                    )),
    
    reference_type  VARCHAR(32),   -- order, wave, inventory, purchase
    reference_id    UUID,
    
    batch_number    VARCHAR(64),
    performed_by    UUID,
    performed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    notes           TEXT
);

CREATE INDEX idx_movements_item ON stock_movements(item_id);
CREATE INDEX idx_movements_ref ON stock_movements(reference_type, reference_id);

-- ------------------------------------------------------------
-- Триггеры
-- ------------------------------------------------------------
CREATE TRIGGER trg_waves_updated BEFORE UPDATE ON pick_waves
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Автоматический расчёт release_at = production_start_at - 72 hours
CREATE OR REPLACE FUNCTION set_wave_release_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.release_at := NEW.production_start_at - INTERVAL '72 hours';
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_wave_release_at
    BEFORE INSERT OR UPDATE OF production_start_at ON pick_waves
    FOR EACH ROW
    EXECUTE FUNCTION set_wave_release_at();
