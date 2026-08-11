#!/usr/bin/env bash
set -euo pipefail

# Seed demo data for UPS-ECO-SYSTEM
DATABASE_URL="${DATABASE_URL:-postgres://ups:ups_secret@localhost:5432/ups_eco?sslmode=disable}"

echo "→ Seeding demo data..."

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
-- Families
INSERT INTO item_families (code, name, description) VALUES
  ('UPS', 'Источники бесперебойного питания', 'Готовые ИБП'),
  ('BATTERY', 'Аккумуляторные батареи', 'AGM / GEL / LiFePO4'),
  ('INVERTER', 'Инверторы', 'Чистый синус'),
  ('CHARGER', 'Зарядные устройства', NULL),
  ('CASE', 'Корпуса', NULL),
  ('PCB', 'Печатные платы', NULL),
  ('COOLING', 'Охлаждение', NULL),
  ('CABLE', 'Кабели и жгуты', NULL)
ON CONFLICT (code) DO NOTHING;

-- Items (products + components)
INSERT INTO items (article, name, item_type, attributes, uom, weight_kg, is_active, is_purchasable, is_sellable, is_manufacturable, family_id)
SELECT
  v.article, v.name, v.item_type::varchar, v.attributes::jsonb, 'шт', v.weight,
  true,
  v.item_type IN ('component', 'raw', 'replaceable'),
  v.item_type IN ('product', 'assembly'),
  v.item_type IN ('product', 'assembly'),
  f.id
FROM (VALUES
  ('UPS-1K-12V',  'ИБП 1кВА 12В',     'product',   '{"line":"Standard","power_kva":1,"voltage":12}', 8.5),
  ('UPS-3K-24V',  'ИБП 3кВА 24В',     'product',   '{"line":"Standard","power_kva":3,"voltage":24}', 18.2),
  ('UPS-10K-48V', 'ИБП 10кВА 48В',    'product',   '{"line":"Industrial","power_kva":10,"voltage":48}', 45.0),
  ('BAT-12V-100AH','АКБ 12В 100Ач',   'component', '{"chemistry":"AGM","capacity_ah":100}', 28.5),
  ('BAT-12V-200AH','АКБ 12В 200Ач',   'component', '{"chemistry":"AGM","capacity_ah":200}', 55.0),
  ('INV-1K-12V',  'Инвертор 1кВА 12В','component', '{"power_kva":1,"voltage":12}', 2.1),
  ('INV-3K-24V',  'Инвертор 3кВА 24В','component', '{"power_kva":3,"voltage":24}', 4.8),
  ('CHG-12V-20A', 'Зарядное 12В 20А', 'component', '{"current_a":20,"voltage":12}', 1.2),
  ('CASE-1K',     'Корпус 1кВА',      'component', '{}', 3.5),
  ('PCB-CTRL-V2', 'Плата управления v2','component','{"revision":"V2"}', 0.15),
  ('FAN-80MM',    'Вентилятор 80мм',  'component', '{}', 0.08),
  ('CABLE-PWR-2M','Кабель питания 2м','component', '{"length_m":2}', 0.35)
) AS v(article, name, item_type, attributes, weight)
JOIN item_families f ON f.code = CASE
  WHEN v.article LIKE 'UPS-%' THEN 'UPS'
  WHEN v.article LIKE 'BAT-%' THEN 'BATTERY'
  WHEN v.article LIKE 'INV-%' THEN 'INVERTER'
  WHEN v.article LIKE 'CHG-%' THEN 'CHARGER'
  WHEN v.article LIKE 'CASE-%' THEN 'CASE'
  WHEN v.article LIKE 'PCB-%' THEN 'PCB'
  WHEN v.article LIKE 'FAN-%' THEN 'COOLING'
  WHEN v.article LIKE 'CABLE-%' THEN 'CABLE'
END
ON CONFLICT (article) DO NOTHING;

-- BOM for UPS-1K-12V
WITH parent AS (
  SELECT id FROM items WHERE article = 'UPS-1K-12V'
),
header AS (
  INSERT INTO bom_headers (parent_item_id, version, revision, name, status, source)
  SELECT id, '1.0', 1, 'Спецификация ИБП 1кВА 12В', 'active', 'seed'
  FROM parent
  ON CONFLICT DO NOTHING
  RETURNING id
),
h AS (
  SELECT id FROM bom_headers WHERE parent_item_id = (SELECT id FROM parent) AND status = 'active' LIMIT 1
)
INSERT INTO bom_lines (bom_header_id, child_item_id, quantity, uom, node_type, position)
SELECT h.id, i.id, q.qty, 'шт', 'component', q.pos
FROM h
CROSS JOIN (VALUES
  ('INV-1K-12V',  1, 10),
  ('BAT-12V-100AH',1, 20),
  ('CHG-12V-20A', 1, 30),
  ('CASE-1K',     1, 40),
  ('PCB-CTRL-V2', 1, 50),
  ('FAN-80MM',    2, 60),
  ('CABLE-PWR-2M',1, 70)
) AS q(article, qty, pos)
JOIN items i ON i.article = q.article
ON CONFLICT DO NOTHING;

