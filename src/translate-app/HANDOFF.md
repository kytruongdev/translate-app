# HANDOFF — Structured PDF Translation Pipeline

> Tài liệu này mô tả trạng thái hiện tại của feature branch `pdf-ocr-with-format`,
> những gì đã làm, và những gì còn lại — để bất kỳ ai (hoặc Claude session mới) có thể tiếp tục ngay.

---

## 1. Context & Branch

| Item | Value |
|---|---|
| **Branch** | `pdf-ocr-with-format` |
| **Base** | `master` |
| **Last commit** | `36ad262` — `[claude] Implement structured PDF translation pipeline (Phase 1+2)` |
| **Status** | Phase 1 + Phase 2 hoàn thành ✅. Còn lại: FE export (nhỏ) + testing. |

---

## 2. Mục tiêu của feature này

**Replace toàn bộ** pipeline PDF cũ (Tesseract + plain text) bằng một pipeline **layout-aware** mới:

- PDF input → go-fitz render từng trang thành PNG → Python sidecar (RapidOCR + rapid_layout + rapid_table) phân tích layout → Go dịch từng segment → export `.html` với bảng biểu, hình ảnh được giữ nguyên.
- Output format thay đổi: `.docx` → `.html` cho PDF files.
- **Không có routing**: mọi PDF (scan hay digital) đều đi qua cùng một pipeline.

---

## 3. Những gì đã làm (DONE ✅)

### Phase 1 — Python OCR Sidecar (`TASK-BE-00` + `TASK-DEV-00`)

**File:** `ocr_sidecar.py` (rewrite hoàn toàn)
- Nhận N PNG paths làm CLI arguments
- Chạy `rapid_layout` để detect text/title/table/figure regions
- `text`/`title` → `rapidocr` extract text
- `table` → `rapid_table` → HTML string
- `figure` → Figure Classifier (whitelist: logo/seal/signature → decorative; còn lại có text → informational)
- Output: `{"pages": [{"page_no", "width", "height", "regions": [...]}]}`

**Files:** `requirements.txt`, `paddleocr-darwin-arm64.spec`, `paddleocr-windows-amd64.spec` (NEW)
- Thêm `rapid_layout`, `numpy` vào requirements
- Cả hai spec files dùng `collect_all` cho `cv2`, `rapid_layout`, `rapid_table`, `rapidocr_onnxruntime`

**File:** `backend/Makefile`
- Thêm `sidecar-mac`: build `paddleocr-darwin-arm64` via PyInstaller → `bin/`
- Thêm `sidecar-win`: build `paddleocr-windows-amd64.exe` via PyInstaller → `bin/`
- `build build-macos`: bundle `bin/paddleocr-darwin-arm64` vào `.app`, warn nếu không có

### Phase 2 — Go Backend (`TASK-DB-01` + `TASK-BE-01..08`)

**DB layer — `output_format` column (migration 008 đã tồn tại):**
- `backend/internal/repository/sqlc/queries/files.sql`: `InsertFile` + `UpdateFileTranslated` + `GetFileById` có `output_format`
- `backend/internal/repository/sqlcgen/models.go`: `File.OutputFormat string`
- `backend/internal/repository/sqlcgen/files.sql.go`: scan + params updated manually (SQLC không re-generate)
- `backend/internal/model/file.go`: `OutputFormat string` field
- `backend/internal/repository/convert.go`: `fileFromRow()` maps `r.OutputFormat`
- `backend/internal/repository/file_repo.go`: `UpdateTranslated(..., outputFormat string)` interface + impl
- `backend/internal/bridge/types.go`: `FileResult.OutputFormat string`
- `backend/internal/controller/file/translate.go`: set `outputFormat` khi insert (pdf→"html", xlsx→"xlsx", docx→"docx")
- Tất cả callers của `UpdateTranslated` + `FileResult` đã được cập nhật

