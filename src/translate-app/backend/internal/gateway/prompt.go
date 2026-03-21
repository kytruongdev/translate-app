package gateway

import (
	"strings"
)

// Target language labels for prompts (architecture §5.4 / §9.1).
var targetLangNames = map[string]string{
	"en-US": "English (United States)",
	"en-GB": "English (United Kingdom)",
	"en-AU": "English (Australia)",
	"ko":    "Korean",
	"ja":    "Japanese",
	"zh-CN": "Simplified Chinese",
	"zh-TW": "Traditional Chinese",
	"fr":    "French",
	"de":    "German",
	"es":    "Spanish",
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
Only translate text content, never translate or modify Markdown syntax.`

// BuildTranslationSystemPrompt builds the system instruction per doc/architecture-document.md §9.1.
func BuildTranslationSystemPrompt(sourceLang, targetLocale, style string, preserveMarkdown bool) string {
	target := TargetLangLabel(targetLocale)
	styleNorm := strings.ToLower(strings.TrimSpace(style))
	if styleNorm != "business" && styleNorm != "academic" {
		styleNorm = "casual"
	}

	var base string
	if sourceLang == "vi" {
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

	if preserveMarkdown {
		return base + "\n\n" + markdownPreserveRule
	}
	return base
}
