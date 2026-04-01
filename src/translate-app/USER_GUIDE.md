# Hướng dẫn sử dụng ứng dụng G&J

---

## 1. Giới thiệu

**G&J** là ứng dụng dịch thuật chuyên sâu hỗ trợ xử lý văn bản và tài liệu định dạng DOCX dựa trên công nghệ AI, hỗ trợ hệ điều hành **Windows** và **MacOS**

Sau khi cài đặt thành công, ứng dụng và dữ liệu nằm ở các đường dẫn:
**macOS**:     
- App: /Applications/GnJ.app
- DB: ~/Library/Application Support/TranslateApp/data.db                                                                                                                                                   
- Log: ~/Library/Application Support/TranslateApp/app.log
- Files dịch: ~/Library/Application Support/TranslateApp/files/                                                                                                                                            
                                                                                                                                                                                                             
**Windows**:                                                                                                                                                                                                   
- App: C:\Program Files\GnJ\                                                                                                                                                                               
- DB: C:\Users\<tên>\AppData\Roaming\TranslateApp\data.db                                                                                                                                                  
- Log: C:\Users\<tên>\AppData\Roaming\TranslateApp\app.log
- Files dịch: C:\Users\<tên>\AppData\Roaming\TranslateApp\files\   


---

## 2. Giao diện chương trình
Giao diện được chia thành hai khu vực chính:

### Thanh bên (Sidebar)
Quản lý các phiên làm việc và tùy chỉnh hệ thống, gồm các chức năng:
- **Bắt đầu phiên dịch mới**: ngoài khung nhập liệu ở trang chính, user có thể click vào button **+ Bắt đầu phiên dịch mới** để tạo session dịch mới.
- **Danh sách session**: Các cuộc hội thoại được nhóm theo thời gian (Hôm nay, Hôm qua, Cũ hơn). Bạn có thể Ghim (Pin) các phiên quan trọng lên đầu.
- **Tìm kiếm (Icon Kính lúp)**: Nằm ở phía dưới danh sách hội thoại. Khi nhấp vào, một bảng tìm kiếm sẽ hiện ra cho phép tìm từ khóa trong toàn bộ lịch sử dịch thuật.
- **Cài đặt & Giao diện**: Nằm dưới cùng, cho phép chuyển đổi chế độ Sáng/Tối và cấu hình AI Provider.

### Khung hội thoại (Main Panel)
Nơi diễn ra quá trình tương tác:
- **Thanh nhập liệu**: Nằm dưới cùng để nhập văn bản hoặc đính kèm tệp DOCX. Tại đây bạn có thể chọn ngôn ngữ đích và phong cách dịch.

---

## 3. Các tính năng chính

### Chế độ Bubble:
- Dịch các đoạn văn ngắn, hội thoại nhanh.

### Chế độ Dịch song ngữ (Bilingual Mode)

- **Cơ chế tự động**: Khi user dán một đoạn văn dài (> 2000 ký tự) hoặc văn bản có cấu trúc phức tạp, app sẽ chuyển sang chế độ song ngữ.
- **Đối chiếu trực quan (Hover Sync)**: Khi user di chuột vào một đoạn văn bản ở bên bản dịch, đoạn văn gốc tương ứng sẽ tự động được làm nổi bật với màu sắc đồng bộ, giúp kiểm soát nội dung chính xác mà không bị lạc dòng.
- **Hiển thị tối ưu**: Hỗ trợ hiển thị Markdown, bảng biểu và danh sách một cách chuyên nghiệp.

### Dịch tệp tin (DOCX Translation)
Xử lý tệp tin Word phức tạp lên tới 200 trang:
- **Tương tác**: Bạn có thể **kéo thả** trực tiếp file vào app, hoặc click vào icon attach file để bắt đầu.
- **% hoàn thành**: Trong quá trình dịch, app thể hiện thông tin **Đã dịch x%** giúp người dùng theo dõi chính xác tiến trình dịch
- **Hủy phiên dịch**: Trong trường hợp tiến trình dịch thuật quá lâu do network bị chậm, người dùng có thể hủy phiên dịch bằng cách click vào icon **X** để kết thúc process.
- **Giữ nguyên cấu trúc file**: Sau khi dịch, file xuất ra (Export) sẽ giữ nguyên mọi bảng biểu, hình ảnh và cấu trúc tiêu đề như file gốc.

### Tìm kiếm và Truy xuất thông minh
- **Phạm vi**: Tìm kiếm quét qua cả tiêu đề phiên, nội dung gốc và bản dịch.
- **Highlight thông minh**: Khi bạn click vào một kết quả từ bảng tìm kiếm, app sẽ tự động "nhảy" đến đúng tin nhắn đó với hiệu ứng highlight để bạn nhận diện ngay lập tức vị trí cần tìm.

### Dịch lại (Retranslate)
Nếu kết quả dịch chưa ưng ý, bạn không cần nhập lại:
- Sử dụng nút **Retranslate** (hiện ra khi hover vào tin nhắn) để yêu cầu J dịch lại với một ngôn ngữ hoặc phong cách khác (Thông thường - Casual/Nghiệp vụ - Business/Học thuật - Academic).

### Các chức năng khác
- **Đổi tên session**: user có thể đổi tên bất kỳ sesion từ 2 phía: 
    - tại Sidebar: hover vào icon 3 chấm > Đổi tên, label tên sẽ đổi thành inline text field cho phép nhập tên mới, sau đó Enter để save
    - title của session hiện tại: user click vào title, label tên sẽ đổi thành inline text field cho phép nhập tên mới, sau đó Enter để save
- **Sao chép nội dung bản dịch**: hỗ trợ cho bản dịch ngắn và bản dịch dài. User hover vào khung **bubble view** của trợ lý AI, hoặc **bilingual view**, icon **Sao chép** hiện ra cho phép user copy nhanh văn bản.

---

## 4. Mẹo sử dụng nhanh
- **Phím tắt**: `Enter` để gửi, `Shift + Enter` để xuống dòng.
- **Thu gọn Sidebar**: Nhấn vào icon Menu ở góc trên bên trái để mở rộng tối đa không gian đọc văn bản.
- **Highlight đồng bộ**: Trong chế độ song ngữ, hãy tận dụng việc di chuột để đối soát nhanh các thuật ngữ chuyên môn.

---

## 5. Những nâng cấp trong version 2:
- Fix bug đang có trong version 1
- Hỗ trợ dich file pdf/txt/md/excel
- Session management: hỗ trợ lưu trữ, xóa các session đã hoàn thành
- AI provider management: cấu hình Model AI, theo dõi token tiêu thụ