**New Go files:**
- `backend/internal/controller/file/render_pdf.go` — `renderPDFToImages(ctx, pdfPath)` dùng go-fitz 200 DPI
- `backend/internal/controller/file/ocr_result.go` — `StructuredOCRResult`, `OCRPage`, `OCRRegion` structs + `regionKey()`
- `backend/internal/controller/file/ocr_runner.go` — `findStructuredOCRSidecar()` + `runStructuredOCR()`
- `backend/internal/controller/file/pipeline_pdf_structured.go` — `runStructuredPDFTranslate()` (full pipeline) + helpers

**Modified Go files:**
- `backend/internal/controller/file/image_proc.go`: thêm `extractFigureCrops()` — crop tất cả figure regions → `map[regionKey]base64PNG`
- `backend/internal/controller/file/pdf_html_builder.go`: rewrite `assembleStructuredHTML(result, translated, figureCrops)` — text/title/table/figure regions → HTML
- `backend/internal/controller/file/pipeline.go`: route `.pdf` → `runStructuredPDFTranslate` (thay vì `runPDFTranslate`)
- `backend/internal/controller/file/new.go`: `ExportFile` dùng `f.OutputFormat` để chọn save dialog filter

**Dependency added:** `github.com/gen2brain/go-fitz v1.24.15` (MuPDF Go binding)

**Deleted:** `backend/repro_ocr_test.go` (scratch file, conflicted với `main.go`)

---

## 4. Những gì còn lại (TODO)

### `TASK-FE-01` — Frontend: export button nhận diện HTML _(nhỏ)_

**Vấn đề:** `FileResult.OutputFormat` giờ có giá trị `"html"` cho PDF files, nhưng FE chưa dùng nó.

**Cần làm:**
1. Kiểm tra `file:done` event handler trong FE — field `outputFormat` đã có trong payload nhờ `bridge/types.go`.
2. Wails tự generate TS bindings từ Go types → kiểm tra `frontend/wailsjs/go/models.ts` có `FileResult.outputFormat` chưa. Nếu chưa có → chạy `wails generate module` hoặc cập nhật thủ công.
3. Trong `FileTranslationCard` component (hoặc tương đương), nơi hiển thị "Export" button và thông báo thành công: cập nhật text để hiển thị `.html` thay vì `.docx` khi `outputFormat === "html"`.
4. **Không cần thay đổi logic export** — `ExportFile` IPC call vẫn giữ nguyên signature. Chỉ là UX text thay đổi.

**Files cần tìm (chưa confirm vì FE chưa được explore):**
- `frontend/src/` — search `file:done` hoặc `ExportFile` hoặc `FileTranslationCard`
- `frontend/wailsjs/go/models.ts` — kiểm tra `FileResult` type

### `TASK-DEV-01` — Testing _(manual)_

**Regression (automated):** `go test ./...` từ `backend/` → expect 100% pass ✅ (đã pass sau Phase 2)

**Manual happy path (cần file test thực tế):**
1. PDF digital có bảng biểu → verify HTML table rows/cols đúng
2. PDF scan (chụp từ máy quét) → verify text readable, table còn structure
3. PDF có con dấu/chữ ký → verify ảnh xuất hiện đúng vị trí trong HTML

**Manual sad path:**
1. PDF có mật khẩu → verify error message hiển thị (không phải spinner chạy mãi)
2. Cancel giữa chừng → verify `status = cancelled`, không treo UI
3. Xóa `bin/paddleocr-darwin-arm64` → verify error message rõ ràng thay vì crash

---

## 5. Architecture tóm tắt

