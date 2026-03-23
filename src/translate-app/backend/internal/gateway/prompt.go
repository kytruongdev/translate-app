package gateway

import (
	"strings"
)

// Target language labels for prompts (architecture §5.4 / §9.1).
var targetLangNames = map[string]string{
	"en":    "English (United States)",
	"en-US": "English (United States)",
	"en-GB": "English (United Kingdom)",
	"en-AU": "English (Australia)",
	"vi":    "Vietnamese",
	"vi-VN": "Vietnamese",
	"ko":    "Korean",
	"ko-KR": "Korean",
	"ja":    "Japanese",
	"ja-JP": "Japanese",
	"zh-CN": "Simplified Chinese",
	"zh-TW": "Traditional Chinese",
	"fr":    "French",
	"fr-FR": "French",
	"de":    "German",
	"de-DE": "German",
	"es":    "Spanish",
	"es-ES": "Spanish",
}

// TargetLangLabel returns a human-readable target language name for the AI prompt.
func TargetLangLabel(locale string) string {
	if v, ok := targetLangNames[strings.TrimSpace(locale)]; ok {
		return v
	}
	if locale == "" {
		return "English (United States)"
	}
	return locale
}

const markdownPreserveRule = `IMPORTANT: Preserve ALL Markdown formatting tags exactly (# ## ### **bold** *italic* > - etc.)
Only translate text content, never translate or modify Markdown syntax.

CRITICAL OUTPUT LANGUAGE: Every human-readable word and sentence in your reply must be written in the target language specified above.
Do not leave paragraphs in the source language. Names may stay as-is when conventional; translate titles, headings, and body text fully.
Never insert phrases in Chinese, Japanese, or Korean (or any non-target language) for clarification or emphasis — translate all prose into the target language only.`

func sourceLangKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasPrefix(s, "vi") {
		return "vi"
	}
	if s == "auto" || s == "" || s == "unknown" {
		return "auto"
	}
	return s
}

// targetMonolingualConstraint giảm lẫn ngôn ngữ (vd. chèn tiếng Trung giữa đoạn tiếng Anh khi đích là en).
func targetMonolingualConstraint(targetLocale string) string {
	target := TargetLangLabel(targetLocale)
	tl := strings.ToLower(strings.TrimSpace(targetLocale))
	if tl == "" {
		tl = "en"
	}
	switch {
	case tl == "zh-cn" || strings.HasPrefix(tl, "zh-cn"):
		return "MONOLINGUAL OUTPUT (Chinese): Write the entire translation in Simplified Chinese (简体中文). " +
			"Do not switch mid-paragraph to Japanese, Korean, or long English runs except proper names or quoted text."
	case tl == "zh-tw" || tl == "zh-hk" || strings.HasPrefix(tl, "zh-tw"):
		return "MONOLINGUAL OUTPUT (Chinese): Write the entire translation in Traditional Chinese (繁體中文). " +
			"Do not switch mid-paragraph to Simplified Chinese, Japanese, Korean, or long English except proper names or quoted text."
	case strings.HasPrefix(tl, "zh"):
		return "MONOLINGUAL OUTPUT (Chinese): Write entirely in " + target + ". " +
			"Do not mix unrelated languages in the middle of a sentence."
	case strings.HasPrefix(tl, "ja") || strings.HasPrefix(tl, "jp"):
		return "MONOLINGUAL OUTPUT (Japanese): Use only Japanese (hiragana, katakana, kanji as appropriate). " +
			"Do not insert Chinese or Korean sentences mid-sentence."
	case strings.HasPrefix(tl, "ko"):
		return "MONOLINGUAL OUTPUT (Korean): Use only Korean. Do not insert Chinese or Japanese sentences mid-sentence."
	default:
		// en, vi, fr, de, es, … — hay gặp model chèn Hán tự khi đích là Latin
		return "MONOLINGUAL OUTPUT: Write the entire translation in " + target + " only.\n" +
			"Do not use Chinese characters (Han script) except unavoidable proper names or globally fixed technical tokens.\n" +
			"Never continue or finish a sentence in a different language — especially do not switch mid-sentence to Chinese, Japanese, or Korean.\n" +
			"If the source is already in " + target + ", keep that language; only polish clarity and grammar."
	}
}

// BuildTranslationSystemPrompt builds the system instruction per doc/architecture-document.md §9.1.
func BuildTranslationSystemPrompt(sourceLang, targetLocale, style string, preserveMarkdown bool) string {
	target := TargetLangLabel(targetLocale)
	styleNorm := strings.ToLower(strings.TrimSpace(style))
	if styleNorm != "business" && styleNorm != "academic" {
		styleNorm = "casual"
	}

	var base string
	if sourceLangKey(sourceLang) == "vi" {
		switch styleNorm {
		case "business":
			base = "You are a professional translator. Translate the text from Vietnamese to " + target + "\n" +
				"in a formal, clear, and professional tone suitable for business communication.\n" +
				"Preserve technical terms. Output ONLY the translated text, no explanations."
		case "academic":
			base = "You are a scholarly translator. Translate the text from Vietnamese to " + target + "\n" +
				"with precision and rigor, using domain-appropriate terminology.\n" +
				"Maintain logical structure and formal register.\n" +
				"Output ONLY the translated text, no explanations."
		default: // casual
			base = "You are a translator. Translate the text from Vietnamese to " + target + " naturally\n" +
				"and conversationally, as if explaining to a friend. Use everyday language, avoid stiff phrasing.\n" +
				"Output ONLY the translated text, no explanations."
		}
	} else if sourceLangKey(sourceLang) == "en" {
		switch styleNorm {
		case "business":
			base = "You are a professional translator. The source text is in English. Translate it to " + target + "\n" +
				"in a formal, clear, and professional tone suitable for business communication.\n" +
				"Preserve technical terms. Output ONLY the translated text, no explanations."
		case "academic":
			base = "You are a scholarly translator. The source text is in English. Translate it to " + target + "\n" +
				"with precision and rigor, using domain-appropriate terminology.\n" +
				"Maintain logical structure and formal register.\n" +
				"Output ONLY the translated text, no explanations."
		default:
			base = "You are a translator. The source text is in English. Translate it to " + target + "\n" +
				"naturally and conversationally, as if explaining to a friend.\n" +
				"Use everyday language, avoid stiff phrasing.\n" +
				"Output ONLY the translated text, no explanations."
		}
	} else {
		// unknown — let model detect source language
		switch styleNorm {
		case "business":
			base = "You are a professional translator. Detect the language of the text and translate it\n" +
				"to " + target + " in a formal, clear, and professional tone suitable for business communication.\n" +
				"Preserve technical terms. Output ONLY the translated text, no explanations."
		case "academic":
			base = "You are a scholarly translator. Detect the language of the text and translate it\n" +
				"to " + target + " with precision and rigor, using domain-appropriate terminology.\n" +
				"Maintain logical structure and formal register.\n" +
				"Output ONLY the translated text, no explanations."
		default:
			base = "You are a translator. Detect the language of the text and translate it to " + target + "\n" +
				"naturally and conversationally, as if explaining to a friend.\n" +
				"Use everyday language, avoid stiff phrasing.\n" +
				"Output ONLY the translated text, no explanations."
		}
	}

	base = base + "\n\n" + targetMonolingualConstraint(targetLocale)

	if preserveMarkdown {
		return base + "\n\n" + markdownPreserveRule
	}
	return base
}
