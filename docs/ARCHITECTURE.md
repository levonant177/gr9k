# Архитектура UPS-ECO-SYSTEM

## Обзор

Система построена как модульный монолит с чёткими bounded contexts, готовый к выделению сервисов.

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  SvelteKit  │────▶│  Go Core    │────▶│ PostgreSQL  │
│  (Web UI)   │     │  REST/gRPC  │     │   16+       │
└─────────────┘     └──────┬──────┘     └─────────────┘
                           │
┌─────────────┐            │            ┌─────────────┐
│  Flutter    │────────────┤            │   Redis     │
│  (ТСД)      │            │            └─────────────┘
└─────────────┘            │
                           ├───────────▶ Kafka (events)
                           │
┌─────────────┐            │            ┌─────────────┐
│  FastAPI    │────────────┘            │ ClickHouse  │
│  Analytics  │                         │   (OLAP)    │
└─────────────┘                         └─────────────┘
```

## Bounded Contexts (Этап 1)

1. **Master Data** — items, families, attributes
2. **BOM** — headers, lines, explosion, requirements
3. **Orders** (следующий) — sales orders, production orders
4. **Warehouse** — zones, locations, reservations, waves
5. **MES** — routes, QR scanning, QC protocols

## Event Bus (Kafka topics, планируемые)

- `item.created` / `item.updated`
- `bom.activated`
- `order.status_changed`
- `reservation.created` / `reservation.released`
- `production.started` / `production.completed`
- `shipment.blocked` / `shipment.completed`

## RBAC (из ТЗ)

| Роль              | Видит BOM | Списание | Цены | Отгрузка |
|-------------------|-----------|----------|------|----------|
| Гость             | нет       | нет      | нет  | нет      |
| Менеджер          | нет*      | нет      | нет  | нет      |
| Технолог          | да        | нет      | нет  | нет      |
| Админ склада      | да        | да       | нет  | да       |
| Снабженец         | да        | нет      | нет  | нет      |
| Руководитель      | да        | да       | да   | да       |

\* Менеджер видит только 3 индикатора доступности.
