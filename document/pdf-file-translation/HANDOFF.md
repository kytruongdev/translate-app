# Handoff — Mistral OCR Integration

> Viết ngày 2025-04-07. Dùng cho session tiếp theo hoặc developer khác pick up.

---

## Tóm tắt nhanh

Đang tích hợp **Mistral OCR** vào production PDF translation pipeline để thay thế GPT-4o làm OCR engine chính. Test thủ công đã pass — chất lượng tốt hơn, cost thấp hơn, không cần render PNG. **Chưa wire vào production.**

---

## Trạng thái hiện tại

### Đã xong

| Việc | File |
|---|---|
| Pipeline PDF structured (render PNG → OCR → translate → HTML) | `internal/controller/file/pipeline_pdf_structured.go` |
| GPT-4o vision OCR | `internal/controller/file/ocr_gpt_vision.go` |
| OCR runner (fallback chain: GPT-4o → sidecar → python) | `internal/controller/file/ocr_runner.go` |
| HTML assembler (text/title/table/figure) | `internal/controller/file/pdf_html_builder.go` |
| Mistral implementation trong **dev preview tool** | `cmd/ocr-gpt-preview/mistral_ocr.go` |
| Test Mistral trên file thực, gen `mistral_preview_v1.html` | output trong `backend/` |
| ARCHITECTURE.md updated đầy đủ | `pdf-file-translation/ARCHITECTURE.md` |

### Chưa làm

Mistral **chưa được wire vào** `internal/` — chỉ tồn tại trong dev tool.

---

## Việc cần làm (theo thứ tự)

### Bước 1 — Tạo `internal/controller/file/ocr_mistral.go`

Port logic từ `cmd/ocr-gpt-preview/mistral_ocr.go` vào internal package.

**Signature cần expose:**
```go
func runMistralOCR(ctx context.Context, pdfPath string, apiKey string, onPage func(done, total int)) (*StructuredOCRResult, error)
```

**Logic cần port:**
- Đọc PDF bytes → base64 encode → gọi `https://api.mistral.ai/v1/ocr` với model `mistral-ocr-latest`
- Parse response: `pages[].markdown` → `markdownToRegions()` → `OCRPage`
- `markdownToRegions()`: heading (`#`/`##`) → `title`, markdown table → `table` (convert sang HTML), image ref → `figure` decorative, text block → `text`
- Gọi `onPage(pageNo, total)` sau mỗi page để emit progress

**Khác với dev tool:**
- Trả về `*StructuredOCRResult` (internal struct), không phải `[]pageResult`
- Không cần `pageFilter` parameter
- Import `mistralOCREndpoint`, `mistralOCRModel` là constants local

**Reference:** `cmd/ocr-gpt-preview/mistral_ocr.go` — hàm `runMistralOCR` và `markdownToRegions`

---

### Bước 2 — Sửa `internal/controller/file/ocr_runner.go`

Thêm Mistral làm tier đầu tiên trong `runStructuredOCR`:

```go
func runStructuredOCR(ctx context.Context, imagePaths []string, pdfPath string, mistralKey string, openAIKey string, onPage func(done, total int)) (*StructuredOCRResult, error) {
    // Tier 1: Mistral (không cần imagePaths)
    if mistralKey != "" {
        return runMistralOCR(ctx, pdfPath, mistralKey, onPage)
    }
    // Tier 2: GPT-4o vision
    if openAIKey != "" {
        return runGPTVisionOCR(ctx, imagePaths, openAIKey, onPage)
    }
    // Tier 3: sidecar binary → python fallback (code hiện tại giữ nguyên)
    ...
}
```

**Lưu ý:** Thêm param `pdfPath string` vào signature — Mistral cần đường dẫn PDF gốc, không cần `imagePaths`.

---

### Bước 3 — Sửa `internal/controller/file/pipeline_pdf_structured.go`

Bước render PNG (`renderPDFToImages`) **chỉ chạy khi không dùng Mistral**:

```go
func (c *controller) runStructuredPDFTranslate(ctx context.Context, p fileTranslateParams, fail func(string)) {
    usingMistral := c.keys.MistralKey != ""

    var imagePaths []string
    var tempDir string

    if !usingMistral {
        // Bước 1: Render PNG (chỉ cần cho GPT-4o và sidecar)
        var err error
        imagePaths, tempDir, err = renderPDFToImages(ctx, p.FilePath)
        if err != nil { ... }
    }

    // Bước 2: OCR
    ocrResult, err := runStructuredOCR(ctx, imagePaths, p.FilePath, c.keys.MistralKey, c.keys.OpenAIKey, onPage)
    ...

    // Bước 3: Figure crops (chỉ khi có PNG)
    figureCrops := map[string]string{}
    if !usingMistral && len(imagePaths) > 0 {
        figureCrops = extractFigureCrops(ocrResult, imagePaths)
    }

    // Bước 4: Xóa temp PNG
    if tempDir != "" {
        _ = os.RemoveAll(tempDir)
    }
    ...
}
```

---

### Bước 4 — Quyết định về Figure Crops khi dùng Mistral

Mistral không có PNG → `figureCrops` sẽ rỗng. Có 2 lựa chọn:

**Option A (đơn giản, chọn trước):** Skip figure crops — figure decorative/informational hiển thị không có ảnh, chỉ có text annotation. `assembleStructuredHTML` đã handle `figureCrops` rỗng gracefully (không crash, chỉ không có `<img>`).

**Option B (đầy đủ hơn):** Sau khi Mistral OCR xong, nếu có figure regions → render PNG **chỉ cho những trang đó** → crop figure → embed Base64. Tốn thêm công implement nhưng output đẹp hơn.

> Gợi ý: Bắt đầu với Option A, ship nhanh, upgrade lên B sau nếu cần.

---

## Các file quan trọng cần đọc

```
backend/
├── config/keys.go                              ← MistralKey, OpenAIKey (gitignored!)
├── cmd/ocr-gpt-preview/
│   ├── main.go                                 ← dev CLI, có --engine mistral flag
│   └── mistral_ocr.go                          ← SOURCE OF TRUTH để port
└── internal/controller/file/
    ├── pipeline_pdf_structured.go              ← main PDF pipeline (sửa bước 3)
    ├── ocr_runner.go                           ← fallback chain (sửa bước 2)
    ├── ocr_gpt_vision.go                       ← GPT-4o implementation (reference)
    ├── ocr_result.go                           ← StructuredOCRResult struct
    ├── pdf_html_builder.go                     ← HTML assembler
    └── render_pdf.go                           ← renderPDFToImages()
```

## Test file

```
file-test/pdf-ocr/XÁC NHẬN CHỦ ĐẦU TƯ CIPUTRA.pdf
```

Chạy preview để verify sau khi integrate:
```bash
cd backend
go run ./cmd/ocr-gpt-preview "../../../file-test/pdf-ocr/XÁC NHẬN CHỦ ĐẦU TƯ CIPUTRA.pdf" mistral_preview_v2.html --engine mistral
```

---

## Lưu ý khác

- `config/keys.go` chứa real API keys, **không được commit** (đã gitignored — verify trước khi push)
- Sau khi tích hợp xong, production flow sẽ là: Mistral OCR → translate bằng OpenAI (vẫn dùng OpenAI cho translation, chỉ OCR là Mistral)
- Architecture đầy đủ: `pdf-file-translation/ARCHITECTURE.md`
