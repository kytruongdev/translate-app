.PHONY: dev build sqlc migrate tidy seed-sample

# Run from this directory (backend = Wails module root).
dev:
	wails dev

build:
	wails build

sqlc:
	sqlc generate

migrate:
	@echo "Migrations are applied on startup (see internal/infra/db/sqlite.go)."

tidy:
	go mod tidy

# Chèn session + tin nhắn demo (idempotent). Đóng app trước khi chạy nếu bị lock DB.
seed-sample:
	go run ./cmd/seed-sample
