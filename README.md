## Project layout

```
$HOME/go/src/tufano.com/etl/
├── go.mod                 ← module tufano.com/etl
├── go.sum
├── cmd/                   ← entrypoints (CLI, loaders, services)
│   ├── load/
│   │   └── main.go        ← example: go run ./cmd/load to run ETL
│   └── api/
│       └── main.go        ← optional future API service
├── internal/              ← reusable packages (db, utils, config)
│   ├── db/
│   │   ├── connect.go     ← creates pgxpool connections
│   │   └── migrate.go     ← wraps go-migrate
│   └── ingest/
│       └── csv_loader.go  ← parses CSVs → COPY into Postgres
├── db/
│   ├── migrations/        ← go-migrate scripts
│   │   ├── 0001_init.up.sql
│   │   └── 0001_init.down.sql
│   └── seed/
│       └── seed.sql
├── data/                  ← synthetic CSVs or scraped input files
├── docker-compose.yml     ← runs Postgres + Metabase
├── Makefile               ← convenience targets
└── README.md

```
