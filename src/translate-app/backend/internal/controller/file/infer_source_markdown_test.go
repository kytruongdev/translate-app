package file

import (
	"strings"
	"testing"
)

func TestPlainTextLooksLikeMarkdown(t *testing.T) {
	if !plainTextLooksLikeMarkdown("## Hi\n\nbody") {
		t.Fatal("expected markdown")
	}
	if !plainTextLooksLikeMarkdown("x **a** b **c**") {
		t.Fatal("expected bold pair")
	}
	if plainTextLooksLikeMarkdown("CHỈ LÀ CHỮ HOA\n\nđoạn") {
		t.Fatal("should not treat as md")
	}
}

func TestInferMarkdownFromPlain_NumberedHeadings(t *testing.T) {
	in := "1. Giới thiệu\n\nNội dung đoạn một.\n\n2. Nội dung chính\n\nChi tiết."
	out := inferMarkdownFromPlain(in)
	if !strings.Contains(out, "## 1. Giới thiệu") {
		t.Fatalf("missing heading 1: %q", out)
	}
	if !strings.Contains(out, "## 2. Nội dung chính") {
		t.Fatalf("missing heading 2: %q", out)
	}
	if !strings.Contains(out, "Nội dung đoạn một.") {
		t.Fatal("lost body")
	}
}

func TestInferMarkdownFromPlain_UppercaseTitle(t *testing.T) {
	in := "LUẬN ÁN TIẾN SĨ LUẬT HỌC\n\nĐây là phần mở đầu bình thường."
	out := inferMarkdownFromPlain(in)
	if !strings.HasPrefix(out, "## LUẬN ÁN TIẾN SĨ LUẬT HỌC") {
		t.Fatalf("expected title promoted: %q", out)
	}
}

func TestInferMarkdownFromPlain_SectionKeyword(t *testing.T) {
	in := "CHƯƠNG 1\n\nNội dung chương."
	out := inferMarkdownFromPlain(in)
	if !strings.HasPrefix(out, "## CHƯƠNG 1") {
		t.Fatalf("expected chapter heading: %q", out)
	}
}

func TestSourceMarkdownFromPlain_SkipsWhenAlreadyMD(t *testing.T) {
	in := "## Đã có\n\ntext"
	got := sourceMarkdownFromPlain(in)
	if got != in {
		t.Fatalf("should not rewrite: %q", got)
	}
}
