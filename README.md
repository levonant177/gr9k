# UPS-ECO-SYSTEM

**Автоматизированная система управления предприятием (ERP + CRM + WMS + MES)**  
для производителя ИБП (серийные коробки + инжиниринговые проекты)

**Код проекта:** UPS-ECO-SYSTEM  
**Версия ТЗ:** 4.1 (утверждена 11.08.2026)  
**Срок реализации:** 24 недели

---

## Стек

| Слой                    | Технология                          |
|-------------------------|-------------------------------------|
| Backend Core            | Go (REST/gRPC)                      |
| Backend Analytics       | Python + FastAPI                    |
| Frontend Web            | SvelteKit + TailwindCSS             |
| Mobile (ТСД)            | Flutter                             |
| OLTP DB                 | PostgreSQL 16+                      |
| OLAP DB                 | ClickHouse                          |
| Cache / Lock            | Redis + Dragonfly                   |
| Message Broker          | Kafka                               |
| CI/CD / Orchestration   | GitLab CI + Kubernetes (K3s)        |
| Хранилище файлов        | S3 (MinIO on-prem)                  |
| Терминалы               | Android 10+ (Zebra / Chainway)      |

---

## Структура репозитория

```
ups-eco-system/
├── backend/
│   ├── core/                 # Go — основной backend (REST + gRPC)
│   └── analytics/            # Python + FastAPI — MRP, аналитика, отчёты
├── frontend/
│   └── web/                  # SvelteKit + Tailwind (Light Telegram design)
├── mobile/
│   └── tsd/                  # Flutter — терминалы сбора данных
├── infra/
│   ├── docker/               # docker-compose, Dockerfile'ы
│   ├── k8s/                  # манифесты K3s
│   ├── migrations/           # SQL-миграции PostgreSQL
│   └── scripts/              # утилиты
├── shared/
│   ├── proto/                # gRPC / protobuf
│   └── types/                # общие типы
├── docs/                     # документация, ТЗ, ADR
└── .gitlab/ci/               # CI/CD пайплайны
```

---

## Быстрый старт (локальная разработка)

```bash
# 1. Поднять инфраструктуру
cd infra/docker
docker compose up -d

# 2. Применить миграции
./scripts/migrate.sh up

# 3. Backend Core
cd ../../backend/core
go mod tidy
go run ./cmd/api

# 4. Frontend
cd ../../frontend/web
npm install
npm run dev
```

---

## Этапы реализации (по ТЗ)

| Этап | Содержание                                      | Срок   | Статус      |
|------|-------------------------------------------------|--------|-------------|
| 0    | Инфраструктура (K8s, базы, CI/CD)               | 2 нед. | В работе    |
| 1    | Master Data (справочники, BOM, импорт Excel)    | 3 нед. | Каркас      |
| 2    | WMS Core (адресация, резервирование, волны)     | 4 нед. | —           |
| 3    | CRM + Портал (калькулятор, статусы, портал)     | 3 нед. | —           |
| 4    | MES (маршруты, сканирование, ОТК)               | 5 нед. | —           |
| 5    | Analytics + MRP + интеграция с 1С               | 4 нед. | —           |
| 6    | Нагрузочное тестирование и UAT                  | 3 нед. | —           |

---

## Дизайн-гайд (Light Telegram)

- Фон: `#F0F2F5`
- Карточки: `#FFFFFF`
- Акцент: `#3390EC`
- Шрифты: Inter (текст), JetBrains Mono (цифры/код)
- Стеклянные элементы + Liquid Glass навигация

---

## Лицензия и конфиденциальность

Внутренний проект. Все изменения — только через письменный запрос с оценкой влияния на сроки и бюджет (согласно ТЗ 4.1).
