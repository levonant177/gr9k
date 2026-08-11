-- ============================================================
-- UPS-ECO-SYSTEM · Migration 003
-- Stock balances + Availability indicators
-- ============================================================

CREATE TABLE IF NOT EXISTS stock_balances (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    item_id         UUID NOT NULL REFERENCES items(id),
    zone            VARCHAR(20) NOT NULL DEFAULT 'racks'
                    CHECK (zone IN ('receiving', 'racks', 'bulk', 'climate')),
    location_code   VARCHAR(64),
    quantity        NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    reserved_qty    NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (reserved_qty >= 0),
    batch_number    VARCHAR(64),
    expiry_date     DATE,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_stock_item ON stock_balances(item_id);
CREATE INDEX IF NOT EXISTS idx_stock_zone ON stock_balances(zone);

CREATE UNIQUE INDEX IF NOT EXISTS uq_stock_item_zone_loc_batch
    ON stock_balances (item_id, zone, COALESCE(location_code, ''), COALESCE(batch_number, ''));

CREATE OR REPLACE VIEW v_item_availability AS
SELECT
    i.id,
    i.article,
    i.name,
    i.item_type,
    i.attributes,
    COALESCE((
        SELECT SUM(sb.quantity - sb.reserved_qty)
        FROM stock_balances sb
        WHERE sb.item_id = i.id
    ), 0) AS on_hand,
    CASE
        WHEN i.item_type IN ('product', 'assembly') THEN
            COALESCE((
                SELECT MIN(FLOOR(
                    COALESCE((
                        SELECT SUM(sb.quantity - sb.reserved_qty)
                        FROM stock_balances sb
                        WHERE sb.item_id = bl.child_item_id
                    ), 0) / NULLIF(bl.quantity, 0)
                ))
                FROM bom_headers bh
                JOIN bom_lines bl ON bl.bom_header_id = bh.id AND bl.parent_line_id IS NULL
                WHERE bh.parent_item_id = i.id AND bh.status = 'active'
            ), 0)
        ELSE
            COALESCE((
                SELECT SUM(sb.quantity - sb.reserved_qty)
                FROM stock_balances sb
                WHERE sb.item_id = i.id
            ), 0)
    END AS can_build_now,
    CASE
        WHEN i.item_type IN ('product', 'assembly') THEN
            COALESCE((
                SELECT MIN(FLOOR(
                    COALESCE((
                        SELECT SUM(sb.quantity - sb.reserved_qty)
                        FROM stock_balances sb
                        WHERE sb.item_id = bl.child_item_id
                    ), 0) / NULLIF(bl.quantity, 0)
                ))
                FROM bom_headers bh
                JOIN bom_lines bl ON bl.bom_header_id = bh.id AND bl.parent_line_id IS NULL
                WHERE bh.parent_item_id = i.id AND bh.status = 'active'
            ), 0)
        ELSE
            COALESCE((
                SELECT SUM(sb.quantity - sb.reserved_qty)
                FROM stock_balances sb
                WHERE sb.item_id = i.id
            ), 0)
    END AS will_be_available,
    NULL::date AS available_date
FROM items i
WHERE i.is_active = true AND i.is_sellable = true;

CREATE OR REPLACE FUNCTION get_availability(p_article VARCHAR)
RETURNS TABLE (
    article           VARCHAR,
    name              VARCHAR,
    on_hand           NUMERIC,
    can_build_now     NUMERIC,
    will_be_available NUMERIC,
    available_date    DATE
) AS $$
BEGIN
    RETURN QUERY
    SELECT v.article, v.name, v.on_hand, v.can_build_now, v.will_be_available, v.available_date
    FROM v_item_availability v
    WHERE v.article = p_article;
END;
$$ LANGUAGE plpgsql STABLE;
