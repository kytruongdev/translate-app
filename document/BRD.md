# Tài liệu Yêu Cầu Nghiệp Vụ (BRD)
## Ứng dụng Dịch Thuật AI — Translate App

**Phiên bản:** 1.0
**Ngày:** 2026-03-19
**Nguồn phân tích:** `mockup/mockup.html`

---

## 1. Tổng Quan Ứng Dụng

### 1.1 Mô Tả Ứng Dụng

Translate App là một ứng dụng dịch thuật AI dạng chat (chat-based translation assistant), cho phép người dùng dịch văn bản, file tài liệu và trò chuyện về ngôn ngữ/thuật ngữ thông qua giao diện hội thoại. Ứng dụng có phong cách thiết kế gần gũi với các trợ lý AI hiện đại như Gemini và ChatGPT.

### 1.2 Mục Tiêu

- Cung cấp công cụ dịch thuật AI chất lượng cao, hỗ trợ nhiều phong cách dịch (phổ thông, học thuật, kinh doanh).
- Cho phép dịch văn bản ngắn lẫn tài liệu dài (file PDF, DOCX, TXT) trong cùng một giao diện chat.
- Lưu trữ lịch sử các phiên dịch thuật để người dùng tra lại dễ dàng.
- Hỗ trợ chế độ Chat để người dùng hỏi-đáp về thuật ngữ, ngữ pháp, ngữ cảnh bên cạnh chức năng dịch thuần túy.
- Cho phép export kết quả dịch ra định dạng PDF hoặc DOCX.

### 1.3 Đối Tượng Người Dùng

| Nhóm | Mô tả |
|---|---|
| Chuyên viên văn phòng | Cần dịch email, báo cáo, hợp đồng sang tiếng Anh hoặc ngôn ngữ khác |
| Kế toán / Tài chính | Dịch tài liệu thuế, báo cáo tài chính |
| Pháp lý | Dịch hợp đồng, văn bản pháp lý, thuật ngữ chuyên ngành |
| Học thuật / Nghiên cứu | Dịch bài báo khoa học, thuyết trình |
| Người dùng phổ thông | Dịch tin nhắn, email, hội thoại hàng ngày |

---

## 2. Các Tính Năng Chính

### 2.1 Quản Lý Phiên Dịch (Session Management)

**Tạo phiên mới**
- Người dùng bấm nút "Bắt đầu phiên dịch mới" ở đầu sidebar để khởi tạo một cuộc hội thoại dịch thuật mới.
- Khi tạo phiên mới, trang bắt đầu (Start View) hiện ra với chữ chào và các gợi ý nhanh.

**Danh sách lịch sử phiên**
- Sidebar hiển thị tất cả các phiên đã tạo, được nhóm theo thời gian:
  - **Ghim:** Phiên được ghim sẽ nằm ở nhóm riêng phía trên cùng, có nền phân biệt.
  - **Hôm nay:** Các phiên tạo trong ngày.
  - **N ngày trước:** Nhóm theo số ngày trước đó (1 ngày trước, 2 ngày trước,...).
  - **Ngày cụ thể:** Ví dụ "Chủ nhật, 15.3.2026" cho các phiên từ ngày thứ 3 trở đi.
- Tiêu đề phiên được tự động đặt theo nội dung và hiển thị dạng text ellipsis khi quá dài.

**Chọn/Mở phiên**
- Click vào bất kỳ session item nào trong sidebar để chuyển sang xem nội dung phiên đó.
- Phiên đang xem được đánh dấu `active` (nền đậm hơn).

**Ghim / Bỏ ghim phiên**
- Mỗi phiên có menu 3 chấm (dấu `...`) xuất hiện khi hover.
- Menu có tùy chọn **Ghim** / **Bỏ ghim** để cố định phiên quan trọng lên đầu sidebar.

**Đổi tên phiên**
- Từ menu 3 chấm, chọn **Đổi tên** để chỉnh sửa inline ngay trong sidebar (không dùng popup/prompt).
- Nhấn Enter để lưu, Escape để hủy.

