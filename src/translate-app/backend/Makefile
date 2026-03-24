.PHONY: dev build fetch-pandoc sqlc migrate tidy seed-sample reset-data

PANDOC_VERSION := 3.6.4
PANDOC_ARCH    := $(shell uname -m)
APP_BUNDLE     := build/bin/translate-app.app/Contents/MacOS

# Run from this directory (backend = Wails module root).
dev:
	wails dev

# Download pandoc binary for the current platform into bin/pandoc.
fetch-pandoc:
	mkdir -p bin
	curl -sL "https://github.com/jgm/pandoc/releases/download/$(PANDOC_VERSION)/pandoc-$(PANDOC_VERSION)-$(PANDOC_ARCH)-macOS.zip" \
		-o /tmp/pandoc-dl.zip
	unzip -p /tmp/pandoc-dl.zip "pandoc-$(PANDOC_VERSION)-$(PANDOC_ARCH)/bin/pandoc" > bin/pandoc
	chmod +x bin/pandoc
	rm /tmp/pandoc-dl.zip
	@echo "pandoc $(PANDOC_VERSION) ready at bin/pandoc"

# Build Wails app and bundle pandoc binary inside the .app.
build: bin/pandoc
	wails build
	cp bin/pandoc "$(APP_BUNDLE)/pandoc"
	chmod +x "$(APP_BUNDLE)/pandoc"

bin/pandoc:
	$(MAKE) fetch-pandoc

sqlc:
	sqlc generate

migrate:
	@echo "Migrations are applied on startup (see internal/infra/db/sqlite.go)."

tidy:
	go mod tidy

# Chèn session + tin nhắn demo (idempotent). Đóng app trước khi chạy nếu bị lock DB.
seed-sample:
	go run ./cmd/seed-sample

# Xóa SQLite + thư mục file dịch (UserConfigDir/TranslateApp). Đóng app trước.
reset-data:
	@bash scripts/reset-user-data.sh
