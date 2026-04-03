# User Stories & Technical Tasks: Structured PDF Translation

## User Stories (Đứng từ góc độ Người dùng)

1.  **[US-01] Dịch PDF Scan giữ format:** Là một người dùng "no-tech", tôi muốn dịch một file PDF scan có bảng biểu và nhận được một file HTML có cấu trúc tương đương sau khi bấm nút **Export**, để tôi dễ dàng đối chiếu số liệu.
2.  **[US-02] Bảo toàn Con dấu & Chữ ký:** Là một nhân viên văn phòng, tôi muốn bản dịch HTML giữ nguyên ảnh con dấu và chữ ký từ bản gốc, để tôi yên tâm về tính xác thực của tài liệu.
3.  **[US-03] Trải nghiệm đồng bộ:** Tôi muốn quy trình upload và dịch file diễn ra giống hệt như khi tôi dịch file Word, không cần làm quen với giao diện mới.

---

## Technical Task List (Đứng từ góc độ Developer)

### Nhóm 1: Sidecar OCR & Multi-OS Support
*   **[TASK-BE-01] Multi-OS Sidecar Loader:** 
    *   Phát hiện OS (Darwin/Windows) trong Go.
    *   Tải đúng binary PaddleOCR từ thư mục `bin/`.
*   **[TASK-BE-02] Structured OCR Output:** 
    *   Xử lý kết quả JSON từ PaddleOCR để lấy tọa độ vùng (Table, Seal, Signature).

### Nhóm 2: Image & HTML Logic (Backend)
*   **[TASK-BE-03] Imaging Module (Crop & Base64):** 
    *   Dùng `disintegration/imaging` để cắt ảnh dấu/chữ ký từ trang PDF gốc.
    *   Chuyển đổi ảnh sang chuỗi Base64 để nhúng vào HTML.
*   **[TASK-BE-04] Export HTML Integration:** 
    *   Cập nhật `ExportFile` trong `controller/file/new.go` để hỗ trợ filter `.html`.
    *   Đảm bảo `SaveDialog` hiển thị đúng định dạng Web Page cho file PDF Scan.

### Nhóm 3: AI Translation & Safety (Gateway)
*   **[TASK-BE-05] HTML Translation Prompt:** 
    *   Thiết kế prompt dịch cho GPT-4o-mini (giữ nguyên cấu trúc HTML tag).
*   **[TASK-BE-06] Structured Pipeline (No-Regression):** 
    *   Thêm `runStructuredPDFTranslate` trong `pipeline_pdf.go`.
    *   Đảm bảo tách biệt hoàn toàn với logic dịch DOCX và PDF Plain hiện có.

### Nhóm 4: Database & Infrastructure
*   **[TASK-DB-01] Safe Migration (008):** 
    *   Thêm cột `output_format` vào bảng `files`.
    *   Cập nhật SQLC và Repository Layer.
*   **[TASK-DEV-01] Regression Testing Suite:** 
    *   Chạy test tự động cho DOCX/Chat trên macOS.
    *   Thực hiện test thủ công việc dịch PDF Scan trên macOS.

---

## Thứ tự Ưu tiên (Execution Order)
1.  **Giai đoạn 1 (DB & Infrastructure):** Hoàn thành TASK-DB-01 và TASK-BE-01.
2.  **Giai đoạn 2 (Image & OCR):** Hoàn thành TASK-BE-02 và TASK-BE-03 (Trích xuất được cấu trúc và ảnh dấu).
3.  **Giai đoạn 3 (AI & Pipeline):** Hoàn thành TASK-BE-05 và TASK-BE-06 (Dịch mã HTML và lắp ghép).
4.  **Giai đoạn 4 (Export & Validation):** Hoàn thành TASK-BE-04 và TASK-DEV-01.
