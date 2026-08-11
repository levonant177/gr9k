-- ============================================================
-- UPS-ECO-SYSTEM · Migration 001
-- Master Data + BOM Schema
-- PostgreSQL 16+
-- ============================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "ltree";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ------------------------------------------------------------
-- 1. Справочники (Master Data)
-- ------------------------------------------------------------

CREATE TABLE item_families (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            VARCHAR(32)  NOT NULL UNIQUE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    attribute_schema JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE items (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    family_id       UUID REFERENCES item_families(id),
    
    article         VARCHAR(64)  NOT NULL UNIQUE,
    name            VARCHAR(512) NOT NULL,
    
    item_type       VARCHAR(20)  NOT NULL 
                    CHECK (item_type IN ('product', 'assembly', 'component', 'replaceable', 'raw')),
    
    attributes      JSONB        NOT NULL DEFAULT '{}',
    
    uom             VARCHAR(16)  NOT NULL DEFAULT 'шт',
    weight_kg       NUMERIC(12,4),
    dimensions      JSONB,
    
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    is_purchasable  BOOLEAN      NOT NULL DEFAULT false,
    is_sellable     BOOLEAN      NOT NULL DEFAULT false,
    is_manufacturable BOOLEAN    NOT NULL DEFAULT false,
    
    shelf_life_days INTEGER,
    
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    
    CONSTRAINT chk_article_format CHECK (article ~ '^[A-Z0-9\-\.]+$')
);

CREATE INDEX idx_items_article ON items(article);
CREATE INDEX idx_items_type ON items(item_type);
CREATE INDEX idx_items_attributes ON items USING GIN (attributes);
CREATE INDEX idx_items_name_trgm ON items USING GIN (name gin_trgm_ops);
CREATE INDEX idx_items_family ON items(family_id);

-- ------------------------------------------------------------
-- 2. BOM Header
-- ------------------------------------------------------------

CREATE TABLE bom_headers (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parent_item_id  UUID NOT NULL REFERENCES items(id),
    
    version         VARCHAR(32)  NOT NULL DEFAULT '1.0',
    revision        INTEGER      NOT NULL DEFAULT 1,
    
    name            VARCHAR(255),
    description     TEXT,
    
    status          VARCHAR(20)  NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'active', 'obsolete', 'archived')),
    
    effective_from  DATE,
    effective_to    DATE,
    
    source          VARCHAR(50),
    source_file     VARCHAR(512),
    
    created_by      UUID,
    approved_by     UUID,
    approved_at     TIMESTAMPTZ,
    
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    
    UNIQUE (parent_item_id, version, revision)
);

CREATE INDEX idx_bom_headers_parent ON bom_headers(parent_item_id);
CREATE INDEX idx_bom_headers_status ON bom_headers(status);

-- ------------------------------------------------------------
-- 3. BOM Lines
-- ------------------------------------------------------------