```
PDF File
  │
  ├─ renderPDFToImages()       ← go-fitz 200 DPI → temp PNG dir
  │   [render_pdf.go]
  │
  ├─ runStructuredOCR()        ← Python sidecar subprocess (1 invocation)
  │   [ocr_runner.go]             paddleocr-darwin-arm64 page-0001.png ... page-000N.png
  │                               → stdout JSON: {"pages": [...]}
  │
  ├─ extractFigureCrops()      ← crop figure bbox từ PNG → Base64
  │   [image_proc.go]
  │
  ├─ os.RemoveAll(tempDir)     ← XÓA temp PNG dir tại đây
  │
  ├─ collectSegments()         ← phân loại: text/title/table/figure text
  │   [pipeline_pdf_structured.go]
  │
  ├─ translatePDFSegments()    ← dịch concurrent (MaxBatchConcurrency goroutines)
  │   [pipeline_pdf_structured.go]
  │
  ├─ assembleStructuredHTML()  ← ghép translated + figureCrops → HTML string
  │   [pdf_html_builder.go]
  │
  └─ Persist + Emit
      source.md + translated.html → UserFilesDir/files/{fileId}/
      DB: output_format = "html", status = "done"
      Events: file:done {outputFormat: "html"}
```

---

## 6. Sidecar JSON Contract

```json
{
  "pages": [
    {
      "page_no": 1,
      "width": 1654,
      "height": 2339,
      "regions": [
        { "type": "text",  "bbox": [100, 80, 900, 120], "content": "Tiêu đề" },
        { "type": "title", "bbox": [100, 140, 600, 180], "content": "Chương 1" },
        { "type": "table", "bbox": [100, 200, 1500, 900], "html": "<table>...</table>" },
        { "type": "figure", "figure_type": "decorative", "bbox": [50, 20, 300, 120] },
        { "type": "figure", "figure_type": "informational", "bbox": [100, 500, 1500, 1200],
          "text_lines": ["Doanh thu Q1", "2,500 tỷ"] }
      ]
    }
  ]
}
```

---

## 7. Build Instructions

### Build sidecar (macOS)
```bash
# Từ translate-app/src/translate-app/
pip install -r requirements.txt
cd backend && make sidecar-mac
# Output: backend/bin/paddleocr-darwin-arm64
```

### Build sidecar (Windows AMD64 trên ARM64 VM)
```bash
# Cài AMD64 MSYS2 trước: pacman -S mingw-w64-x86_64-mupdf mingw-w64-x86_64-gcc
pip install -r requirements.txt
cd backend && make sidecar-win
# Output: backend/bin/paddleocr-windows-amd64.exe
```

### Build Go backend (kiểm tra compile)
```bash
cd backend && go build ./...
```

### Build full app (macOS)
```bash
cd backend
make sidecar-mac           # build Python OCR sidecar
make fetch-pandoc          # download pandoc (nếu chưa có)
make fetch-pdftotext-macos # download pdftotext (nếu chưa có)
make fetch-pdftopng-macos  # download pdftopng (nếu chưa có)
make build                 # wails build + bundle all binaries
```

### Dev mode
```bash
cd backend && make dev
```

---

## 8. Key Files Map

| File | Mô tả |
|---|---|
| `ocr_sidecar.py` | Python OCR sidecar — layout-aware, batch processing |
| `paddleocr-darwin-arm64.spec` | PyInstaller spec macOS |
| `paddleocr-windows-amd64.spec` | PyInstaller spec Windows (NEW) |
| `requirements.txt` | Python dependencies |
| `backend/Makefile` | Build targets (sidecar-mac, sidecar-win, build) |
| `backend/go.mod` | go-fitz đã được add |
| `backend/internal/controller/file/render_pdf.go` | PDF → PNG via go-fitz |
| `backend/internal/controller/file/ocr_result.go` | Go structs cho sidecar JSON |
| `backend/internal/controller/file/ocr_runner.go` | Sidecar subprocess runner |
| `backend/internal/controller/file/image_proc.go` | `extractFigureCrops()` |
| `backend/internal/controller/file/pdf_html_builder.go` | HTML assembler |
| `backend/internal/controller/file/pipeline_pdf_structured.go` | Full PDF pipeline |
| `backend/internal/controller/file/pipeline.go` | Route `.pdf` → new pipeline |
| `backend/internal/controller/file/new.go` | `ExportFile` HTML support |
| `backend/internal/controller/file/translate.go` | Set `outputFormat` khi insert |
| `backend/internal/model/file.go` | `OutputFormat string` field |
| `backend/internal/bridge/types.go` | `FileResult.OutputFormat` |
| `backend/internal/repository/file_repo.go` | `UpdateTranslated` + `outputFormat` |
| `backend/internal/repository/sqlcgen/models.go` | `File.OutputFormat` |
| `backend/internal/repository/sqlcgen/files.sql.go` | Generated SQLC (manual update) |
| `backend/internal/infra/db/migrations/008_pdf_structured_support.sql` | DB migration |

