// Command seed-sample chèn session + message mẫu vào DB của app (cùng path với wails dev).
//
// Cách dùng (từ thư mục backend):
//
//	go run ./cmd/seed-sample
//
// Đóng app trước khi chạy để tránh lock SQLite (hoặc chạy lại app sau khi seed).
package main

import (
	"log"

	appdb "translate-app/internal/infra/db"
)

func main() {
	db, err := appdb.Open()
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := appdb.SeedSampleData(db); err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Println("OK — đã chèn 4 session mẫu. Mở lại Translate App và bấm refresh sidebar nếu cần.")
}
