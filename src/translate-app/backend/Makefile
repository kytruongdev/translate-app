.PHONY: dev build fetch-pandoc fetch-pdftotext-macos fetch-pdftotext-windows sqlc migrate tidy seed-sample reset-data

PANDOC_VERSION  := 3.6.4
PANDOC_ARCH     := $(shell uname -m)
XPDF_VERSION    := 4.05
APP_BUNDLE      := build/bin/GnJ.app/Contents/MacOS
WAILS           := $(HOME)/go/bin/wails

# Run from this directory (backend = Wails module root).
dev:
	$(WAILS) dev

# Download pandoc binary for the current platform into bin/pandoc.
fetch-pandoc:
	mkdir -p bin
	curl -sL "https://github.com/jgm/pandoc/releases/download/$(PANDOC_VERSION)/pandoc-$(PANDOC_VERSION)-$(PANDOC_ARCH)-macOS.zip" \
		-o /tmp/pandoc-dl.zip
	unzip -p /tmp/pandoc-dl.zip "pandoc-$(PANDOC_VERSION)-$(PANDOC_ARCH)/bin/pandoc" > bin/pandoc
	chmod +x bin/pandoc
	rm /tmp/pandoc-dl.zip
	@echo "pandoc $(PANDOC_VERSION) ready at bin/pandoc"

# Download pdftotext (XPDF, macOS universal) into bin/pdftotext.
# XPDF is statically linked — no dylib deps, safe to bundle inside .app.
# Run once before building macOS: make fetch-pdftotext-macos
fetch-pdftotext-macos:
	mkdir -p bin
	curl -sL "https://dl.xpdfreader.com/xpdf-tools-mac-$(XPDF_VERSION).zip" \
		-o /tmp/xpdf-mac.zip
	unzip -p /tmp/xpdf-mac.zip "xpdf-tools-mac-$(XPDF_VERSION)/bin64/pdftotext" > bin/pdftotext-mac
	chmod +x bin/pdftotext-mac
	rm /tmp/xpdf-mac.zip
	@echo "pdftotext (XPDF $(XPDF_VERSION)) ready at bin/pdftotext-mac"

# Download pdftotext.exe (XPDF, Windows 64-bit) into bin/pdftotext.exe.
# XPDF is a single standalone exe — no DLLs needed, safe to bundle.
# Run once before cross-compiling to Windows: make fetch-pdftotext-windows
fetch-pdftotext-windows:
	mkdir -p bin
	curl -sL "https://dl.xpdfreader.com/xpdf-tools-win-$(XPDF_VERSION).zip" \
		-o /tmp/xpdf-win.zip
	unzip -p /tmp/xpdf-win.zip "xpdf-tools-win-$(XPDF_VERSION)/bin64/pdftotext.exe" > bin/pdftotext.exe
	rm /tmp/xpdf-win.zip
	@echo "pdftotext (XPDF $(XPDF_VERSION)) ready at bin/pdftotext.exe"

# Build Wails app and bundle pandoc + pdftotext inside the .app.
build: bin/pandoc bin/pdftotext-mac
	$(WAILS) build
	cp bin/pandoc "$(APP_BUNDLE)/pandoc"
	chmod +x "$(APP_BUNDLE)/pandoc"
	cp bin/pdftotext-mac "$(APP_BUNDLE)/pdftotext"
	chmod +x "$(APP_BUNDLE)/pdftotext"

bin/pandoc:
	$(MAKE) fetch-pandoc

bin/pdftotext-mac:
	$(MAKE) fetch-pdftotext-macos

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
