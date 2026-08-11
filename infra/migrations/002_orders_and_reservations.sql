-- ============================================================
-- UPS-ECO-SYSTEM · Migration 002
-- Orders, Reservations, Purchase Requests (ТЗ 2.6)
-- ============================================================

-- ------------------------------------------------------------
-- Клиенты / контрагенты (минимум для CRM)
-- ------------------------------------------------------------
CREATE TABLE counterparts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            VARCHAR(32) UNIQUE,
    name            VARCHAR(512) NOT NULL,
    inn             VARCHAR(20),
    kpp             VARCHAR(20),
    type            VARCHAR(20) NOT NULL DEFAULT 'customer'
                    CHECK (type IN ('customer', 'supplier', 'both')),
    contacts        JSONB DEFAULT '{}',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_counterparts_type ON counterparts(type);
CREATE INDEX idx_counterparts_name ON counterparts USING GIN (name gin_trgm_ops);

-- ------------------------------------------------------------
-- Заказы (мультипозиционные)
-- ------------------------------------------------------------
CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    number          VARCHAR(32) NOT NULL UNIQUE,          -- UPS-2026-00001
    counterpart_id  UUID REFERENCES counterparts(id),
    
    order_type      VARCHAR(20) NOT NULL DEFAULT 'sales'
                    CHECK (order_type IN ('sales', 'production', 'project')),
    
    -- Статусная модель сделки (7 этапов из ТЗ)
    status          VARCHAR(32) NOT NULL DEFAULT 'request'
                    CHECK (status IN (
                        'request',           -- Запрос ТЗ
                        'quote',             -- КП
                        'negotiation',       -- Согласование
                        'confirmed',         -- Подтверждён
                        'in_production',     -- В производстве
                        'ready',             -- Готов к отгрузке
                        'shipped',           -- Отгружен
                        'commissioning',     -- Пусконаладка
                        'completed',         -- Завершён
                        'cancelled'
                    )),
    
    payment_status  VARCHAR(20) NOT NULL DEFAULT 'unpaid'
                    CHECK (payment_status IN ('unpaid', 'partial', 'paid')),
    
    total_amount    NUMERIC(18,2) DEFAULT 0,
    currency        VARCHAR(3) DEFAULT 'RUB',
    
    planned_ship_date DATE,
    notes           TEXT,
    
    -- Портал клиента
    client_token    VARCHAR(64) UNIQUE,
    client_token_expires TIMESTAMPTZ,
    
    created_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_counterpart ON orders(counterpart_id);
CREATE INDEX idx_orders_number ON orders(number);
CREATE INDEX idx_orders_client_token ON orders(client_token) WHERE client_token IS NOT NULL;

CREATE TABLE order_lines (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    item_id         UUID NOT NULL REFERENCES items(id),
    
    quantity        NUMERIC(18,6) NOT NULL CHECK (quantity > 0),
    uom             VARCHAR(16) NOT NULL DEFAULT 'шт',
    unit_price      NUMERIC(18,2) DEFAULT 0,
    amount          NUMERIC(18,2) GENERATED ALWAYS AS (quantity * unit_price) STORED,
    
    -- Связь с производственным заказом
    production_order_id UUID,
    
    position        INTEGER NOT NULL DEFAULT 10,
    notes           TEXT,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_lines_order ON order_lines(order_id);
CREATE INDEX idx_order_lines_item ON order_lines(item_id);

-- ------------------------------------------------------------
-- Резервы (ТЗ 2.6)
-- ------------------------------------------------------------
CREATE TABLE reservations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    item_id         UUID NOT NULL REFERENCES items(id),
    order_id        UUID REFERENCES orders(id) ON DELETE SET NULL,
    order_line_id   UUID REFERENCES order_lines(id) ON DELETE SET NULL,
    
    quantity        NUMERIC(18,6) NOT NULL CHECK (quantity > 0),
    uom             VARCHAR(16) NOT NULL DEFAULT 'шт',
    
    status          VARCHAR(20) NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'released', 'consumed', 'expired')),
    
    -- Срок резерва
    expires_at      TIMESTAMPTZ,
    
    -- Счёт (для связи с 1С)
    invoice_number  VARCHAR(64),
    
    created_by      UUID,
    released_at     TIMESTAMPTZ,
    release_reason  VARCHAR(255),
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_reservations_item ON reservations(item_id) WHERE status = 'active';
CREATE INDEX idx_reservations_order ON reservations(order_id);
CREATE INDEX idx_reservations_expires ON reservations(expires_at) WHERE status = 'active';

