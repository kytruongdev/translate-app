# User Stories & Technical Tasks: Structured PDF Translation

## User Stories

1.  **[US-01] Dịch PDF scan giữ format:** Là kế toán, tôi muốn upload file PDF scan có bảng số liệu và nhận được file HTML có đúng hàng, đúng cột sau khi Export, để tôi đối chiếu mà không phải reformat lại.

2.  **[US-02] Hình ảnh gốc trong bản dịch:** Là nhân viên văn phòng, tôi muốn bản dịch HTML giữ nguyên con dấu và chữ ký từ tài liệu gốc, để tôi xác nhận tính xác thực mà không cần mở song song 2 file.

3.  **[US-03] Không phải học giao diện mới:** Tôi muốn upload PDF và nhận kết quả giống hệt như khi tôi dịch file Word — cùng một nút, cùng một flow, chỉ khác là file tải về là `.html`.

4.  **[US-04] Lỗi phải rõ ràng:** Khi file không dịch được (có mật khẩu, quá lớn, sidecar lỗi), tôi muốn thấy thông báo lỗi cụ thể — không phải spinner chạy mãi hay crash âm thầm.

---

## Technical Tasks

### Nhóm 0: Sidecar — Layout-Aware OCR (nền tảng của toàn bộ pipeline)

**[TASK-BE-00] Nâng cấp sidecar thành layout-aware, batch processing, với figure classification**
*   Tích hợp `rapid_layout` để phát hiện layout regions per page: `text`, `title`, `table`, `figure`.
*   Với `text`/`title` → `rapidocr` → extract text content.
*   Với `table` → `rapid_table` → HTML table string.
*   Với `figure` → chạy **Figure Classifier**:
    1.  Check whitelist (dùng `cv2`): có phải logo / seal / signature không? Nếu có → `figure_type = "decorative"`, chỉ trả bbox.
    2.  Nếu không → chạy `rapidocr` trên vùng crop: có text không? Nếu có → `figure_type = "informational"`, trả bbox + `text_lines`. Nếu không → `figure_type = "decorative"`, chỉ trả bbox.
*   **Whitelist heuristics ban đầu (cv2):**
    *   Seal: circularity > 0.7 AND aspect ratio gần 1:1 AND dominant color đỏ/tím.
    *   Signature: bbox ở dưới trang (y_min > 65% height) AND ít text AND wide aspect ratio.
    *   Logo: bbox ở top 20% trang AND diện tích < 8% trang.
*   **Batch mode:** Một invocation cho tất cả pages, model load một lần.
*   Output JSON tuân theo contract trong `ARCHITECTURE.md` (bao gồm `figure_type` và `text_lines`).

**[TASK-DEV-00] Sidecar build automation**
*   Makefile target `make sidecar-mac`: build `paddleocr-darwin-arm64` bằng PyInstaller, output vào `bin/`.
*   Makefile target `make sidecar-win`: cross-compile hoặc hướng dẫn build trên Windows, output `paddleocr-windows-amd64.exe` vào `bin/`.
*   Cập nhật `.spec` file để include `rapid_layout`, `rapid_table`, và `cv2`.

---

### Nhóm 1: Go Orchestration — PDF rendering, gọi sidecar & parse kết quả

**[TASK-BE-01] Thay PDF renderer bằng go-fitz (MuPDF)**
*   Thêm dependency `github.com/gen2brain/go-fitz`.
*   Viết `renderPDFToImages(ctx, pdfPath, dpi) ([]string, string, error)`:
    *   Tạo temp dir → render từng trang → `page-0001.png`, `page-0002.png`, ...
    *   Trả về danh sách PNG paths và temp dir path.
    *   Propagate context (support cancel giữa các trang).
*   Xóa `findPDFRenderer()`, `pdftopng/pdftoppm` logic trong `ocr.go` sau khi verify pipeline mới hoạt động.
*   **CGo note:** Cập nhật Makefile và CI build để handle CGo + libmupdf trên cả macOS và Windows.

**[TASK-BE-02] Go sidecar loader (multi-OS)**
*   Detect OS (Darwin/Windows) → chọn đúng binary từ `bin/`.
*   Search order: bundled next to exe → `bin/` relative to cwd (dev) → error nếu không tìm thấy.
*   Nếu binary không tìm thấy: return error rõ ràng — không fallback âm thầm.
*   Gọi sidecar: `paddleocr-darwin-arm64 page-0001.png page-0002.png ... page-000N.png`.
*   Parse JSON stdout → `StructuredOCRResult` struct.
*   Propagate context (support cancel).

