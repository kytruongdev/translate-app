# Business Requirements: Structured PDF Translation

## 1. Mục tiêu (Goals)

**Thay thế** pipeline dịch PDF hiện tại (Tesseract plain text → DOCX) bằng một pipeline mới có khả năng bảo toàn cấu trúc tài liệu và xuất ra HTML.

**Trường hợp phải xử lý tốt nhất:** PDF bản scan (không có text layer). Đây là worst case — nếu scan PDF hoạt động tốt, digital PDF chắc chắn cũng tốt.

**Không còn dual pipeline, không có routing, không có lựa chọn từ phía user.** Mọi file PDF đều đi qua một luồng duy nhất. Lý do: plain text pipeline (pdftotext) không thể giữ bảng biểu và hình ảnh dù là PDF digital — nên chia pipeline sẽ cho output chất lượng không đồng đều. Structured pipeline (render → RapidOCR) xử lý được cả hai loại.

---

## 2. Đối tượng Người dùng (Target Audience)

*   **Nhân viên văn phòng / Kế toán:** Cần bản dịch giữ nguyên bảng biểu (hàng, cột, số liệu) và các hình ảnh đặc thù của tài liệu gốc (con dấu, chữ ký) để đối chiếu.
*   **Người dùng phổ thông:** Cần kết quả tốt mà không cần hiểu gì về công nghệ bên dưới.

---

## 3. Các yêu cầu Nghiệp vụ (Core Requirements)

### 3.1. Trải nghiệm người dùng — đơn giản, không thay đổi
*   **Import:** Giữ nguyên luồng hiện tại — kéo thả hoặc chọn file → chat bubble hiện tên file + progress ring.
*   **Không có lựa chọn thêm:** User không cần biết "scan hay digital", không cần chọn mode. App tự xử lý.
*   **Output:** Sau khi dịch xong, File Card hiện trong chat (tên file, số trang, số token). User bấm **Export** → SaveDialog → lưu `.html` về máy.
*   **Không có HTML viewer trong app:** App không tích hợp trình xem HTML. User mở bằng browser.

### 3.2. Chất lượng bản dịch — output phải đáp ứng đủ 4 tiêu chí

1. **Dịch chính xác:** Nội dung được dịch đúng ngữ nghĩa, đúng style (casual/business/academic).
2. **Bảng biểu nguyên vẹn:** HTML output giữ đúng số hàng, số cột, nội dung đúng ô.
3. **Visual elements được xử lý đúng theo loại:**
   *   **Decorative figures** (logo, con dấu, chữ ký, ảnh minh họa thuần túy): crop nguyên từ bản gốc, nhúng vào HTML dưới dạng Base64, không dịch bất kỳ text nào bên trong — text trong ảnh là phần của visual identity, không phải nội dung độc lập.
   *   **Informational figures** (chart, biểu đồ, diagram, sơ đồ): crop nguyên từ bản gốc và nhúng vào HTML, đồng thời extract text bên trong và dịch, hiển thị dưới dạng annotation block bên dưới ảnh gốc. User thấy được cả ảnh gốc lẫn nội dung đã dịch.
   *   **Cơ chế phân loại:** Whitelist-based — logo/seal/signature được nhận diện bằng heuristics (hình dạng, vị trí, màu sắc) và bỏ qua dịch. Mọi figure không nằm trong whitelist mà có text đều được coi là informational. Whitelist được thiết kế để mở rộng dần theo thực tế sử dụng.
4. **Thứ tự nội dung đúng:** Thứ tự đọc (top-to-bottom, left-to-right) được bảo toàn theo trang.

### 3.3. Không hồi quy (No Regression)
*   DOCX pipeline không bị thay đổi.
*   Chat translation không bị thay đổi.
*   Export DOCX (cho file Word cũ) không bị thay đổi.

### 3.4. Đa nền tảng
*   macOS arm64 + Windows amd64.
*   Sidecar binary được đóng gói sẵn trong app, không yêu cầu user cài thêm.

---

## 4. Tiêu chí Thành công (Acceptance Criteria)

### Happy path
*   [ ] Upload PDF scan có bảng → HTML output có đúng số hàng, đúng số cột.
*   [ ] Upload PDF có decorative figures (logo, con dấu, chữ ký) → HTML chứa ảnh crop đúng vùng, không có text nào bên trong bị tách ra hay dịch riêng.
*   [ ] Upload PDF có informational figures (chart, biểu đồ) → HTML chứa ảnh gốc + annotation block bên dưới với text đã dịch.
*   [ ] Bấm Export → SaveDialog mở với filter `.html` → lưu file thành công.
*   [ ] Nội dung text được dịch đúng nghĩa (manual review với file test chuẩn).
*   [ ] Thứ tự nội dung (paragraph, bảng, ảnh) trong HTML khớp với thứ tự trong PDF gốc.

### Sad path
*   [ ] PDF có mật khẩu → error message rõ ràng, không crash.
*   [ ] PDF > 200 trang hoặc > 50MB → error message trước khi bắt đầu xử lý.
*   [ ] Sidecar binary không tìm thấy → error message rõ ràng, gợi ý reinstall app.
*   [ ] Sidecar crash giữa chừng → `file:error` được emit, status DB = `error`.
*   [ ] User cancel → `file:cancelled` được emit, status DB = `cancelled`.
*   [ ] API rate limit / timeout → retry với backoff; nếu vẫn fail → `file:error`.

### Regression
*   [ ] Dịch file DOCX vẫn hoạt động bình thường.
*   [ ] Dịch text qua chat vẫn hoạt động bình thường.
*   [ ] Export DOCX (file Word cũ) vẫn hoạt động bình thường.
