package gateway

import (
	"strings"
)

// markdownPreserveRuleGPT — lighter version for GPT-family models that reliably
// follow format instructions without needing CJK-specific warnings.
const markdownPreserveRuleGPT = `IMPORTANT: Preserve ALL Markdown formatting exactly (# ## ### **bold** *italic* > - etc.)
Only translate text content; never translate or modify Markdown syntax.`

// BuildTranslationSystemPromptGPT builds the system prompt for GPT-family models
// (gpt-4o-mini, gpt-4o, …). Omits the heavy Chinese-drift guardrails that are
// only needed for Qwen/Ollama — keeps instructions clean and token-efficient.
func BuildTranslationSystemPromptGPT(sourceLang, targetLocale, style string, preserveMarkdown bool) string {
	target := TargetLangLabel(targetLocale)
	styleNorm := strings.ToLower(strings.TrimSpace(style))
	if styleNorm != "business" && styleNorm != "academic" {
		styleNorm = "casual"
	}

	srcKey := sourceLangKey(sourceLang)
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
		base = "You are a professional translator. Translate the text " + fromClause + "to " + target +
			" in a formal, clear, and professional tone suitable for business communication.\n" +
			"Preserve technical terms. Output ONLY the translated text, no explanations."
	case "academic":
		base = "You are a scholarly translator. Translate the text " + fromClause + "to " + target +
			" with precision and rigor, using domain-appropriate terminology.\n" +
			"Maintain logical structure and formal register. Output ONLY the translated text, no explanations."
	default:
		base = "You are a translator. Translate the text " + fromClause + "to " + target +
			" naturally and conversationally, using everyday language.\n" +
			"Output ONLY the translated text, no explanations."
	}

	if preserveMarkdown {
		base += "\n\n" + markdownPreserveRuleGPT
	}
	return base
}

// BuildDocxBatchSystemPromptGPT builds the DOCX batch system prompt for GPT-family
// models. Keeps the <<<N>>> format rule (still needed) but drops Qwen-specific
// Chinese-drift constraints.
func BuildDocxBatchSystemPromptGPT(from, to, style string) string {
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
			"Preserve technical terms."
	case "academic":
		base = "You are a scholarly translator. Translate each paragraph " + fromClause + "to " + target +
			" with precision and rigor, using domain-appropriate terminology.\n" +
			"Maintain logical structure and formal register."
	default:
		base = "You are a translator. Translate each paragraph " + fromClause + "to " + target +
			" naturally and conversationally, using everyday language."
	}

	markerRule := "\n\nFORMAT RULE (critical):\n" +
		"The user message contains paragraphs numbered with <<<N>>> markers.\n" +
		"Return each translated paragraph preceded by its marker, like this:\n\n" +
		"<<<1>>>\n[translation of paragraph 1]\n\n<<<2>>>\n[translation of paragraph 2]\n\n" +
		"IMPORTANT: Keep every <<<N>>> marker EXACTLY as written — exactly 3 angle brackets on each side, same number N.\n" +
		"Do NOT add extra angle brackets. Do NOT remove markers. Do NOT add any explanation or commentary."

	return base + markerRule
}
