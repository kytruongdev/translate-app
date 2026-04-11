package gateway

import (
	"strings"
)

// markdownPreserveRuleGPT — lighter version for GPT-family models that reliably
// follow format instructions without needing CJK-specific warnings.
const markdownPreserveRuleGPT = `IMPORTANT: Preserve ALL Markdown formatting exactly (# ## ### **bold** *italic* > - etc.)
Only translate text content; never translate or modify Markdown syntax.`

// properNounRule instructs the AI not to translate proper nouns — company names,
// government offices, project names, and personal names must be preserved in their
// original form. Administrative units keep the Vietnamese place name with an
// accurate English level label appended.
// This is a category-based rule, not a term-specific whitelist.
const properNounRule = `PROPER NOUNS (preserve — do not translate):
- Organization and company names: keep in original Vietnamese (e.g. "Tổng Công ty cổ phần X" stays as-is)
- Government agencies and offices: keep in original Vietnamese
- Real estate project names: keep in original Vietnamese
- Personal names: romanize by removing all Vietnamese diacritics (e.g. "Đặng Thị Hiền" → "Dang Thi Hien", "Nguyễn Văn A" → "Nguyen Van A"). Do NOT keep diacritics in English output.
- Administrative units: keep the Vietnamese place name (without diacritics) and append the correct English level (e.g. "Hoài Đức" → "Hoai Duc District", "Hà Nội" → "Hanoi City")

EXCEPTION — document type names and titles are NOT proper nouns; translate them:
- "GIẤY KHAI SINH" → "BIRTH CERTIFICATE"
- "Hợp đồng mua bán" → "Purchase Agreement"
- "Biên bản" → "Minutes" / "Record"
- Any Vietnamese document title must be translated to its English equivalent.`

// abbreviationRule instructs the AI to translate Vietnamese abbreviations to their
// full English meaning rather than retaining them as opaque Vietnamese acronyms.
const abbreviationRule = `ABBREVIATIONS:
Vietnamese abbreviations must be translated to their full English meaning.
Do not retain Vietnamese acronyms in the output (e.g. "CCCD số" → "ID Card No.", "UBND" → "People's Committee").`

// vietnameseDateRule explains the Vietnamese date-in-words pattern so the model
// does not misread "năm [X][Y]" (year seventy-something) as a generic time phrase.
const vietnameseDateRule = `VIETNAMESE WRITTEN-OUT DATES:
The pattern "Ngày [X] tháng [Y] năm [Z]" (or variants with words instead of numbers) is a specific calendar date — day X, month Y, year Z.
"năm bảy sáu" = year seventy-six (1976); "năm chín ba" = year ninety-three (1993), etc.
Translate these as natural English dates: e.g. "Ngày hai tháng tám năm bảy sáu" → "The second day of August, nineteen seventy-six (1976)".
Do NOT interpret them as relative time descriptions ("previous year", "next month", etc.).`

// buildCompletenessRule returns a rule that forbids summarising, abbreviating, or
// generating placeholder text — critical for long-form PDF/document chunks where
// GPT-4o-mini might otherwise write "[Summary continues]" or "[Figures continue]".
func buildCompletenessRule(target string) string {
	return "COMPLETENESS RULE (critical):\n" +
		"Translate EVERY sentence and EVERY item completely.\n" +
		"Do NOT summarise, skip, abbreviate, or replace any content with placeholders " +
		`(e.g. "[continues]", "[summary]", "[omitted]", "..." or similar).` + "\n" +
		"If the input contains mixed languages (e.g. a table with both Vietnamese and " +
		target + " columns), translate ALL non-" + target + " text into " + target + ".\n" +
		"Output ONLY the translated text — no commentary or meta-notes."
}

// BuildTranslationSystemPromptGPT builds the system prompt for GPT-family models
// (gpt-4o-mini, gpt-4o, …). Omits the heavy Chinese-drift guardrails that are
// only needed for Qwen/Ollama — keeps instructions clean and token-efficient.
func BuildTranslationSystemPromptGPT(sourceLang, targetLocale, style string, preserveMarkdown bool) string {
	target := TargetLangLabel(targetLocale)
	styleNorm := strings.ToLower(strings.TrimSpace(style))
	if styleNorm != "business" && styleNorm != "academic" {
		styleNorm = "casual"
	}

	var base string
	switch styleNorm {
	case "business":
		base = "You are a professional translator. Translate the text to " + target +
			" in a formal, clear, and professional tone suitable for business communication.\n" +
			"Preserve technical terms. Output ONLY the translated text, no explanations."
	case "academic":
		base = "You are a scholarly translator. Translate the text to " + target +
			" with precision and rigor, using domain-appropriate terminology.\n" +
			"Maintain logical structure and formal register. Output ONLY the translated text, no explanations."
	default:
		base = "You are a translator. Translate the text to " + target +
			" naturally and conversationally, using everyday language.\n" +
			"Output ONLY the translated text, no explanations."
	}

	base += "\n\n" + buildCompletenessRule(target)
	if preserveMarkdown {
		base += "\n\n" + markdownPreserveRuleGPT
	}
	return base
}

