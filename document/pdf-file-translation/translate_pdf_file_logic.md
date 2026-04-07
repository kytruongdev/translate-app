# PDF Translation — Logic & Pipeline

> Tài liệu này mô tả chi tiết pipeline xử lý dịch file PDF cho G & J app.
> Approach: **Option 3** — Rule-based detect + Rule-based clean + AI translate.

---

## 1. Tổng quan

**Input:** File PDF (text layer)
**Output:** File DOCX plain text (không giữ formatting gốc)
**Limitation rõ ràng với user:**
- Bảng biểu, 2 cột có thể mất cấu trúc
- PDF scan không được hỗ trợ
- PDF có mật khẩu không được hỗ trợ

---

## 2. Pipeline chi tiết

### Bước 1: Validate (ReadFileInfo)

```
Input: file path

1. path rỗng           → error "Đường dẫn file không hợp lệ"
2. File không tồn tại  → error "File không tồn tại"
3. File là directory   → error
4. ext ≠ .pdf          → error "Chỉ hỗ trợ PDF"
5. File size > 50MB    → error "File quá lớn (tối đa 50MB)"
6. Open PDF            → lỗi open → error "Không mở được PDF (có thể được bảo vệ bằng mật khẩu)"
7. Đọc số trang        → pageCount > 200 → error "Tệp quá lớn (tối đa 200 trang)"
8. Extract text sample (5 trang đầu)
   → totalChars / pageCount < 50 chars/page
   → error "PDF này có vẻ là bản scan, chưa được hỗ trợ"
9. Tính metadata:
   → charCount, pageCount, estimatedChunks, estimatedMinutes
   → Trả về FileInfo
```

**Ngưỡng detect scan:** < 50 chars/trang trung bình
- PDF bình thường: 1500-3000 chars/trang
- PDF scan: 0-20 chars/trang (chỉ có metadata)
- Edge case: PDF mostly images + ít text → cảnh báo thay vì lỗi cứng nếu > 50 chars/trang

---

### Bước 2: Extract + Rule-based Clean

```
Đọc từng page (1 → N):

  a. Extract text từ page i (rsc.io/pdf)
  b. Rule-based clean cho page đó:
     - Xóa dòng chỉ chứa số (page numbers): /^\s*\d+\s*$/
     - Xóa dòng quá ngắn < 3 chars (noise, artifacts)
     - Fix hyphen line-break: "transla-\ntion" → "translation"
     - Fix soft hyphen (U+00AD): xóa hoặc replace
     - Collapse 3+ newlines → 2 newlines (giữ paragraph break)
     - Trim whitespace thừa đầu/cuối mỗi dòng

  c. Merge cross-page paragraph:
     - Giữ "fragment buffer" = phần text cuối trang chưa kết thúc
       (dấu hiệu: dòng cuối không kết thúc bằng ".", "!", "?", ":")
     - Merge fragment buffer vào đầu trang kế tiếp
     - Flush buffer khi gặp paragraph boundary (double newline)

  d. Append vào text accumulator
```

**Edge cases:**

| Case | Xử lý |
|---|---|
| Trang trống | Skip, không append gì |
| Trang chỉ có ảnh | Extract ra rỗng → skip |
| Dòng rất dài (> 500 chars không có newline) | Giữ nguyên, không can thiệp |
| Unicode đặc biệt (ligature ﬁ, ﬂ) | Replace thành "fi", "fl" |
| Ký tự không decode được | Replace bằng space, không throw error |
| Header/footer lặp lại | Rule-based không detect được → để AI tự bỏ qua khi dịch |

---

### Bước 3: Chunk

```
Text accumulator (~N chars) → chia thành chunks 2500 chars

Rules:
  - Không cắt giữa paragraph (double newline = boundary)
  - Đoạn > 2500 chars → tạo chunk riêng (không chia nhỏ hơn)
  - Đoạn nhỏ → gom lại đến đủ 2500 chars rồi flush

Output: []chunk (ordered)
```

---

### Bước 4: Translate (AI)

```
Với mỗi chunk:
  → Gọi TranslateBatchStream (giống DOCX pipeline)
  → Prompt bình thường: dịch text, không yêu cầu clean thêm
  → Concurrent: MaxBatchConcurrency (4 với OpenAI)
  → Emit file:progress sau mỗi chunk xong
```

**Lưu ý:** Không dùng prompt đặc biệt cho PDF — AI đủ thông minh để dịch
text hơi bẩn mà không cần hướng dẫn thêm. Thêm instruction vào prompt
sẽ tốn token mà không cải thiện đáng kể.

---

### Bước 5: Assemble + Write DOCX