**Đổi tên phiên từ tiêu đề chat**
- Tiêu đề phiên hiển thị lớn ở đầu khung chat, click vào để chỉnh sửa inline.
- Nhấn Enter để lưu, Escape để hủy, blur ngoài để tự lưu.

---

### 2.2 Nhập Liệu và Gửi Yêu Cầu

**Ô nhập liệu (Chat Input)**
- Textarea đa dòng, hỗ trợ nhập văn bản tự do.
- Placeholder: "Nhập hoặc dán văn bản, hoặc đính kèm file để dịch..."
- Gửi bằng nút Send hoặc phím **Enter** (Shift+Enter để xuống dòng).

**Đính kèm file**
- Nút đính kèm (icon paperclip) mở file picker.
- Hỗ trợ định dạng: `.pdf`, `.docx`, `.txt`.
- File được hiển thị dưới dạng bubble riêng trong feed, kèm tên file và dung lượng.

**Chọn kiểu dịch (Translation Style)**
- Chip **"Casual"** (mặc định) nằm trong ô nhập liệu, click để mở popover chọn:
  - **Casual** (Phổ thông)
  - **Business** (Kinh doanh)
  - **Academic** (Học thuật)
- Lựa chọn áp dụng cho lần gửi tiếp theo, nhãn chip cập nhật ngay.

**Chọn chế độ gửi (Send Mode)**
- Chip **"Dịch"** / **"Chat"** nằm trong ô nhập liệu, click để mở popover chuyển đổi:
  - **Dịch:** AI thực hiện dịch văn bản.
  - **Chat:** AI phản hồi dạng hội thoại (hỏi đáp thuật ngữ, giải thích ngữ pháp...).
- Chuyển đổi bằng toggle switch kiểu Material 3. Ngoài ra còn hỗ trợ chuyển đổi bằng phím tắt.

---

### 2.3 Trang Bắt Đầu (Start View)

- Hiện ra khi mở app lần đầu hoặc bấm "Bắt đầu phiên dịch mới".
- **Chữ chào động:** Chữ "Hi there" / "Xin chào" xuất hiện theo kiểu từng ký tự fade-in, sau đó xóa dần và lặp lại (animation loop xen kẽ hai ngôn ngữ).
- **Văn bản mô tả ngắn:** Hướng dẫn người dùng gõ/dán văn bản, đính kèm file hoặc thử gợi ý.
- **Gợi ý nhanh (Quick Suggestions):** 4 chip gợi ý sẵn có để người dùng thử nhanh:
  1. **Dịch câu mẫu** — Dịch một câu tiếng Việt sang tiếng Anh.
  2. **Giải thích thuật ngữ** — Giải thích thuật ngữ pháp lý "force majeure".
  3. **Email trang trọng** — Dịch formal cho email gửi đối tác.
  4. **Viết lại thân thiện** — Viết lại câu theo phong cách thân thiện dùng trong chat.
- Click chip gợi ý: điền nội dung vào ô input và chuyển sang khung chat.

---

### 2.4 Khung Chat (Chat View)

**Feed tin nhắn**
- Hiển thị lịch sử tin nhắn cuộc hội thoại theo dạng feed cuộn dọc.
- Có animation `msg-enter` (fade + slide up) khi tin nhắn xuất hiện, mỗi tin delay lần lượt.

**Tin nhắn người dùng (User Message)**
- Hiển thị căn phải, avatar "U" gradient hồng.
- Bubble màu gradient nhạt tông hồng.
- Nhãn phụ: giờ phút · ngôn ngữ nguồn.

**Tin nhắn dịch ngắn (Short Translation)**
- AI trả về bubble đơn giản, avatar "T" gradient xanh.
- Bubble màu gradient nhạt tông xanh lá/xanh dương.
- Nhãn phụ: giờ phút · ngôn ngữ đích · kiểu dịch.
- **Hover actions:** Khi hover vào bubble, hiện nổi 2 nút:
  - **Dịch lại** (icon refresh)
  - **Copy** (icon copy)

