# Business Requirements: Structured PDF Translation (Pro)

## 1. Mục tiêu (Goals)
Nâng cấp khả năng dịch thuật PDF của TranslateApp từ trích xuất văn bản thô (Plain Text) lên trích xuất và bảo toàn cấu trúc (Structured Layout) cho cả PDF kỹ thuật số và PDF bản scan.

## 2. Đối tượng Người dùng (Target Audience)
*   **Người dùng phổ thông (Non-tech):** Cần dịch tài liệu phức tạp một cách đơn giản nhất.
*   **Nhân viên văn phòng/Kế toán:** Cần bản dịch giữ nguyên hàng, cột, bảng biểu và hình ảnh con dấu/chữ ký để đối chiếu.

## 3. Các yêu cầu Nghiệp vụ (Core Requirements)
### 3.1. Đồng bộ Trải nghiệm người dùng (UX Consistency)
*   **Import:** Giữ nguyên luồng hiện tại (Kéo thả/Chọn file -> Chat Bubble hiện tên file & Progress).
*   **Giao diện phản hồi:** Sau khi dịch xong, AI trả về tin nhắn dạng "File Card" (tên file, số trang, số token). Không thêm trình xem HTML phức tạp trong app.
*   **Export:** Người dùng bấm nút **Export** trên File Card để lưu file dịch về máy. 

### 3.2. Bảo toàn Cấu trúc & Nội dung
*   **Bảng biểu (Tables):** Giữ đúng số hàng, số cột và vị trí các ô.
*   **Hình ảnh đặc biệt:** Trích xuất và nhúng lại **Con dấu (Seals)** và **Chữ ký (Signatures)** từ bản gốc vào bản dịch.
*   **Định dạng đầu ra:** File **HTML (.html)** để hiển thị bảng biểu linh hoạt nhất.

### 3.3. Chất lượng & An toàn (Developer Mandates)
*   **Không Hồi quy (No Regression):** Đảm bảo tính năng mới không làm hỏng (break) việc dịch DOCX, PDF Digital (Plain) hay Chat hiện tại.
*   **Đa nền tảng (Cross-platform):** Hoạt động ổn định trên cả macOS và Windows. Có thể test trực tiếp trên Mac trong quá trình phát triển.

## 4. Tiêu chí Thành công (Acceptance Criteria)
*   [ ] Người dùng dịch được file PDF scan có bảng biểu mà không bị lệch hàng/cột.
*   [ ] File HTML xuất ra hiển thị được ảnh con dấu/chữ ký thật từ bản gốc (Base64).
*   [ ] Nút Export hỗ trợ lưu định dạng `.html` một cách tự nhiên.
*   [ ] Các test case cũ (DOCX/Chat) vẫn pass 100%.