---

## 9. Decisions & Gotchas

### go-fitz (CGo)
- Library: `github.com/gen2brain/go-fitz v1.24.15`
- Cần `mupdf` installed trên build machine: `brew install mupdf` (macOS)
- Windows: cần `mingw-w64-x86_64-mupdf` từ AMD64 MSYS2

### sqlcgen files là manual update
- `sqlcgen/models.go` và `sqlcgen/files.sql.go` được update thủ công vì migration 008 thêm column sau khi SQLC đã generate.
- Nếu chạy lại `sqlc generate`, cần đảm bảo `sqlc.yaml` trỏ đúng schema path có migration 008.

### CSS `%` trong fmt.Sprintf template
- `pdfHTMLTemplate` dùng `fmt.Sprintf` → tất cả `%` trong CSS phải escape thành `%%`.
- Hiện tại đã fix: `width: 100%%;`, `max-width: 100%%;`.

### `pipeline_pdf.go` vẫn còn
- `runPDFTranslate()` trong `pipeline_pdf.go` là dead code (không còn được gọi).
- Giữ lại để tham khảo. Có thể xóa sau khi testing ổn định.

### Tesseract artifacts
- `bin/tesseract`, `bin/tessdata/`, `bundle-tesseract-macos`, `fetch-tessdata` targets trong Makefile: không còn cần cho PDF pipeline mới.
- Giữ nguyên các targets đó để không break existing setup. Có thể cleanup sau.

### Sidecar model load time
- Lần đầu tiên chạy sidecar, ONNX models load từ disk (~2-5 giây). Các trang tiếp theo nhanh hơn vì model đã load.
- Sidecar nhận **tất cả** trang trong 1 invocation → model chỉ load 1 lần cho toàn bộ file.

### Translation của table HTML
- Table HTML được gửi thẳng lên AI với `preserveMarkdown=true`.
- AI được expected giữ nguyên HTML structure, chỉ dịch text bên trong `<td>` và `<th>`.
- Nếu AI modify HTML structure → table có thể bị broken. Đây là known risk của v1, acceptable.

---

## 10. Testing checklist nhanh

```bash
# 1. Build check
cd backend && go test ./...  # expect all pass

# 2. Sidecar test thủ công (nếu đã có bin/paddleocr-darwin-arm64)
./bin/paddleocr-darwin-arm64 /path/to/test-page.png
# Expect: JSON với "pages" array

# 3. Dev run
cd backend && make dev
# Upload PDF → observe pipeline logs → export HTML → open in browser
```

---

## 11. PR checklist trước khi merge

- [ ] `go test ./...` pass 100%
- [ ] `make sidecar-mac` thành công (cần Python env)
- [ ] `make build` thành công (cần go-fitz + pandoc + pdftotext + pdftopng)
- [ ] Manual test: PDF digital có bảng → HTML đúng structure
- [ ] Manual test: PDF scan → text readable
- [ ] Manual test: PDF có seal/signature → ảnh giữ nguyên trong HTML
- [ ] Manual test: Export HTML → file mở được trong browser
- [ ] `TASK-FE-01` done: export UX text đúng (`.html` thay vì `.docx`)
- [ ] Không có regression trên DOCX/Chat translation

---

*Tạo bởi Claude Sonnet 4.6 · Branch: pdf-ocr-with-format · Commit: 36ad262*