**Upload file từ người dùng (File Bubble)**
- Hiển thị bubble riêng dạng file attachment căn phải.
- Chứa icon tài liệu, tên file, giờ phút và dung lượng.

**Upload văn bản dài (Text Upload Bubble)**
- Khi người dùng dán đoạn văn dài, hiển thị dạng bubble nhỏ gọn với icon tài liệu và nhãn "Văn bản đã dán".

**Thẻ dịch song ngữ (Translation Card)**
- Dùng cho kết quả dịch file hoặc văn bản dài.
- **Topbar của card** gồm 3 vùng (grid 3 cột):
  - **Trái:** Tiêu đề (tên file hoặc trống).
  - **Giữa:** Toggle chế độ xem (Song ngữ / Chỉ bản dịch / Chỉ nguồn).
  - **Phải:** Nhóm action icons (ẩn khi không hover, hiện khi hover/focus vào card).
- **Nội dung song ngữ (Bilingual View):**
  - Chia 2 cột: Nguồn (trái, tông xanh nhạt) và Bản dịch (phải, tông hồng nhạt).
  - Header cột nhỏ: "Nguồn · Tiếng Việt" / "Bản dịch · Tiếng Anh".
  - Nội dung hỗ trợ heading (h3), đoạn văn, danh sách có thứ tự/không thứ tự.
  - Scrollbar xuất hiện khi hover vào panel.
- **Chế độ xem:**
  - **Song ngữ:** Hiển thị cả 2 cột (mặc định).
  - **Chỉ bản dịch:** Ẩn cột nguồn, hiện 1 cột bản dịch toàn bộ.
  - **Chỉ nguồn:** Ẩn cột bản dịch, hiện 1 cột nguồn toàn bộ.
- **Nhãn footer:** Giờ phút · kiểu dịch, hiển thị bên dưới card.

**Action Icons trên Translation Card** (hiện khi hover/focus vào card):
- **Export** — Mở popover chọn định dạng export.
- **Dịch lại** — Mở popover cấu hình và kích hoạt dịch lại.
- **Copy** — Copy toàn bộ bản dịch vào clipboard.
- **Xem toàn màn hình** (luôn hiện) — Mở modal fullscreen để đọc thoải mái.

---

### 2.5 Tính Năng Dịch Lại (Retranslate)

- Kích hoạt từ nút "Dịch lại" trên bubble ngắn hoặc action icon của translation card.
- Mở **popover "Dịch lại"** nổi gần nút bấm (kiểu ChatGPT, có mũi tên chỉ nguồn).
- Popover cho phép cấu hình:
  - **Model online** (dropdown): GPT-4 Translate, Claude Translate, v.v.
  - **Model offline** (dropdown): Local Model A, Local Model B (dịch trên thiết bị).
  - **Kiểu dịch** (segmented control): Casual / Business / Academic.
- Nút **Hủy** hoặc **Dịch lại** để xác nhận.
- Kết quả dịch lại được thêm vào feed **dưới dạng reply/quote** của bản gốc:
  - Quote block hiển thị snippet bản gốc và link "↩ Bản gốc" để cuộn về.
  - Click link "Bản gốc" cuộn feed về đúng vị trí và highlight nhấp nháy bản gốc.
  - Card mới có nhãn footer: giờ · "Bản dịch lại" · style · model.

---

### 2.6 Xem Toàn Màn Hình (Fullscreen Modal)

- Mở bằng nút fullscreen (icon 4 mũi tên) trên translation card.
- Hiển thị dạng **modal nổi có backdrop mờ**, bo góc 16px, chiếm gần toàn màn hình.
- Animation scale-in từ 0.97 → 1 khi mở.
- Topbar của modal giống card, với các nút: Export, Dịch lại, Copy, Đóng (X).
- Nội dung song ngữ không giới hạn chiều cao, cuộn tự nhiên.
- Toggle chế độ xem (Song ngữ / Chỉ bản dịch / Chỉ nguồn) hoạt động độc lập trong modal.
- Footer hiển thị kiểu dịch.
- Đóng bằng nút X, click backdrop, hoặc phím Escape.