// BuildPDFBatchSystemPromptGPT builds the system prompt for batched PDF text
// segment translation using <<<N>>> markers. glossary is an optional pre-built
// glossary string injected for terminology consistency across batches.
// rules is an optional newline-joined list of active translation_rules content blocks.
func BuildPDFBatchSystemPromptGPT(targetLocale, glossary, rules string) string {
	target := TargetLangLabel(targetLocale)

	var sb strings.Builder
	sb.WriteString("You are a professional translator. Translate each segment to ")
	sb.WriteString(target)
	sb.WriteString(" in a formal, clear, and professional tone. Preserve technical terms.\n\n")

	if glossary != "" {
		sb.WriteString("GLOSSARY (critical — when a term appears here, use the exact translation provided, do not deviate):\n\n")
		sb.WriteString(glossary)
		sb.WriteString("\n\n")
	}

	sb.WriteString(properNounRule)
	sb.WriteString("\n\n")
	sb.WriteString(abbreviationRule)
	sb.WriteString("\n\n")
	sb.WriteString(vietnameseDateRule)
	sb.WriteString("\n\n")
	if rules != "" {
		sb.WriteString(rules)
		sb.WriteString("\n\n")
	}
	sb.WriteString(buildCompletenessRule(target))
	sb.WriteString("\n\nFORMAT RULE (critical):\n")
	sb.WriteString("The user message contains segments numbered with <<<N>>> markers.\n")
	sb.WriteString("Return each translated segment preceded by its marker, like this:\n\n")
	sb.WriteString("<<<1>>>\n[translation of segment 1]\n\n<<<2>>>\n[translation of segment 2]\n\n")
	sb.WriteString("IMPORTANT: Keep every <<<N>>> marker EXACTLY as written. Do NOT add commentary.")

	return sb.String()
}

// BuildPDFHTMLSystemPromptGPT builds the system prompt for individual PDF HTML
// segment (table) translation. glossary is an optional pre-built glossary string
// for terminology consistency. rules is an optional newline-joined list of active
// translation_rules content blocks.
func BuildPDFHTMLSystemPromptGPT(targetLocale, glossary, rules string) string {
	target := TargetLangLabel(targetLocale)

	var sb strings.Builder
	sb.WriteString("You are a professional translator. Translate the text to ")
	sb.WriteString(target)
	sb.WriteString(" in a formal, clear, and professional tone. Preserve technical terms.\n\n")

	if glossary != "" {
		sb.WriteString("GLOSSARY (critical — when a term appears here, use the exact translation provided, do not deviate):\n\n")
		sb.WriteString(glossary)
		sb.WriteString("\n\n")
	}

	sb.WriteString(properNounRule)
	sb.WriteString("\n\n")
	sb.WriteString(abbreviationRule)
	sb.WriteString("\n\n")
	sb.WriteString(vietnameseDateRule)
	sb.WriteString("\n\n")
	if rules != "" {
		sb.WriteString(rules)
		sb.WriteString("\n\n")
	}
	sb.WriteString(buildCompletenessRule(target))
	sb.WriteString("\n\nHTML PRESERVATION (critical):\n")
	sb.WriteString("The input contains HTML markup. Preserve ALL HTML tags and attributes exactly as-is.\n")
	sb.WriteString("Only translate the visible text content inside the tags.\n")
	sb.WriteString("Never translate, rename, or modify tag names, attribute names, or attribute values.")

	return sb.String()
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

	var base string
	switch styleNorm {
	case "business":
		base = "You are a professional translator. Translate each paragraph to " + target +
			" in a formal, clear, and professional tone suitable for business communication.\n" +
			"Preserve technical terms."
	case "academic":
		base = "You are a scholarly translator. Translate each paragraph to " + target +
			" with precision and rigor, using domain-appropriate terminology.\n" +
			"Maintain logical structure and formal register."
	default:
		base = "You are a translator. Translate each paragraph to " + target +
			" naturally and conversationally, using everyday language."
	}

	markerRule := "\n\nFORMAT RULE (critical):\n" +
		"The user message contains paragraphs numbered with <<<N>>> markers.\n" +
		"Return each translated paragraph preceded by its marker, like this:\n\n" +
		"<<<1>>>\n[translation of paragraph 1]\n\n<<<2>>>\n[translation of paragraph 2]\n\n" +
		"IMPORTANT: Keep every <<<N>>> marker EXACTLY as written — exactly 3 angle brackets on each side, same number N.\n" +
		"Do NOT add extra angle brackets. Do NOT remove markers. Do NOT add any explanation or commentary."

	return base + markerRule + "\n\n" + buildCompletenessRule(target)
}

