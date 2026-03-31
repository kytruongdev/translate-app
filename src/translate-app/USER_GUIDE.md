# Hướng dẫn sử dụng G & J (GnJ)

G & J là ứng dụng dịch thuật chuyên sâu hỗ trợ xử lý văn bản và tài liệu định dạng DOCX dựa trên công nghệ AI.

---

## 1. Giới thiệu
Ứng dụng sử dụng mô hình ngôn ngữ lớn để thực hiện việc chuyển ngữ. Trong giao diện:
- **Icon G (Hồng)**: Đại diện cho nội dung do người dùng nhập vào.
- **Icon J (Xanh)**: Đại diện cho nội dung phản hồi từ trợ lý AI.

---

## 2. Giao diện chương trình

Giao diện được chia thành hai khu vực chính:

### Thanh bên (Sidebar)
Quản lý các phiên làm việc (Sessions):
- **Tìm kiếm**: Thanh tìm kiếm nằm ở trên cùng để truy xuất nội dung cũ.
- **Danh sách phiên**: Hiển thị các cuộc hội thoại được nhóm theo thời gian.
- **Chức năng phụ**: Ghim (Pin) phiên quan trọng, đổi tên phiên, hoặc lưu trữ (Archive) các phiên không còn sử dụng.
- **Cài đặt & Giao diện**: Nút chuyển đổi chế độ Sáng/Tối và tùy chỉnh hệ thống ở dưới cùng.

### Khung hội thoại (Main Panel)
Nơi diễn ra quá trình dịch thuật:
- **Tiêu đề**: Hiển thị tên phiên dịch hiện tại.
- **Danh sách tin nhắn**: Hiển thị nội dung gốc và nội dung dịch theo trình tự thời gian.
- **Thanh nhập liệu**: Nơi nhập văn bản, đính kèm tệp, chọn ngôn ngữ đích và phong cách dịch (Casual/Business/Academic).

---

## 3. Các tính năng chính

### Chế độ Dịch song ngữ (Bilingual Mode)
Đây là chế độ hiển thị song song bản gốc và bản dịch để người dùng dễ dàng đối chiếu:
- **Cơ chế kích hoạt**: App tự động chuyển sang chế độ này khi văn bản nhập vào có độ dài trên 2000 ký tự hoặc có cấu trúc phức tạp.
- **Tính năng đối chiếu (Hover Sync)**: Khi di chuột vào một đoạn văn ở cột bản dịch, đoạn văn tương ứng ở cột bản gốc sẽ được làm nổi bật (highlight). Tính năng này giúp kiểm soát độ chính xác của từng phân đoạn văn bản.

### Dịch tệp tin (DOCX Translation)
Xử lý trực tiếp các tệp tin tài liệu:
- **Định dạng**: Hỗ trợ tệp Microsoft Word (`.docx`).
- **Giữ nguyên định dạng**: Bản dịch sau khi hoàn thành sẽ giữ nguyên các yếu tố như font chữ, bảng biểu, hình ảnh và tiêu đề từ file gốc.
- **Giới hạn**: Hệ thống hỗ trợ tệp có độ dài tối đa 200 trang.
- **Xuất tệp**: Sau khi dịch xong, người dùng có thể tải file về máy thông qua nút **Export**.

### Tìm kiếm toàn cục (Global Search)
Chức năng tìm kiếm cho phép truy lục thông tin nhanh chóng:
- **Phạm vi**: Tìm kiếm từ khóa trong tiêu đề phiên, nội dung văn bản gốc và nội dung đã dịch.
- **Tương tác**: Kết quả tìm kiếm hiển thị dưới dạng đoạn trích (snippet). Nhấp vào kết quả sẽ dẫn trực tiếp đến vị trí tin nhắn đó trong phiên hội thoại tương ứng.

### Dịch lại và Chỉnh sửa phong cách (Retranslate)
Người dùng có thể yêu cầu AI dịch lại một nội dung đã có:
- **Tùy chọn**: Có thể đổi sang ngôn ngữ đích khác hoặc thay đổi phong cách dịch (Thân mật, Công việc, hoặc Học thuật) cho cùng một nội dung gốc.
- **Ứng dụng**: Áp dụng được cho cả văn bản chat thông thường và nội dung tệp tin đã tải lên.

---

## 4. Cài đặt hệ thống
Người dùng có thể tùy chỉnh các thông số kỹ thuật:
- **AI Provider**: Lựa chọn nhà cung cấp (mặc định là OpenAI).
- **Model**: Chọn các phiên bản mô hình khác nhau (ví dụ: GPT-4o, GPT-4o mini) để cân bằng giữa tốc độ và chất lượng.
- **Theme**: Chuyển đổi giao diện theo sở thích cá nhân hoặc theo cài đặt hệ thống.