---

### 2.7 Export Bản Dịch

- Kích hoạt từ nút Export trên card hoặc trong modal fullscreen.
- Mở **popover "Export"** cho phép chọn định dạng:
  - **.pdf** — Xuất file PDF.
  - **.docx** — Xuất file Microsoft Word.
- Nút **Hủy** hoặc **Export** để xác nhận.

---

### 2.8 Copy Bản Dịch

- Nút Copy trên bubble ngắn: copy nội dung bản dịch vào clipboard.
- Nút Copy trên translation card: copy toàn bộ nội dung panel "Bản dịch" vào clipboard.
- Nút Copy trong modal fullscreen: copy toàn bộ bản dịch.

---

### 2.9 Cài Đặt (Settings)

**Mở cài đặt**
- Nút "Setting" ở góc dưới sidebar (icon gear).
- Mở **settings popover** kiểu Gemini nổi phía trên nút, animation fade+scale.

**Menu cài đặt gồm 2 mục:**

**a) Model AI**
- Click để mở **modal "Model AI"** (xuất hiện 1/3 từ trên màn hình).
- Cho phép cấu hình:
  - **Chọn Model:** Online / Offline.
  - **Kiểu dịch mặc định:** Phổ thông (Casual) / Học thuật (Academic) / Kinh Doanh (Business).
- Nút **Hủy** hoặc **Lưu** để xác nhận.
- Đóng bằng click backdrop hoặc Escape.

**b) Giao diện (Theme)**
- Hover vào mục "Giao diện" mở submenu bên phải với 3 lựa chọn:
  - **Hệ thống** — Theo cài đặt OS (dark/light).
  - **Sáng** — Light mode cố định.
  - **Tối** — Dark mode cố định.
- Lựa chọn hiện tại có dấu tích (checkmark).
- Lưu vào localStorage, tự động áp dụng khi load lại trang.
- Khi chế độ "Hệ thống", app lắng nghe thay đổi OS preference real-time.

---

### 2.10 Thu Gọn / Mở Rộng Sidebar

- Nút hamburger (3 gạch ngang) ở đầu sidebar toggle trạng thái collapsed/expanded.
- **Trạng thái mở rộng (360px):** Hiện đầy đủ sidebar với tên phiên, nhóm ngày, nút.
- **Trạng thái thu gọn (80px):** Ẩn text, chỉ hiện icon placeholder tròn cho mỗi phiên; nhóm "Ghim" ẩn hoàn toàn; nút Settings và New Session thu thành icon.

---

### 2.11 Tooltip

- Tooltip Material 3 xuất hiện sau 220ms hover trên icon buttons.
- Hiện phía trên button mặc định, tự chuyển xuống nếu không đủ chỗ.
- Ẩn khi di chuột ra, scroll, hoặc nhấn Escape.

---

### 2.12 Highlight Thuật Ngữ (Term Highlight)

- Các thuật ngữ đặc biệt trong bản dịch được highlight nền vàng nhạt, gạch chân đứt nét.
- Hover vào thuật ngữ hiện tooltip giải thích (tooltip riêng cho term, không phải m3-tooltip chung).

---

## 3. Yêu Cầu Giao Diện

### 3.1 Design System

**Typography**
- Font chính: Google Sans Text, Google Sans, Product Sans, Inter, system-ui (fallback chain).
- Font trang bắt đầu (chữ Hello): MTD Geraldyne (custom OTF), fallback Segoe Script, Brush Script MT, cursive.
- Font code/render: Roboto (import từ Google Fonts).
- Font size body: 14px, line-height: 1.5.