// GlossaryExtractionResult is the structured JSON response from BuildGlossaryExtractionPrompt.
type GlossaryExtractionResult struct {
	DocType      string `json:"doc_type"`
	IsNewDocType bool   `json:"is_new_doc_type"`
	Glossary     []struct {
		Sources []string `json:"sources"`
		Target  string   `json:"target"`
	} `json:"glossary"`
}

// BuildGlossaryExtractionPrompt builds the system prompt for extracting glossary terms
// and detecting document type from raw OCR markdown.
// docTypesList is a comma-separated string of existing doc_type IDs.
func BuildGlossaryExtractionPrompt(docTypesList string) string {
	return `You are an Expert Terminologist and Knowledge Engineer. Your task is to analyze the provided text and extract a comprehensive Glossary of terms that are essential for an accurate and professional translation.

TASK 1 — DOCUMENT TYPE:
From this list of document types: "` + docTypesList + `"
Identify which type best matches the document. If none match, suggest a new type in English snake_case (e.g. "insurance_contract") and set is_new_doc_type to true.

TASK 2 — GLOSSARY EXTRACTION:
Extract ONLY terms from these categories:
- Acronyms & abbreviations with their full forms (e.g. "UBND" / "Ủy ban nhân dân")
- Official organization names, government bodies, notary offices that appear multiple times
- Contract party labels used repeatedly (Bên A → Party A, Bên B → Party B)
- Domain-specific technical terms that could be translated inconsistently across batches

OCR VARIANT DETECTION — for each term, group all surface forms of the SAME concept:
- Case variations (ALL CAPS vs Title Case)
- Potential OCR errors or misspellings of that same term
- Short forms vs full forms of the same concept

GROUPING RULE (critical):
One entry = one concept only. "sources" must contain ONLY different written forms of the EXACT SAME concept.
NEVER group different concepts, even if they appear near each other.

BAD — label grouped with value:   sources: ["Họ và tên", "Nguyễn Văn A"]
GOOD: omit both — labels are structural, names are data

BAD — different concepts grouped: sources: ["Văn phòng công chứng Đông Đô", "Công chứng viên"]
GOOD: two separate entries — one for the office, one for the job title

BAD — different place names:      sources: ["Kim Chung", "Di Trạch", "Hoài Đức"]
GOOD: three separate entries — one per place name

BAD — different parties grouped:  sources: ["Bên A", "Bên B"]
GOOD: two separate entries — "Bên A" → "Party A", "Bên B" → "Party B"

WHAT NOT TO EXTRACT — skip all of the following:
- Form field labels (Họ và tên, Địa chỉ, Mã số thuế, Ngày tháng năm, Tỉnh/TP, Quận/Huyện, ...)
- Personal names of individuals (buyers, sellers, signatories, witnesses, tax officers)
- Specific addresses, streets, wards, communes
- Tax codes, reference numbers, file IDs, phone numbers, dates, monetary amounts
- Common Vietnamese words with obvious translations (không, và, của, người mua, người bán, ...)
- Anything appearing only once that is clearly document-specific data

TRANSLATION STANDARD:
Provide the most formal, professionally accepted equivalent in the target language.
Avoid literal translations — use industry-standard legal or domain-appropriate terms.

Return ONLY valid JSON, no commentary:
{
  "doc_type": "legal",
  "is_new_doc_type": false,
  "glossary": [
    {
      "sources": ["UBND", "Ủy ban nhân dân", "Uỷ ban nhân dân"],
      "target": "People's Committee"
    },
    {
      "sources": ["Bên A"],
      "target": "Party A"
    },
    {
      "sources": ["Bên B"],
      "target": "Party B"
    }
  ]
}`
}