SELECT 'Families: ' || count(*) FROM item_families
UNION ALL
SELECT 'Items: ' || count(*) FROM items
UNION ALL
SELECT 'BOM headers: ' || count(*) FROM bom_headers
UNION ALL
SELECT 'BOM lines: ' || count(*) FROM bom_lines;
SQL

echo "✓ Seed completed"

# Sample order for status flow testing
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO counterparts (code, name, type, inn)
VALUES ('CUST-001', 'ООО «ЭнергоСнаб»', 'customer', '7701234567')
ON CONFLICT (code) DO NOTHING;

WITH cust AS (SELECT id FROM counterparts WHERE code = 'CUST-001'),
     prod AS (SELECT id FROM items WHERE article = 'UPS-1K-12V')
INSERT INTO orders (number, counterpart_id, order_type, status, payment_status, total_amount, notes)
SELECT 'UPS-2026-00001', cust.id, 'project', 'quote', 'unpaid', 185000, 'Демо-заказ для тестирования статусов'
FROM cust
ON CONFLICT (number) DO NOTHING;

WITH o AS (SELECT id FROM orders WHERE number = 'UPS-2026-00001'),
     prod AS (SELECT id FROM items WHERE article = 'UPS-1K-12V')
INSERT INTO order_lines (order_id, item_id, quantity, uom, unit_price, position)
SELECT o.id, prod.id, 2, 'шт', 92500, 10
FROM o, prod
WHERE NOT EXISTS (SELECT 1 FROM order_lines WHERE order_id = o.id);

SELECT 'Orders: ' || count(*) FROM orders;
SQL

# WMS locations + sample stock
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO warehouse_locations (zone_id, code, aisle, rack, shelf, bin)
SELECT z.id, v.code, v.aisle, v.rack, v.shelf, v.bin
FROM warehouse_zones z
JOIN (VALUES
  ('racks', 'R-A-01-01', 'A', '01', '01', '01'),
  ('racks', 'R-A-01-02', 'A', '01', '01', '02'),
  ('racks', 'R-A-02-01', 'A', '02', '01', '01'),
  ('racks', 'R-B-01-01', 'B', '01', '01', '01'),
  ('bulk',  'B-01',      '1', NULL, NULL, NULL),
  ('bulk',  'B-02',      '2', NULL, NULL, NULL),
  ('receiving', 'REC-01', '1', NULL, NULL, NULL),
  ('climate', 'CL-01-01', '1', '01', NULL, NULL)
) AS v(zone_code, code, aisle, rack, shelf, bin) ON z.code = v.zone_code
ON CONFLICT (code) DO NOTHING;

-- Sample stock
INSERT INTO stock_balances (item_id, zone, location_id, quantity, reserved_qty, batch_number, expiry_date)
SELECT i.id, 'racks', l.id, v.qty, 0, v.batch, CURRENT_DATE + v.days
FROM (VALUES
  ('INV-1K-12V',   'R-A-01-01', 50, 'B2026-001', 365),
  ('BAT-12V-100AH','R-A-01-02', 30, 'B2026-002', 180),
  ('CHG-12V-20A',  'R-A-02-01', 40, 'B2026-003', 365),
  ('CASE-1K',      'B-01',      20, NULL, NULL),
  ('PCB-CTRL-V2',  'R-B-01-01', 100,'B2026-004', 730),
  ('FAN-80MM',     'R-B-01-01', 200,'B2026-005', 365),
  ('CABLE-PWR-2M', 'R-A-01-01', 80, 'B2026-006', 365)
) AS v(article, loc, qty, batch, days)
JOIN items i ON i.article = v.article
JOIN warehouse_locations l ON l.code = v.loc
ON CONFLICT DO NOTHING;

SELECT 'Locations: ' || count(*) FROM warehouse_locations
UNION ALL
SELECT 'Stock rows: ' || count(*) FROM stock_balances;
SQL