**Màu sắc — Light Mode**
| Token | Giá trị | Mô tả |
|---|---|---|
| `--bg` | #FFFFFF | Nền chính |
| `--surface` | #FFFFFF | Nền surface (input area, modal) |
| `--sidebar` | #F5F5F5 | Nền sidebar |
| `--active` | #EBEBEB | Hover/active state |
| `--input-bg` | #F0F0F0 | Nền ô nhập liệu |
| `--card-bg` | #FAFAFA | Nền card |
| `--text` | #1A1A1A | Text chính |
| `--text-secondary` | #5C5C5C | Text phụ |
| `--text-tertiary` | #8C8C8C | Text mờ |
| `--primary` | #1A1A1A | Màu nút primary |
| `--accent` | #0ea5e9 | Sky-500, màu nhấn |

**Màu sắc — Dark Mode**
| Token | Giá trị |
|---|---|
| `--bg` | #0f172a |
| `--surface` | #0f172a |
| `--sidebar` | #1e293b |
| `--active` | #334155 |
| `--input-bg` | #1e293b |
| `--text` | #f1f5f9 |
| `--accent` | #38bdf8 (sky-400) |

**Avatar & Bubble Colors**
- User avatar: gradient `#ff9a9e → #fad0c4 → #fecfef` (hồng).
- Assistant avatar: gradient `#89f7fe → #a1e3e2 → #d1fdff` (xanh lá/xanh dương).
- User bubble: gradient nhạt tông hồng (opacity ~12–14%).
- Assistant bubble: gradient nhạt tông xanh lá (opacity ~12–14%).
- Translation panel nguồn (trái): gradient xanh nhạt từ trên xuống.
- Translation panel bản dịch (phải): gradient hồng nhạt từ trên xuống.

**Border Radius**
- `--r-full`: 28px (pill shape)
- `--r-large`: 16px (card, modal, input)
- `--r-medium`: 12px (session item, dropdown)
- `--r-small`: 8px (chip, select, dropdown item)

**Shadow**
- `--shadow-subtle`: 0 1px 2px rgba(0,0,0,.04)
- `--shadow-elev1`: 0 1px 2px + 0 10px 24px (card, bubble mặc định)
- `--shadow-elev2`: 0 2px 6px + 0 16px 40px (hover state)

**Easing**
- `--ease`: cubic-bezier(0.2, 0, 0, 1) — cho enter/exit.
- `--ease-out`: cubic-bezier(0, 0, 0.2, 1) — cho exit.

### 3.2 Layout Tổng Thể

```
┌──────────────────────────────────────────────┐
│  NAV DRAWER (360px)  │  MAIN CONTENT (flex-1) │
│  ┌────────────────┐  │  ┌──────────────────┐  │
│  │ Hamburger btn  │  │  │  START VIEW      │  │
│  ├────────────────┤  │  │  (Hello + chips) │  │
│  │ Btn New Session│  │  │                  │  │
│  ├────────────────┤  │  │  hoặc            │  │
│  │ Sidebar Groups │  │  │                  │  │
│  │  - Ghim        │  │  │  CHAT VIEW       │  │
│  │  - Hôm nay     │  │  │  - Session Header│  │
│  │  - N ngày trước│  │  │  - Chat Feed     │  │
│  │  - Ngày cụ thể │  │  │                  │  │
│  ├────────────────┤  │  ├──────────────────┤  │
│  │ Settings btn   │  │  │  CHAT INPUT AREA │  │
│  └────────────────┘  │  └──────────────────┘  │
└──────────────────────────────────────────────┘
```

- Layout dạng flex ngang, chiều cao 100vh, overflow hidden.
- Sidebar có thể collapse xuống 80px (chỉ icons).
- Main content flex:1, overflow hidden.
- Chat input area sticky ở đáy, tồn tại trong cả start view và chat view (được move DOM khi switch).

### 3.3 Nguyên Tắc Thiết Kế

