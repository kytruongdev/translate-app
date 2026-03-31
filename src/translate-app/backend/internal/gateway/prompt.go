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

// outputLangGuard returns a hard constraint placed at the TOP of the system prompt
// to prevent the model from outputting memorised translations in a wrong language.
func outputLangGuard(target string) string {
	return "ABSOLUTE OUTPUT LANGUAGE RULE: Every single word you write must be in " + target + ".\n" +
		"Do NOT produce output in Chinese, Japanese, Korean, or any other language under any circumstance.\n" +
		"This applies to ALL content: body text, quoted titles, article names, journal names, citations, " +
		"proper nouns, and headings. Translate everything fresh from the source text — " +
		"do not use or recall any pre-existing translations you may know.\n" +
		"Violation of this rule is a critical error.\n\n"
}

// mixedLangNote instructs the model to translate embedded foreign-language text
// (e.g. Chinese journal citations inside a Vietnamese document).
const mixedLangNote = "If you encounter text written in Chinese, Japanese, Korean, or any other language " +
	"within the source, translate it into the target language as well — do not copy it verbatim."

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

// BuildDocxBatchSystemPrompt builds the system prompt for DOCX paragraph-batch translation.
// Unlike BuildTranslationSystemPrompt, this does NOT say "output ONLY translated text"
// (which conflicts with marker preservation). Instead it makes the <<<N>>> format rule
// explicit in the system prompt so the AI has no conflicting instructions.
func BuildDocxBatchSystemPrompt(from, to, style string) string {
	target := TargetLangLabel(to)
	styleNorm := strings.ToLower(strings.TrimSpace(style))
	if styleNorm != "business" && styleNorm != "academic" {
		styleNorm = "casual"
	}

	srcKey := sourceLangKey(from)
	var fromClause string
	switch srcKey {
	case "vi":
		fromClause = "from Vietnamese "
	case "en":
		fromClause = "from English "
	default:
		fromClause = "" // auto-detect
	}

	var base string
	switch styleNorm {
	case "business":
		base = "You are a professional translator. Translate each paragraph " + fromClause + "to " + target +
			" in a formal, clear, and professional tone suitable for business communication.\n" +
			"Preserve technical terms. " + mixedLangNote
	case "academic":
		base = "You are a scholarly translator. Translate each paragraph " + fromClause + "to " + target +
			" with precision and rigor, using domain-appropriate terminology.\n" +
			"Maintain logical structure and formal register. " + mixedLangNote
	default:
		base = "You are a translator. Translate each paragraph " + fromClause + "to " + target +
			" naturally and conversationally, using everyday language.\n" + mixedLangNote
	}

	markerRule := "\n\nFORMAT RULE (critical):\n" +
		"The user message contains paragraphs numbered with <<<N>>> markers.\n" +
		"Return each translated paragraph preceded by its marker, like this:\n\n" +
		"<<<1>>>\n[translation of paragraph 1]\n\n<<<2>>>\n[translation of paragraph 2]\n\n" +
		"IMPORTANT: Keep every <<<N>>> marker EXACTLY as written — exactly 3 angle brackets on each side, same number N.\n" +
		"Do NOT add extra angle brackets. Do NOT remove markers. Do NOT add any explanation or commentary."

	body := base + markerRule + "\n\n" + targetMonolingualConstraint(to)

	tl := strings.ToLower(strings.TrimSpace(to))
	if !strings.HasPrefix(tl, "zh") && !strings.HasPrefix(tl, "ja") &&
		!strings.HasPrefix(tl, "ko") && tl != "jp" {
		return outputLangGuard(target) + body
	}
	return body
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
				"Preserve technical terms. " + mixedLangNote + "\n" +
				"Output ONLY the translated text, no explanations."
		case "academic":
			base = "You are a scholarly translator. Translate the text from Vietnamese to " + target + "\n" +
				"with precision and rigor, using domain-appropriate terminology.\n" +
				"Maintain logical structure and formal register. " + mixedLangNote + "\n" +
				"Output ONLY the translated text, no explanations."
		default: // casual
			base = "You are a translator. Translate the text from Vietnamese to " + target + " naturally\n" +
				"and conversationally, as if explaining to a friend. Use everyday language, avoid stiff phrasing.\n" +
				mixedLangNote + "\n" +
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
		// unknown source — model detects language automatically.
		// OUTPUT CONSTRAINT is explicit: regardless of what language the model detects,
		// the entire output must be in the target language (no source-language passthrough).
		outputConstraint := "Your output MUST be entirely in " + target + " — translate everything, " +
			"including any embedded foreign-language passages or citations."
		switch styleNorm {
		case "business":
			base = "You are a professional translator. Identify the source language of the text and translate it\n" +
				"to " + target + " in a formal, clear, and professional tone suitable for business communication.\n" +
				"Preserve technical terms. " + outputConstraint + "\n" +
				"Output ONLY the translated text, no explanations."
		case "academic":
			base = "You are a scholarly translator. Identify the source language of the text and translate it\n" +
				"to " + target + " with precision and rigor, using domain-appropriate terminology.\n" +
				"Maintain logical structure and formal register. " + outputConstraint + "\n" +
				"Output ONLY the translated text, no explanations."
		default:
			base = "You are a translator. Identify the source language of the text and translate it to " + target + "\n" +
				"naturally and conversationally, as if explaining to a friend.\n" +
				"Use everyday language, avoid stiff phrasing. " + outputConstraint + "\n" +
				"Output ONLY the translated text, no explanations."
		}
	}

	body := base + "\n\n" + targetMonolingualConstraint(targetLocale)
	if preserveMarkdown {
		body = body + "\n\n" + markdownPreserveRule
	}

	// Prepend hard output-language guard for non-CJK targets.
	tl := strings.ToLower(strings.TrimSpace(targetLocale))
	if !strings.HasPrefix(tl, "zh") && !strings.HasPrefix(tl, "ja") &&
		!strings.HasPrefix(tl, "ko") && tl != "jp" {
		return outputLangGuard(target) + body
	}
	return body
}
