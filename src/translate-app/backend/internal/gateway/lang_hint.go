package gateway

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// reVietnameseSpecific matches characters that are unique to Vietnamese
// and not found in French, Spanish, Portuguese, or other Latin-script languages.
// This avoids false positives on English documents containing words like "café" or "résumé".
// Unique Vietnamese chars: ă/Ă, đ/Đ, ơ/Ơ, ư/Ư, and tone-marked compounds using these bases.
var reVietnameseSpecific = regexp.MustCompile(
	`(?i)[ăđơưắằẳẵặấầẩẫậếềểễệốồổỗộớờởỡợứừửữự]`)

func containsEastAsianScript(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
		// Hiragana + Katakana
		if r >= 0x3040 && r <= 0x30FF {
			return true
		}
		// Hangul syllables
		if r >= 0xAC00 && r <= 0xD7AF {
			return true
		}
	}
	return false
}

func chunkLooksMostlyEnglish(text string) bool {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) < 28 {
		return false
	}
	letters := 0
	asciiLetters := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
			if r <= unicode.MaxASCII && ((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
				asciiLetters++
			}
		}
	}
	if letters < 14 {
		return false
	}
	return float64(asciiLetters)/float64(letters) >= 0.88
}

// SourceLangForTranslate returns "vi" / "en" when rõ ràng; else "auto" (nhánh detect — dễ lẫn ngôn ngữ hơn).
func SourceLangForTranslate(text string) string {
	if reVietnameseSpecific.MatchString(text) {
		return "vi"
	}
	if containsEastAsianScript(text) {
		return "auto"
	}
	if chunkLooksMostlyEnglish(text) {
		return "en"
	}
	return "auto"
}