- **Không viền rõ ràng:** Phân tách vùng bằng màu nền và shadow, không dùng border.
- **Depth bằng shadow:** 3 mức độ nổi (subtle, elev1, elev2).
- **Motion design (Fluent):** Transition 0.2–0.28s với custom easing, animation enter/exit nhất quán.
- **Hover states:** Mọi interactive element phải có hover/active rõ ràng.
- **Focus outline:** Tắt browser default outline, dùng box-shadow hoặc background custom.
- **Dark mode đầy đủ:** Toàn bộ design token có biến thể dark mode.

---

## 4. Luồng Người Dùng (User Flows)

### 4.1 Luồng Dịch Văn Bản Ngắn

```
1. Mở app → Start View hiện với chữ Hello và 4 gợi ý nhanh
2. Chọn kiểu dịch (Chip "Casual") nếu muốn thay đổi
3. Nhập văn bản vào ô input
4. Bấm gửi (Enter hoặc nút Send)
5. Chat View hiện: bubble người dùng + bubble bản dịch từ AI
6. (Tùy chọn) Hover vào bubble AI → hiện nút Dịch lại / Copy
7. (Tùy chọn) Bấm Dịch lại → Popover → chọn model/style → xác nhận
8. Bản dịch lại hiện với quote bản gốc
```

### 4.2 Luồng Dịch File Tài Liệu

```
1. Trong ô input, bấm nút đính kèm (paperclip)
2. Chọn file .pdf / .docx / .txt
3. File bubble hiện trong feed
4. AI xử lý → Translation Card hiện với chế độ Song ngữ mặc định
5. (Tùy chọn) Chuyển sang "Chỉ bản dịch" hoặc "Chỉ nguồn"
6. (Tùy chọn) Bấm fullscreen để đọc thoải mái
7. (Tùy chọn) Bấm Export → chọn .pdf / .docx → tải về
8. (Tùy chọn) Bấm Dịch lại với model/style khác
```

### 4.3 Luồng Chat Hỏi Đáp

```
1. Trong ô input, bấm chip "Dịch" → chuyển sang "Chat" (toggle switch)
2. Nhập câu hỏi về thuật ngữ, ngữ pháp, ngữ cảnh...
3. AI phản hồi dạng hội thoại trong bubble assistant
```

### 4.4 Luồng Sử Dụng Gợi Ý Nhanh

```
1. Start View → bấm một trong 4 chip gợi ý
2. Nội dung gợi ý điền vào ô input
3. Chuyển sang Chat View (feed trống, session title "Phiên dịch mới")
4. Người dùng gửi hoặc chỉnh sửa trước khi gửi
```

### 4.5 Luồng Quản Lý Phiên

```
Ghim phiên:
  Session item → hover → 3 chấm → Ghim
  → Phiên chuyển lên nhóm "Ghim" với nền phân biệt

Đổi tên phiên (từ sidebar):
  Session item → hover → 3 chấm → Đổi tên
  → Title inline edit → Enter để lưu / Escape để hủy

Đổi tên phiên (từ chat header):
  Click vào tiêu đề phiên → inline edit → Enter lưu / Escape hủy

Tạo phiên mới:
  Btn "Bắt đầu phiên dịch mới" → Start View → feed reset
```

### 4.6 Luồng Cấu Hình

```
Đổi theme:
  Settings btn → popover → "Giao diện" → hover → submenu
  → Chọn Hệ thống / Sáng / Tối → đóng popover, áp dụng ngay

Đổi Model AI:
  Settings btn → popover → "Model AI"
  → Modal → chọn model + kiểu dịch mặc định → Lưu
```

---

## 5. Các Thành Phần UI (UI Components)

### 5.1 Navigation Drawer (Sidebar)