-- ------------------------------------------------------------
-- Заявки на закупку
-- ------------------------------------------------------------
CREATE TABLE purchase_requests (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    number          VARCHAR(32) NOT NULL UNIQUE,
    
    status          VARCHAR(20) NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'submitted', 'approved', 'ordered', 'cancelled')),
    
    source_order_id UUID REFERENCES orders(id),
    
    notes           TEXT,
    created_by      UUID,
    approved_by     UUID,
    approved_at     TIMESTAMPTZ,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE purchase_request_lines (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_request_id UUID NOT NULL REFERENCES purchase_requests(id) ON DELETE CASCADE,
    item_id             UUID NOT NULL REFERENCES items(id),
    
    quantity            NUMERIC(18,6) NOT NULL CHECK (quantity > 0),
    uom                 VARCHAR(16) NOT NULL DEFAULT 'шт',
    
    preferred_supplier_id UUID REFERENCES counterparts(id),
    needed_by           DATE,
    
    position            INTEGER NOT NULL DEFAULT 10,
    
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ------------------------------------------------------------
-- Закупки (статусы pending/shipped/received)
-- ------------------------------------------------------------
CREATE TABLE purchases (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    number          VARCHAR(32) NOT NULL UNIQUE,
    supplier_id     UUID NOT NULL REFERENCES counterparts(id),
    
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'confirmed', 'shipped', 'received', 'cancelled')),
    
    -- Информация об отгрузке
    tracking_number VARCHAR(128),
    carrier         VARCHAR(64),          -- СДЭК / Деловые линии
    shipped_at      TIMESTAMPTZ,
    expected_at     DATE,
    received_at     TIMESTAMPTZ,
    
    total_amount    NUMERIC(18,2) DEFAULT 0,
    currency        VARCHAR(3) DEFAULT 'RUB',
    
    purchase_request_id UUID REFERENCES purchase_requests(id),
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE purchase_lines (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    purchase_id     UUID NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
    item_id         UUID NOT NULL REFERENCES items(id),
    
    quantity        NUMERIC(18,6) NOT NULL CHECK (quantity > 0),
    uom             VARCHAR(16) NOT NULL DEFAULT 'шт',
    unit_price      NUMERIC(18,2) DEFAULT 0,
    
    received_qty    NUMERIC(18,6) DEFAULT 0,
    
    position        INTEGER NOT NULL DEFAULT 10,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ------------------------------------------------------------
-- Триггеры updated_at
-- ------------------------------------------------------------
CREATE TRIGGER trg_orders_updated BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_order_lines_updated BEFORE UPDATE ON order_lines
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_reservations_updated BEFORE UPDATE ON reservations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_purchases_updated BEFORE UPDATE ON purchases
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_counterparts_updated BEFORE UPDATE ON counterparts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ------------------------------------------------------------
-- Автоосвобождение резервов при отмене заказа
-- ------------------------------------------------------------
CREATE OR REPLACE FUNCTION release_reservations_on_cancel()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'cancelled' AND OLD.status <> 'cancelled' THEN
        UPDATE reservations
        SET status = 'released',
            released_at = now(),
            release_reason = 'order cancelled',
            updated_at = now()
        WHERE order_id = NEW.id AND status = 'active';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_order_cancel_release
    AFTER UPDATE OF status ON orders
    FOR EACH ROW
    EXECUTE FUNCTION release_reservations_on_cancel();