```
Ghép các chunk đã dịch theo thứ tự index
→ Split theo double newline → paragraphs
→ Write từng paragraph vào DOCX (plain, không formatting)
→ Lưu translated.docx vào UserConfigDir/TranslateApp/files/{fileId}/
→ UpdateFile DB (status=done)
→ Emit file:done
```

---

### Bước 6: Error / Cancel

```
Bất kỳ bước nào fail:
  → status = error
  → Emit file:error + translation:error
  → Log FileTranslateFailed (ERROR) với durationMs, fileName

User cancel:
  → ctx bị cancel
  → status = cancelled
  → Emit file:cancelled
  → Log FileTranslateCancelled (WARN)
```

---

## 3. Edge Cases tổng hợp

### 3.1 File-level edge cases

| Edge case | Detect | Xử lý |
|---|---|---|
| PDF scan (no text layer) | charCount < 50/trang | Error: "PDF bản scan chưa được hỗ trợ" |
| PDF có mật khẩu | Open error | Error: "PDF được bảo vệ bằng mật khẩu" |
| PDF > 200 trang | pageCount > 200 | Error: "Tệp quá lớn (tối đa 200 trang)" |
| PDF > 50MB | file size | Error: "Tệp quá lớn (tối đa 50MB)" |
| PDF corrupt/malformed | Open error | Error: "Không mở được file PDF" |
| PDF rỗng (0 trang) | pageCount = 0 | Error: "PDF không có nội dung" |
| PDF mostly images + ít text | charCount thấp nhưng > ngưỡng scan | Cảnh báo trong UI, vẫn dịch phần text có |

### 3.2 Content-level edge cases

| Edge case | Xử lý |
|---|---|
| Paragraph xuyên trang | Fragment buffer merge cross-page |
| Broken words ("transla-\ntion") | Rule-based fix hyphen |
| Page numbers lẫn vào text | Rule-based xóa dòng chỉ có số |
| Header/footer lặp lại | Không detect được → AI tự bỏ qua |
| Bảng biểu | Extract thành text lộn xộn → AI dịch best-effort → cảnh báo trong UI |
| PDF 2 cột | Text bị trộn → AI dịch best-effort → cảnh báo trong UI |
| Ligature (ﬁ, ﬂ, ﬀ) | Replace thành fi, fl, ff trước khi chunk |
| Soft hyphen (U+00AD) | Remove |
| Trang trống | Skip |
| Đoạn văn > 2500 chars | Tạo chunk riêng, không cắt |
| Mixed language (vi + en) | AI detect và dịch phần cần dịch, giữ nguyên phần còn lại |

### 3.3 Runtime edge cases

| Edge case | Xử lý |
|---|---|
| File bị xóa giữa chừng (sau ReadFileInfo) | Revalidate trước khi start pipeline → error |
| API rate limit | Retry với exponential backoff (đã có trong gateway) |
| API timeout | Context cancel → file:error |
| User cancel khi đang dịch | ctx.Done() → dừng pipeline, status=cancelled |
| Disk full khi write DOCX | Error khi write → file:error |
| Chunk dịch ra rỗng | Bỏ qua chunk đó, không append vào output |

---

## 4. UI Considerations

### Cảnh báo hiển thị khi user chọn file PDF:
> ⚠️ *"Bảng biểu và định dạng phức tạp (2 cột, công thức) có thể không được giữ nguyên sau khi dịch."*

### Error messages rõ ràng:
- PDF scan: *"PDF này là bản scan, ứng dụng chưa hỗ trợ loại này."*
- Mật khẩu: *"PDF được bảo vệ bằng mật khẩu, vui lòng gỡ mật khẩu trước."*
- Quá lớn: *"File quá lớn. Tối đa 200 trang hoặc 50MB."*

---

## 5. Phạm vi v1 (không làm)

- PDF scan + OCR
- Giữ nguyên formatting (bảng, heading, bold...)
- PDF 2 cột chính xác
- Công thức toán học
- PDF từ phải sang trái (Arabic, Hebrew)

---

## 6. Tái sử dụng code hiện có

| Component | Tái sử dụng |
|---|---|
| `extractPDFPlain()` | Có sẵn trong `extract.go` |
| Chunking logic | Tái sử dụng từ DOCX pipeline |
| `TranslateBatchStream()` | Giữ nguyên |
| Write DOCX | Tái sử dụng `WriteTranslatedDocx()` hoặc viết plain version |
| `file:progress` events | Giữ nguyên |
| Cancel context | Giữ nguyên |
| DB schema | Không thay đổi (file_type đã có 'pdf' chưa?) |

> **Lưu ý:** Cần kiểm tra lại DB constraint `file_type CHECK (file_type IN ('docx'))` — cần thêm `'pdf'` vào migration mới.
