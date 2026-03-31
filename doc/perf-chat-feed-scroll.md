# Hiệu năng scroll feed chat — phân tích FE/BE

## Kết luận ngắn

| Lớp | Scroll có gọi không? | Ảnh hưởng lag? |
|-----|----------------------|----------------|
| **Backend / Wails IPC** | **Không.** `GetMessages` chỉ chạy khi mở phiên, gửi tin, hoặc `loadMoreMessages` (scroll gần đỉnh). | Không. |
| **Frontend DOM** | Cuộn chỉ thay đổi `scrollTop` trong `.chat-feed`. | **Có — chính là đây.** |

Lag chủ yếu do **số lượng node DOM + React tree** (mỗi tin: `react-markdown`, thẻ song ngữ, `ResizeObserver`, v.v.), không phải do “fetch sai thứ tự” hay backend chậm.

## Luồng dữ liệu (đúng như thiết kế)

1. **Lần đầu:** `loadMessages(sessionId)` → `GetMessages(cursor=0, limit=N)` → chunk **mới nhất** (`ORDER BY display_order DESC` trong SQL).
2. **Scroll lên (gần đỉnh):** `onFeedScroll` → `loadMoreMessages` → `GetMessages(nextCursor, N)` → tin **cũ hơn**, prepend vào store, chỉnh `scrollTop` để giữ vị trí nhìn.
3. **Scroll xuống:** **Không** gọi API; chỉ cuộn trong cùng một danh sách đã mount.

Vì vậy nếu “scroll xuống vẫn lag”, nguyên nhân là **vẫn đang vẽ/paint/layout quá nhiều phần tử**, không phải backend lặp lại request.

## Nguyên nhân FE chi tiết

1. **Không virtualize:** Mọi tin trong session (sau nhiều lần load-more) đều **mount** `ChatMessage` + `MessageMarkdown` (parse markdown + GFM). Chi phí tăng gần tuyến tính với số tin.
2. **`ResizeObserver`** trên panel song ngữ (collapsible) — mỗi thẻ bilingual một observer; số lượng lớn → thêm chi phí khi layout.
3. **`App` đăng ký nhiều slice Zustand** (`sessions`, `streamStatus`, `hasMore`, …). Mỗi lần các field này đổi, `App` re-render; đã giảm một phần nhờ `React.memo` trên `ChatMessage` + lookup `assistantById` O(1).
4. **Spinner / CSS trước đây:** `conic-gradient` + `mask` + `will-change` có thể tăng tải GPU trong lúc hiện spinner; đã đổi sang border đơn giản. Lag scroll **chủ yếu vẫn do list dài**.

## Biện pháp đã / nên làm

- **Virtual list (`@tanstack/react-virtual`):** Chỉ mount các hàng trong (và gần) viewport + `overscan`. Đây là thay đổi có tác động lớn nhất khi feed dài.
- **Giữ `memo` + `Map` quote:** Giảm re-render vô ích khi `App` đổi state khác.
- **Backend:** Chỉ cần giữ pagination ổn định; không cần thêm API cho scroll.

## Tài liệu liên quan

- SQL cursor: `internal/repository/sqlc/queries/messages.sql` — `GetMessagesBySessionCursor`
- FE store: `frontend/src/stores/message/messageStore.ts` — `MESSAGE_PAGE_SIZE`, `loadMoreMessages`
- FE scroll: `frontend/src/App.tsx` — `onFeedScroll`, `feedRef`
