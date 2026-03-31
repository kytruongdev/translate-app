# TranslateApp layout

Khớp `doc/architecture-document.md` §6.1: **`backend/`** và **`frontend/`** là **hai thư mục anh em**, không lồng nhau.

```
src/translate-app/
├── backend/     # Go + Wails (module root)
│   └── dist/    # chỉ do `npm run build` trong frontend tạo ra — static assets embed vào binary
└── frontend/    # React + Vite (source)
```

`vite.config.ts` đặt `build.outDir` = `../backend/dist` vì Go `embed` không được trỏ ra ngoài thư mục chứa `go.mod` (`..` trong pattern embed).

## Chạy thử (desktop)

Cần [Wails CLI](https://wails.io/) và Go trên máy.

```bash
cd src/translate-app/backend
wails dev
```

Cửa sổ app mở ra: có **sidebar** (danh sách session / trạng thái rỗng), **Trang bắt đầu** bên phải, nút **thu gọn sidebar**, **đổi theme** (lưu SQLite), **làm mới danh sách**. Ô nhập văn bản là preview — gửi dịch sẽ có ở epic E4.

Build bản cài đặt: `make build` hoặc `wails build` (trong `backend/`).