| Thành phần | Mô tả |
|---|---|
| `drawer-hamburger` | Nút toggle collapse, icon 3 gạch ngang |
| `btn-new-session` | Nút "Bắt đầu phiên dịch mới", icon dấu cộng |
| `sidebar-group-label` | Label nhóm thời gian (Ghim / Hôm nay / N ngày trước / Ngày cụ thể) |
| `session-item` | Item một phiên trong sidebar, có hover state và active state |
| `session-item-title` | Tên phiên, hỗ trợ inline edit |
| `session-item-menu` | Nút 3 chấm, hiện khi hover, mở dropdown-menu |
| `dropdown-menu` | Menu thả xuống: Ghim/Bỏ ghim + Đổi tên |
| `pinned-group` | Nhóm phiên ghim, nền phân biệt `--active` |
| `btn-sidebar-settings` | Nút Settings ở footer sidebar, icon gear |

### 5.2 Start View

| Thành phần | Mô tả |
|---|---|
| `start-hello-wrap` | Container cố định height 130px cho chữ chào |
| `start-hello` | Chữ Hello/Xin chào, font cursive, gradient animated |
| `start-hello-char` | Mỗi ký tự là 1 span riêng, fade-in tuần tự |
| `start-subtext` | Mô tả ngắn hướng dẫn |
| `start-suggestions-label` | Label "Thử nhanh" |
| `start-suggestion-chip` | Chip gợi ý, pill shape, icon + text, hover lift effect |

### 5.3 Chat Session Header

| Thành phần | Mô tả |
|---|---|
| `chat-session-header` | Sticky header, shadow xuất hiện khi feed cuộn |
| `chat-session-title` | Tên phiên, inline editable khi click |
| `chat-session-title-wrap` | Container tiêu đề |

### 5.4 Chat Feed

| Thành phần | Mô tả |
|---|---|
| `chat-msg.user` | Bubble tin người dùng, căn phải, avatar U |
| `chat-msg.assistant` | Bubble AI, căn trái, avatar T |
| `chat-msg.user-file` | File attachment bubble của người dùng |
| `chat-msg.user-text-upload` | Text upload preview bubble |
| `chat-bubble` | Nội dung bubble với shadow và bo góc |
| `chat-lang-label` | Nhãn phụ: giờ · ngôn ngữ · style |
| `assistant-bubble-wrap` | Container cho bubble AI + hover actions |
| `assistant-bubble-actions` | Toolbar nổi khi hover: Dịch lại + Copy |
| `file-bubble` | Bubble file: icon + tên file |
| `reply-quote` | Block quote khi dịch lại, kèm link bản gốc |
| `translation-card-wrap` | Wrapper ngoài card, chứa cả footer |
| `translation-card` | Thẻ dịch song ngữ có topbar + bilingual view |
| `card-topbar` | 3-column grid: title / view toggle / actions |
| `view-toggle` | Segmented control: Song ngữ / Chỉ bản dịch / Chỉ nguồn |
| `bilingual-view` | Grid 2 cột (hoặc 1 cột khi đổi mode) |
| `translation-panel` | Một cột panel (nguồn hoặc đích) |
| `panel-head` | Header nhỏ của panel: "Nguồn · Tiếng Việt" |
| `panel-body` | Nội dung dịch, hỗ trợ h3/p/ul/ol, scrollable |
| `card-footer` | Nhãn giờ + style bên ngoài card |
| `term-highlight` | Highlight thuật ngữ, vàng nhạt, gạch chân đứt |
| `term-tooltip` | Tooltip giải thích thuật ngữ |

### 5.5 Chat Input Area

| Thành phần | Mô tả |
|---|---|
| `chat-input-area` | Vùng nhập liệu, sticky bottom |
| `text-field` | Textarea đa dòng, resize none |
| `input-controls` | Nhóm nút trong input, căn trái dưới |
| `btn-attach` | Nút đính kèm file, icon paperclip |
| `input-chip` (style) | Chip chọn kiểu dịch: Casual/Business/Academic |
| `input-chip` (mode) | Chip chọn chế độ: Dịch/Chat |
| `input-actions` | Nhóm nút, căn phải dưới |
| `btn-send-icon` | Nút gửi tròn, màu primary (đen/trắng tùy theme) |

