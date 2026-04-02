.PHONY: dev build build-macos build-windows fetch-pandoc fetch-pdftotext-macos fetch-pdftotext-windows fetch-pdftopng-macos fetch-pdftopng-windows fetch-tessdata sqlc migrate tidy seed-sample reset-data

PANDOC_VERSION  := 3.6.4
PANDOC_ARCH     := $(shell uname -m)
XPDF_VERSION    := 4.06
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
	curl -sL "https://dl.xpdfreader.com/xpdf-tools-mac-$(XPDF_VERSION).tar.gz" \
		-o /tmp/xpdf-mac.tar.gz
	tar -xzf /tmp/xpdf-mac.tar.gz -C /tmp "xpdf-tools-mac-$(XPDF_VERSION)/bin64/pdftotext"
	mv "/tmp/xpdf-tools-mac-$(XPDF_VERSION)/bin64/pdftotext" bin/pdftotext-mac
	chmod +x bin/pdftotext-mac
	rm -rf /tmp/xpdf-mac.tar.gz "/tmp/xpdf-tools-mac-$(XPDF_VERSION)"
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

# Download pdftopng (XPDF, macOS universal) into bin/pdftopng-mac.
# Same XPDF archive as pdftotext — extract the pdftopng binary.
# Used for OCR: renders PDF pages to PNG for Tesseract.
fetch-pdftopng-macos:
	mkdir -p bin
	curl -sL "https://dl.xpdfreader.com/xpdf-tools-mac-$(XPDF_VERSION).tar.gz" \
		-o /tmp/xpdf-mac.tar.gz
	tar -xzf /tmp/xpdf-mac.tar.gz -C /tmp "xpdf-tools-mac-$(XPDF_VERSION)/bin64/pdftopng"
	mv "/tmp/xpdf-tools-mac-$(XPDF_VERSION)/bin64/pdftopng" bin/pdftopng-mac
	chmod +x bin/pdftopng-mac
	rm -rf /tmp/xpdf-mac.tar.gz "/tmp/xpdf-tools-mac-$(XPDF_VERSION)"
	@echo "pdftopng (XPDF $(XPDF_VERSION)) ready at bin/pdftopng-mac"

# Download pdftopng.exe (XPDF, Windows 64-bit) into bin/pdftopng.exe.
fetch-pdftopng-windows:
	mkdir -p bin
	curl -sL "https://dl.xpdfreader.com/xpdf-tools-win-$(XPDF_VERSION).zip" \
		-o /tmp/xpdf-win.zip
	unzip -p /tmp/xpdf-win.zip "xpdf-tools-win-$(XPDF_VERSION)/bin64/pdftopng.exe" > bin/pdftopng.exe
	rm /tmp/xpdf-win.zip
	@echo "pdftopng (XPDF $(XPDF_VERSION)) ready at bin/pdftopng.exe"

# Download Tesseract tessdata (fast models) for Vietnamese + English.
# tessdata_fast models are ~4MB each — good quality/speed balance.
# For system Homebrew install, use: brew install tesseract-lang (installs all languages).
fetch-tessdata:
	mkdir -p bin/tessdata
	curl -sL "https://github.com/tesseract-ocr/tessdata_fast/raw/main/vie.traineddata" \
		-o bin/tessdata/vie.traineddata
	curl -sL "https://github.com/tesseract-ocr/tessdata_fast/raw/main/eng.traineddata" \
		-o bin/tessdata/eng.traineddata
	@echo "tessdata ready at bin/tessdata/ (vie + eng)"

# Build macOS .app (auto-fetches binaries if missing).
# Tesseract must be bundled manually — see README for instructions:
#   brew install tesseract  (dev), or copy a static build to bin/tesseract + fetch-tessdata.
build build-macos: bin/pandoc bin/pdftotext-mac bin/pdftopng-mac
	$(WAILS) build
	cp bin/pandoc "$(APP_BUNDLE)/pandoc"
	chmod +x "$(APP_BUNDLE)/pandoc"
	cp bin/pdftotext-mac "$(APP_BUNDLE)/pdftotext"
	chmod +x "$(APP_BUNDLE)/pdftotext"
	cp bin/pdftopng-mac "$(APP_BUNDLE)/pdftopng"
	chmod +x "$(APP_BUNDLE)/pdftopng"
	@if [ -f bin/tesseract ]; then \
		cp bin/tesseract "$(APP_BUNDLE)/tesseract"; \
		chmod +x "$(APP_BUNDLE)/tesseract"; \
		echo "tesseract bundled"; \
	else \
		echo "WARN: bin/tesseract not found — OCR relies on system Homebrew install"; \
	fi
	@if [ -d bin/tessdata ]; then \
		cp -r bin/tessdata "$(APP_BUNDLE)/tessdata"; \
		echo "tessdata bundled"; \
	fi

# Build Windows NSIS installer (cross-compile from Mac, requires mingw-w64).
# brew install mingw-w64  ← cài một lần nếu chưa có
build-windows: bin/pandoc.exe bin/pdftotext.exe bin/pdftopng.exe
	CC=x86_64-w64-mingw32-gcc $(WAILS) build --platform windows/amd64 -nsis

bin/pandoc:
	$(MAKE) fetch-pandoc

bin/pdftotext-mac:
	$(MAKE) fetch-pdftotext-macos

bin/pdftotext.exe:
	$(MAKE) fetch-pdftotext-windows

bin/pdftopng-mac:
	$(MAKE) fetch-pdftopng-macos

bin/pdftopng.exe:
	$(MAKE) fetch-pdftopng-windows

bin/pandoc.exe:
	@echo "bin/pandoc.exe not found — download the Windows pandoc binary manually:"
	@echo "  https://github.com/jgm/pandoc/releases/download/$(PANDOC_VERSION)/pandoc-$(PANDOC_VERSION)-windows-x86_64.zip"
	@echo "  Extract pandoc.exe → bin/pandoc.exe"
	@exit 1

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