CREATE TABLE bom_lines (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    bom_header_id   UUID NOT NULL REFERENCES bom_headers(id) ON DELETE CASCADE,
    
    parent_line_id  UUID REFERENCES bom_lines(id) ON DELETE CASCADE,
    path            LTREE,
    
    child_item_id   UUID NOT NULL REFERENCES items(id),
    
    quantity        NUMERIC(18,6) NOT NULL CHECK (quantity > 0),
    uom             VARCHAR(16)   NOT NULL DEFAULT 'шт',
    
    node_type       VARCHAR(20)   NOT NULL 
                    CHECK (node_type IN ('assembly', 'component', 'replaceable')),
    
    position        INTEGER       NOT NULL DEFAULT 10,
    
    scrap_percent   NUMERIC(5,2)  DEFAULT 0,
    is_optional     BOOLEAN       NOT NULL DEFAULT false,
    notes           TEXT,
    
    replace_group   VARCHAR(64),
    
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_bom_lines_header ON bom_lines(bom_header_id);
CREATE INDEX idx_bom_lines_parent ON bom_lines(parent_line_id);
CREATE INDEX idx_bom_lines_child ON bom_lines(child_item_id);
CREATE INDEX idx_bom_lines_path ON bom_lines USING GIST (path);
CREATE INDEX idx_bom_lines_replace_group ON bom_lines(replace_group) WHERE replace_group IS NOT NULL;

-- ------------------------------------------------------------
-- 4. Триггер path (ltree)
-- ------------------------------------------------------------

CREATE OR REPLACE FUNCTION bom_lines_update_path()
RETURNS TRIGGER AS $$
DECLARE
    parent_path LTREE;
BEGIN
    IF NEW.parent_line_id IS NULL THEN
        NEW.path := NEW.id::text::ltree;
    ELSE
        SELECT path INTO parent_path FROM bom_lines WHERE id = NEW.parent_line_id;
        IF parent_path IS NULL THEN
            RAISE EXCEPTION 'Parent line not found';
        END IF;
        NEW.path := parent_path || NEW.id::text::ltree;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_bom_lines_path
    BEFORE INSERT OR UPDATE OF parent_line_id
    ON bom_lines
    FOR EACH ROW
    EXECUTE FUNCTION bom_lines_update_path();

-- ------------------------------------------------------------
-- 5. View: развёртка BOM
-- ------------------------------------------------------------

CREATE OR REPLACE VIEW v_bom_exploded AS
WITH RECURSIVE bom_tree AS (
    SELECT 
        bh.id                    AS bom_header_id,
        bh.parent_item_id,
        bl.id                    AS line_id,
        bl.parent_line_id,
        bl.child_item_id,
        bl.quantity,
        bl.uom,
        bl.node_type,
        bl.position,
        bl.path,
        1                        AS level,
        bl.quantity::numeric     AS total_qty,
        ARRAY[bl.position]       AS sort_path
    FROM bom_headers bh
    JOIN bom_lines bl ON bl.bom_header_id = bh.id
    WHERE bl.parent_line_id IS NULL
      AND bh.status = 'active'

    UNION ALL

    SELECT 
        bt.bom_header_id,
        bt.parent_item_id,
        bl.id,
        bl.parent_line_id,
        bl.child_item_id,
        bl.quantity,
        bl.uom,
        bl.node_type,
        bl.position,
        bl.path,
        bt.level + 1,
        (bt.total_qty * bl.quantity)::numeric,
        bt.sort_path || bl.position
    FROM bom_tree bt
    JOIN bom_lines bl ON bl.parent_line_id = bt.line_id
    WHERE bt.level < 10
)
SELECT 
    bt.*,
    i.article,
    i.name AS item_name,
    i.item_type,
    i.attributes
FROM bom_tree bt
JOIN items i ON i.id = bt.child_item_id
ORDER BY bt.bom_header_id, bt.sort_path;

-- ------------------------------------------------------------
-- 6. Функция потребности
-- ------------------------------------------------------------

CREATE OR REPLACE FUNCTION get_bom_requirements(
    p_parent_item_id UUID,
    p_quantity       NUMERIC DEFAULT 1
)
RETURNS TABLE (
    item_id     UUID,
    article     VARCHAR,
    name        VARCHAR,
    total_qty   NUMERIC,
    uom         VARCHAR,
    level       INTEGER
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        ve.child_item_id,
        ve.article,
        ve.item_name,
        ve.total_qty * p_quantity,
        ve.uom,
        ve.level
    FROM v_bom_exploded ve
    JOIN bom_headers bh ON bh.id = ve.bom_header_id
    WHERE bh.parent_item_id = p_parent_item_id
      AND bh.status = 'active'
    ORDER BY ve.level, ve.article;
END;
$$ LANGUAGE plpgsql STABLE;

-- ------------------------------------------------------------
-- 7. Защита от циклов
-- ------------------------------------------------------------

CREATE OR REPLACE FUNCTION check_bom_cycle()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.child_item_id = (
        SELECT parent_item_id FROM bom_headers WHERE id = NEW.bom_header_id
    ) THEN
        RAISE EXCEPTION 'BOM cycle detected: item cannot contain itself';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_bom_no_self
    BEFORE INSERT OR UPDATE ON bom_lines
    FOR EACH ROW
    EXECUTE FUNCTION check_bom_cycle();

-- ------------------------------------------------------------
-- 8. Обновление updated_at
-- ------------------------------------------------------------

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_items_updated
    BEFORE UPDATE ON items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_bom_headers_updated
    BEFORE UPDATE ON bom_headers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_bom_lines_updated
    BEFORE UPDATE ON bom_lines
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