### 5.6 Action Buttons (Icon Buttons)

| Thành phần | Mô tả |
|---|---|
| `btn-icon` | Icon button tròn 36px, hover background mờ |
| `.btn-icon[data-retranslate]` | Nút Dịch lại (icon refresh) |
| `.btn-icon[data-copy]` | Nút Copy (icon copy) |
| `.btn-icon[data-fullscreen]` | Nút Xem toàn màn hình (luôn hiện) |
| `.btn-icon.export-trigger` | Nút Export (icon download) |

### 5.7 Popover Components

| Thành phần | Mô tả |
|---|---|
| `retranslate-popover` | Popover Dịch lại: model online/offline + style |
| `export-popover` | Popover Export: chọn định dạng .pdf/.docx |
| `input-style-popover` | Popover chọn kiểu dịch cho input |
| `input-mode-popover` | Popover chọn chế độ Dịch/Chat |
| `settings-popover` | Popover Settings: Model AI + Giao diện |
| `config-submenu` | Submenu chọn theme, xuất hiện bên phải popover |

Tất cả popover có:
- Animation `popover-enter` (fade + slide up).
- Mũi tên chỉ nguồn (CSS before pseudo-element).
- Tự vị trí để không vượt ngoài viewport.
- Đóng khi click ngoài hoặc Escape.

### 5.8 Modal Components

| Thành phần | Mô tả |
|---|---|
| `fullscreen-modal` | Modal xem toàn màn hình cho translation card |
| `fullscreen-backdrop` | Backdrop mờ rgba(0,0,0,.45) |
| `model-ai-modal` | Modal cấu hình Model AI |
| `settings-backdrop` | Backdrop click-outside cho settings/model modal |

### 5.9 Settings & Config Components

| Thành phần | Mô tả |
|---|---|
| `settings-row` | Hàng cài đặt: label + control, hover state |
| `settings-row.compact` | Hàng cài đặt dạng 2-column grid |
| `retranslate-select` | Dropdown select với custom arrow SVG |
| `theme-segmented` | Segmented control pill cho chọn style dịch |
| `m3-switch` | Toggle switch kiểu Material 3 (track + thumb) |

### 5.10 Feedback Components

| Thành phần | Mô tả |
|---|---|
| `m3-tooltip` | Tooltip M3: xuất hiện sau delay, tự định vị |
| `jump-highlight` | Highlight blink khi scroll đến bản gốc từ reply |
| `inline-label` | Pill label nhỏ (style display) |

---

## 6. Yêu Cầu Phi Chức Năng

### 6.1 Hiệu Năng
- Animation và transition mượt 60fps, dùng CSS transform/opacity (không layout thrash).
- Tooltip delay 220ms để không gây phiền.
- Scroll chaining: khi panel dịch không cuộn được, truyền scroll lên chat feed.

### 6.2 Khả Năng Tiếp Cận (Accessibility)
- Các nút có `aria-label` mô tả đầy đủ.
- Modal có `role="dialog"`, `aria-modal="true"`, `aria-labelledby`.
- Popover settings có `role="menu"` và `role="menuitem"`.
- Submenu theme có `role="menuitemradio"`.
- Tooltip sử dụng `role="tooltip"` và `aria-hidden`.
- Focus management: Escape đóng dialog/popover.

### 6.3 Lưu Trữ
- Theme preference lưu vào `localStorage` với key `translate-app-theme`.
- Sync real-time với OS preference khi chọn chế độ "Hệ thống".

### 6.4 Hỗ Trợ Trình Duyệt
- CSS custom properties (`var()`), `color-mix()`, `clamp()`, CSS Grid.
- `scrollbar-gutter: stable` cho smooth scroll layout.
- `-webkit-overflow-scrolling: touch` cho mobile.
- Pointer Events API cho press state management.
- `navigator.clipboard` cho copy (với fallback).

---

*Tài liệu này được tạo dựa trên phân tích toàn bộ mockup HTML (3330 dòng) của Translate App.*
