.PHONY: dev build build-macos build-windows sidecar-mac sidecar-win fetch-pandoc fetch-pdftotext-macos fetch-pdftotext-windows fetch-pdftopng-macos fetch-pdftopng-windows fetch-tessdata bundle-tesseract-macos sqlc migrate tidy seed-sample reset-data

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

# Copy Tesseract binary from Homebrew into bin/tesseract so it gets bundled in the .app.
# Requires: brew install tesseract  (on the build machine)
# After this, run: make build  — dylibbundler will fix up dylib paths automatically.
# Requires dylibbundler: brew install dylibbundler
bundle-tesseract-macos:
	@if ! command -v dylibbundler >/dev/null 2>&1; then \
		echo "ERROR: dylibbundler not found. Install with: brew install dylibbundler"; \
		exit 1; \
	fi
	@TESS=$$(command -v tesseract 2>/dev/null || echo ""); \
	if [ -z "$$TESS" ]; then \
		echo "ERROR: tesseract not found in PATH. Install with: brew install tesseract"; \
		exit 1; \
	fi; \
	mkdir -p bin; \
	cp "$$TESS" bin/tesseract; \
	chmod +x bin/tesseract; \
	echo "tesseract copied from $$TESS to bin/tesseract"

# Build macOS .app (auto-fetches binaries if missing).
# For a fully self-contained build with PDF OCR:
#   1. pip install -r ../requirements.txt pyinstaller
#   2. make sidecar-mac
#   3. make build
build build-macos: bin/pandoc bin/pdftotext-mac bin/pdftopng-mac
	$(WAILS) build
	cp bin/pandoc "$(APP_BUNDLE)/pandoc"
	chmod +x "$(APP_BUNDLE)/pandoc"
	cp bin/pdftotext-mac "$(APP_BUNDLE)/pdftotext"
	chmod +x "$(APP_BUNDLE)/pdftotext"
	cp bin/pdftopng-mac "$(APP_BUNDLE)/pdftopng"
	chmod +x "$(APP_BUNDLE)/pdftopng"
	@if [ -f bin/paddleocr-darwin-arm64 ]; then \
		cp bin/paddleocr-darwin-arm64 "$(APP_BUNDLE)/paddleocr-darwin-arm64"; \
		chmod +x "$(APP_BUNDLE)/paddleocr-darwin-arm64"; \
		echo "OCR sidecar (paddleocr-darwin-arm64) bundled"; \
	else \
		echo "WARN: bin/paddleocr-darwin-arm64 not found — PDF OCR disabled"; \
		echo "  Fix: make sidecar-mac then rebuild"; \
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

# Build OCR sidecar binary for macOS arm64.
# Run from the backend/ directory. Requires: pip install -r ../requirements.txt pyinstaller
sidecar-mac:
	cd .. && pyinstaller --clean paddleocr-darwin-arm64.spec
	mkdir -p bin
	cp ../dist/paddleocr-darwin-arm64 bin/paddleocr-darwin-arm64
	chmod +x bin/paddleocr-darwin-arm64
	@echo "Sidecar ready at bin/paddleocr-darwin-arm64"

# Build OCR sidecar binary for Windows AMD64.
# Must be run on Windows (AMD64 native, or ARM64 VM with AMD64 MSYS2 toolchain).
# Requires: pip install -r ..\requirements.txt pyinstaller
sidecar-win:
	cd .. && pyinstaller --clean paddleocr-windows-amd64.spec
	mkdir -p bin
	cp ../dist/paddleocr-windows-amd64.exe bin/paddleocr-windows-amd64.exe
	@echo "Sidecar ready at bin/paddleocr-windows-amd64.exe"

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
