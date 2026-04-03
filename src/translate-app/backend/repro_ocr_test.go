package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"translate-app/internal/controller/file"
)

// Chú ý: Script này cần chạy trong thư mục backend để findPaddleOCR hoạt động đúng
// Hoặc chúng ta sẽ giả lập context của nó.

func main() {
	// 1. Xác định file PDF test
	pdfPath := "../file-test/pdf-ocr/QUY TRÌNH CUNG CẤP THÔNG TIN VỀ TÓM TẮT HỒ SƠ BỆNH ÁN NGƯỜI BỆNH.pdf"
	if _, err := os.Stat(pdfPath); err != nil {
		log.Fatalf("Không tìm thấy file PDF test tại: %s", pdfPath)
	}

	fmt.Println("--- Đang bắt đầu test Structured OCR ---")
	fmt.Printf("File: %s\n", filepath.Base(pdfPath))

	// 2. Gọi hàm OCR (Sử dụng interface public nếu có thể, hoặc chúng ta chạy nội bộ)
	// Vì ocrStructuredPDF là hàm nội bộ (unexported), mình sẽ dùng cách gọi gián tiếp 
	// hoặc fen có thể tạm thời export nó ra để test.
	
	// Để đơn giản cho việc test ngay lập tức, mình sẽ hướng dẫn fen chạy trực tiếp 
	// cái binary sidecar bằng lệnh shell trước để xem nó ra JSON chuẩn chưa.
}