**[TASK-BE-03] Parse và validate structured OCR output**
*   Unmarshal JSON → `StructuredOCRResult` — cập nhật struct để có `FigureType` và `TextLines` fields.
*   Validate: page count khớp với số PNG đã render; bbox hợp lệ (x_min < x_max, y_min < y_max).
*   Log warning (không error) nếu một page không có region nào — tiếp tục các trang còn lại.

---

### Nhóm 2: HTML Assembly — Ghép kết quả thành HTML

**[TASK-BE-04] Image crop và Base64 embedding cho figure regions**
*   Với mỗi `figure` region: dùng `disintegration/imaging` crop vùng `bbox` từ PNG tương ứng → encode Base64.
*   `figure_type = "decorative"` → `<img>` only.
*   `figure_type = "informational"` → `<img>` + `<div class="figure-translated-text" data-translate="figure">` chứa raw `text_lines` join bằng newline (sẽ được dịch ở bước sau).
*   Bbox là pixel coordinates trên PNG 200 DPI — crop trực tiếp, không scale.
*   **Sau khi tất cả figure crops xong → gọi `os.RemoveAll(tempDir)`.**

**[TASK-BE-05] HTML assembler — pre-translation skeleton**
*   Duyệt pages → regions theo thứ tự → build HTML:
    *   `text`/`title` → `<p data-translate="text">content</p>`.
    *   `table` → `<div data-translate="table">...html...</div>`.
    *   `figure` (decorative) → `<div class="figure-block"><img ...></div>`.
    *   `figure` (informational) → `<div class="figure-block"><img ...><div data-translate="figure">raw text lines</div></div>`.
*   Update `pdf_html_builder.go` để xử lý `figure_type` và HTML structure mới.

---

### Nhóm 3: AI Translation — Dịch nội dung

**[TASK-BE-06] Chiến lược chunking và prompt cho 3 loại segment**

*   **Text segment** (`text`/`title`): gom thành plain text chunks ≤5000 chars, không cắt giữa region. Dịch bằng `TranslateStream`.
*   **Table segment**: HTML table string nguyên vẹn, 1 request per table. Prompt: *"Translate only text inside `<td>` and `<th>`. Do NOT modify HTML structure."* Nếu table HTML > 8000 chars → log warning, gửi nguyên (không cắt table).
*   **Informational figure text**: danh sách text lines từ `text_lines`, join thành plain text, dịch như text thường. Kết quả điền vào annotation block bên dưới ảnh.
*   **Concurrency:** Text chunks + figure text → `MaxBatchConcurrency` (4). Tables → sequential.

**[TASK-BE-07] Structured PDF pipeline (thay thế `runPDFTranslate`)**
*   Thêm `runStructuredPDFTranslate` trong `pipeline_pdf.go`.
*   Flow đầy đủ:
    1.  Render tất cả trang → PNG temp dir (TASK-BE-01, go-fitz).
    2.  Emit `file:progress {percent: 10}`.
    3.  Gọi sidecar (TASK-BE-02) → `StructuredOCRResult`.
    4.  Emit `file:progress {percent: 40}`.
    5.  Parse + validate OCR output (TASK-BE-03).
    6.  Crop figure regions → Base64; xóa temp PNG dir (TASK-BE-04).
    7.  Build pre-translation HTML skeleton (TASK-BE-05).
    8.  Detect source lang từ text content. Emit `file:source`.
    9.  Translate 3 loại segments (TASK-BE-06), emit `file:progress` per chunk (range 40–95%).
    10. Reassemble final HTML.
    11. Ghi `translated.html`. Update DB: `output_format = 'html'`, `status = done`.
    12. Emit `translation:done` + `file:done`.
*   **Thay thế:** Trong `pipeline.go`, route mọi `file_type = pdf` sang `runStructuredPDFTranslate`.
*   Error handling: bất kỳ bước nào fail → `status = error` → emit `file:error`.
*   Cancel: check `ctx.Done()` sau mỗi bước lớn. Nếu cancel sau bước 6 → temp dir đã xóa rồi, không cần xóa lại.

---

### Nhóm 4: Export & Frontend

