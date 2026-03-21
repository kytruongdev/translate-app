# Dữ liệu mẫu (seed) để thử UI

## Cách chạy

1. **Đóng** Translate App (Wails) nếu đang mở — tránh file SQLite bị lock.
2. Từ thư mục `src/translate-app/backend`:

```bash
make seed-sample
# hoặc
go run ./cmd/seed-sample
```

3. Mở lại app (`wails dev` hoặc bản build). Sidebar sẽ có thêm **4 phiên** mẫu; có thể bấm nút refresh danh sách nếu app đã mở sẵn.

Lệnh **idempotent**: mỗi lần chạy sẽ xóa và tạo lại đúng các session có ID cố định (`00000000-0000-4000-a000-…`). Session do bạn tạo tay **không** bị xóa.

## File DB thật nằm ở đâu?

- **macOS:** `~/Library/Application Support/TranslateApp/data.db`
- **Linux:** `~/.config/TranslateApp/data.db`
- **Windows:** `%AppData%\TranslateApp\data.db`

(Cùng logic với `internal/infra/db.Open()`.)

## Nội dung mẫu

| Session | Mục đích |
|--------|-----------|
| Họp khách hàng — confirm slot | 1 cặp user/assistant **bubble**, tiếng Việt → bản dịch EN có markdown nhẹ |
| # SPEC — API phiên dịch | **Bilingual** + bảng markdown (để sau này test TranslationCard 2 cột) |
| Brainstorm tính năng Translate | **Pinned** + 6 tin (thread), target `ja-JP` |
| Văn bản dài (stress test UI) | **Bilingual** + đoạn rất dài (scroll / line-clamp sau này) |

---

Nếu cần chỉnh copy hoặc thêm case, sửa `internal/infra/db/seed_sample.go` rồi chạy lại `make seed-sample`.
