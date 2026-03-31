package db

import (
	"database/sql"
	"fmt"
)

// Sample session IDs (fixed UUIDs) — xóa/ghi lại khi chạy SeedSampleData.
const (
	sampleSessionShort  = "00000000-0000-4000-a000-000000000001"
	sampleSessionBiling = "00000000-0000-4000-a000-000000000002"
	sampleSessionThread = "00000000-0000-4000-a000-000000000003"
	sampleSessionLong   = "00000000-0000-4000-a000-000000000004"
)

var sampleSessionIDs = []string{
	sampleSessionShort,
	sampleSessionBiling,
	sampleSessionThread,
	sampleSessionLong,
}

// SeedSampleData xóa các session mẫu cũ (theo ID cố định) rồi chèn lại dữ liệu demo.
// Gọi sau khi DB đã migrate (vd. qua Open()). An toàn chạy nhiều lần.
func SeedSampleData(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range sampleSessionIDs {
		if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, id); err != nil {
			return fmt.Errorf("delete sample session %s: %w", id, err)
		}
	}

	// --- Session 1: hội thoại ngắn (bubble) ---
	if err := insertSession(tx, sampleSessionShort,
		"Họp khách hàng — confirm slot",
		"active", "en-US", "business", "gemini-2.0-flash",
		"2026-03-20T08:12:00Z", "2026-03-20T08:13:05Z"); err != nil {
		return err
	}
	if err := insertMessage(tx, "00000000-0000-4000-a000-000000000101", sampleSessionShort,
		"user", 1, "bubble",
		"Chị Thuỷ ơi, team em có thể họp 15:00 JST thứ Sáu tuần này được không ạ? Nếu không ổn em xin slot sáng thứ Hai.",
		"", "vi", "en-US", "business", nil, 0,
		"2026-03-20T08:12:00Z", "2026-03-20T08:12:00Z"); err != nil {
		return err
	}
	if err := insertMessage(tx, "00000000-0000-4000-a000-000000000102", sampleSessionShort,
		"assistant", 2, "bubble",
		"",
		"Hi team, could we lock in **15:00 JST this Friday**? If that doesn’t work, a **Monday morning** slot would be great—please share two options.",
		"vi", "en-US", "business", nil, 420,
		"2026-03-20T08:13:05Z", "2026-03-20T08:13:05Z"); err != nil {
		return err
	}

	// --- Session 2: bilingual + markdown ---
	bilingualUser := "# Đặc tả API — Phiên bản nội bộ\n\n" +
		"## Mục tiêu\n" +
		"Xây endpoint **POST /v1/sessions** tạo phiên dịch mới, trả `sessionId` + `messageId`.\n\n" +
		"## Body (JSON)\n" +
		"| Field | Type | Bắt buộc |\n" +
		"|-------|------|----------|\n" +
		"| title | string | Có |\n" +
		"| content | string | Có |\n" +
		"| targetLang | string | Có |\n\n" +
		"## Lỗi\n" +
		"- `400` — thiếu field\n" +
		"- `429` — rate limit provider\n"

	bilingualAsst := "# API Spec — Internal draft\n\n" +
		"## Goal\n" +
		"Expose **POST /v1/sessions** to create a new translation session and return `sessionId` + `messageId`.\n\n" +
		"## Body (JSON)\n" +
		"| Field | Type | Required |\n" +
		"|-------|------|----------|\n" +
		"| title | string | Yes |\n" +
		"| content | string | Yes |\n" +
		"| targetLang | string | Yes |\n\n" +
		"## Errors\n" +
		"- `400` — missing field\n" +
		"- `429` — provider rate limit\n"
	if err := insertSession(tx, sampleSessionBiling,
		"# SPEC — API phiên dịch",
		"active", "en-US", "academic", "gemini-2.0-flash",
		"2026-03-19T11:00:00Z", "2026-03-19T11:02:30Z"); err != nil {
		return err
	}
	if err := insertMessage(tx, "00000000-0000-4000-a000-000000000201", sampleSessionBiling,
		"user", 1, "bilingual",
		bilingualUser,
		"", "vi", "en-US", "academic", nil, 0,
		"2026-03-19T11:00:00Z", "2026-03-19T11:00:00Z"); err != nil {
		return err
	}
	if err := insertMessage(tx, "00000000-0000-4000-a000-000000000202", sampleSessionBiling,
		"assistant", 2, "bilingual",
		bilingualUser,
		bilingualAsst,
		"vi", "en-US", "academic", nil, 980,
		"2026-03-19T11:02:30Z", "2026-03-19T11:02:30Z"); err != nil {
		return err
	}

	// --- Session 3: thread nhiều lượt (bubble) ---
	if err := insertSession(tx, sampleSessionThread,
		"Brainstorm tính năng Translate",
		"pinned", "ja-JP", "casual", "gemini-2.0-flash",
		"2026-03-18T09:00:00Z", "2026-03-18T09:25:00Z"); err != nil {
		return err
	}
	type row struct {
		id    string
		role  string
		order int
		orig  string
		trans string
		tok   int
		ts    string
	}
	for _, m := range []row{
		{"00000000-0000-4000-a000-000000000301", "user", 1, "Ý tưởng: thêm **chế độ offline** cho Ollama local?", "", 0, "2026-03-18T09:00:00Z"},
		{"00000000-0000-4000-a000-000000000302", "assistant", 2, "", "アイデア：**Ollama ローカル向けオフラインモード**を足す？", 55, "2026-03-18T09:01:00Z"},
		{"00000000-0000-4000-a000-000000000303", "user", 3, "Ok — cần UX rõ: badge \"Local\" + không gọi cloud khi bật.", "", 0, "2026-03-18T09:05:00Z"},
		{"00000000-0000-4000-a000-000000000304", "assistant", 4, "", "了解。**「Local」バッジ**と、オン時はクラウドを呼ばないのが分かるUIが必要ですね。", 120, "2026-03-18T09:06:00Z"},
		{"00000000-0000-4000-a000-000000000305", "user", 5, "Edge case: user paste 5MB markdown — giới hạn ở đâu?", "", 0, "2026-03-18T09:20:00Z"},
		{"00000000-0000-4000-a000-000000000306", "assistant", 6, "", "エッジケース：5MBのMarkdown貼り付けは**FEでソフトキャップ**、BEは**ストリーム＋上限エラー**が現実的です。", 200, "2026-03-18T09:25:00Z"},
	} {
		if err := insertMessage(tx, m.id, sampleSessionThread,
			m.role, m.order, "bubble",
			m.orig, m.trans,
			"vi", "ja-JP", "casual", nil, m.tok,
			m.ts, m.ts); err != nil {
			return err
		}
	}

	// --- Session 4: user message dài (để sau này test UserTextCard / line-clamp) ---
	longUser := repeatParagraph(`Đoạn mẫu dài để kiểm tra UI: ứng dụng dịch cần hiển thị văn bản nguồn rõ ràng, không nuốt ký tự đặc biệt như "quotes" và dấu —. `, 45)
	longAsst := repeatParagraph(`Sample long target text: the translate UI should wrap sensibly and remain readable with "quotes" and em-dashes — without breaking layout. `, 45)
	if err := insertSession(tx, sampleSessionLong,
		"Văn bản dài (stress test UI)",
		"active", "en-US", "casual", "gemini-2.0-flash",
		"2026-03-17T16:00:00Z", "2026-03-17T16:02:00Z"); err != nil {
		return err
	}
	if err := insertMessage(tx, "00000000-0000-4000-a000-000000000401", sampleSessionLong,
		"user", 1, "bilingual",
		longUser,
		"", "vi", "en-US", "casual", nil, 0,
		"2026-03-17T16:00:00Z", "2026-03-17T16:00:00Z"); err != nil {
		return err
	}
	if err := insertMessage(tx, "00000000-0000-4000-a000-000000000402", sampleSessionLong,
		"assistant", 2, "bilingual",
		longUser,
		longAsst,
		"vi", "en-US", "casual", nil, 2100,
		"2026-03-17T16:02:00Z", "2026-03-17T16:02:00Z"); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func repeatParagraph(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func insertSession(tx *sql.Tx, id, title, status, targetLang, style, model, createdAt, updatedAt string) error {
	_, err := tx.Exec(`
INSERT INTO sessions (id, title, status, target_lang, style, model, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, title, status, targetLang, style, model, createdAt, updatedAt)
	return err
}

func insertMessage(tx *sql.Tx, id, sessionID, role string, displayOrder int, displayMode, original, translated string,
	sourceLang, targetLang, style string, originalMsgID *string, tokens int, createdAt, updatedAt string,
) error {
	var origRef interface{}
	if originalMsgID != nil {
		origRef = *originalMsgID
	}
	_, err := tx.Exec(`
INSERT INTO messages (
  id, session_id, role, display_order, display_mode,
  original_content, translated_content, source_lang, target_lang, style,
  original_message_id, tokens, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, sessionID, role, displayOrder, displayMode,
		original, translated, sourceLang, targetLang, style,
		origRef, tokens, createdAt, updatedAt)
	return err
}