**[TASK-BE-08] Export handler cho HTML**
*   Cập nhật `ExportFile` trong `controller/file/new.go`:
    *   Đọc `output_format` từ DB record.
    *   Nếu `output_format = html` → `SaveDialog` với filter `*.html`, default filename `{tên_gốc}_translated.html`.
    *   Nếu `output_format = docx` → giữ nguyên behavior hiện tại.
*   Trả về save path → FE hiển thị thông báo thành công.

**[TASK-FE-01] Frontend: Export button nhận diện định dạng HTML**
*   FE đọc `output_format` từ file record (cần expose qua bridge types nếu chưa có).
*   Export button label/icon không cần thay đổi (giữ "Export"), nhưng phần mở rộng file trong thông báo thành công hiển thị đúng (`.html` thay vì `.docx`).

---

### Nhóm 5: Database & Infrastructure

**[TASK-DB-01] Migration 008** *(đã có)*
*   File `008_pdf_structured_support.sql` đã tạo, thêm `output_format TEXT NOT NULL DEFAULT 'docx'`.
*   Cần: cập nhật SQLC queries và repository layer để đọc/ghi `output_format`.
*   Cần: expose `output_format` trong bridge DTO (`FileResult` hoặc tương đương).

**[TASK-DEV-01] Testing**

*   **Regression (automated):** Chạy lại test suite DOCX/Chat hiện tại, verify pass 100%.
*   **Structured PDF (manual, Happy path):**
    *   File test 1: PDF digital có bảng biểu → verify HTML table rows/cols đúng.
    *   File test 2: PDF scan (chụp từ máy quét) → verify text readable, table còn structure.
    *   File test 3: PDF có con dấu/chữ ký → verify ảnh xuất hiện đúng vị trí trong HTML.
*   **Sad path (manual):**
    *   PDF có mật khẩu → verify error message hiển thị.
    *   Cancel giữa chừng → verify status = cancelled, không treo UI.
    *   Xóa sidecar binary → verify error message rõ ràng.

---

## Thứ tự Thực hiện (Execution Order)

```
Giai đoạn 1 — Sidecar foundation
  TASK-BE-00  Nâng cấp sidecar (rapid_layout + rapid_table + figure classifier + batch)
  TASK-DEV-00 Build automation (.spec update + Makefile targets)

  [Checkpoint: Gọi sidecar thủ công với vài PNG test, verify JSON output đúng contract
   — đặc biệt kiểm tra figure_type + text_lines cho ảnh chart vs logo]

Giai đoạn 2 — Go rendering + orchestration + HTML assembly (không cần AI)
  TASK-BE-01  Thay PDF renderer bằng go-fitz
  TASK-BE-02  Go sidecar loader
  TASK-BE-03  Parse + validate OCR output (struct update)
  TASK-BE-04  Image crop + Base64 + temp dir cleanup
  TASK-BE-05  HTML skeleton assembler (update pdf_html_builder.go)
  TASK-DB-01  Migration 008 + SQLC + repository + bridge DTO

  [Checkpoint: Run pipeline PDF → HTML (không dịch), mở file trong browser:
   — text/title regions hiển thị đúng thứ tự
   — table regions có đúng hàng/cột
   — decorative figures hiện ảnh gốc, không có annotation
   — informational figures hiện ảnh gốc + raw text block bên dưới
   — temp PNGs đã bị xóa sau pipeline]

Giai đoạn 3 — AI translation + full pipeline
  TASK-BE-06  Chunking strategy + prompts cho 3 loại segment
  TASK-BE-07  Structured pipeline end-to-end (thay thế runPDFTranslate)

  [Checkpoint: Dịch end-to-end với file test đủ loại region, verify:
   — text dịch đúng nghĩa
   — table HTML giữ nguyên cấu trúc
   — informational figure có annotation dịch đúng bên dưới ảnh]

Giai đoạn 4 — Export + FE + validation
  TASK-BE-08  Export HTML handler
  TASK-FE-01  FE export button nhận diện HTML
  TASK-DEV-01 Testing (regression + happy path + sad path)
```

**Dependency notes:**
*   Giai đoạn 2 hoàn toàn phụ thuộc vào TASK-BE-00 (contract sidecar phải ổn định).
*   TASK-BE-07 phụ thuộc vào tất cả TASK-BE-01 → BE-06.
*   TASK-BE-08 và FE-01 phụ thuộc vào DB-01 (`output_format` phải có trong DTO).
*   TASK-DEV-01 là bước cuối, phụ thuộc vào tất cả.
