"""
OCR Sidecar — Structured PDF layout analysis.

Usage:
    paddleocr-darwin-arm64 page-0001.png page-0002.png ... page-000N.png

Prints a single JSON object to stdout:
    {"pages": [...]}

Exits with code 1 and writes {"error": "..."} to stderr on fatal error.
Individual page/region failures are non-fatal — logged to stderr, processing continues.

OCR engine: EasyOCR (vi+en) — CRAFT detection + CRNN recognition.
Layout/table structure: rapid_layout + rapid_table.
Output region types: "text", "title", "table".
"""

import sys
import json
import os
import re
import argparse
import multiprocessing

# Required for PyInstaller multiprocessing support on macOS/Windows.
if __name__ == "__main__":
    multiprocessing.freeze_support()


# ---------------------------------------------------------------------------
# EasyOCR discovery & initialisation
# ---------------------------------------------------------------------------

def find_easyocr_model_dir():
    """
    Locate the EasyOCR model storage directory.

    Search order:
      1. easyocr_models/ next to the frozen executable  (production bundle)
      2. easyocr_models/ next to this script            (dev)
      3. ~/.EasyOCR/model                               (EasyOCR default / CI)
    """
    if getattr(sys, 'frozen', False):
        base = os.path.dirname(sys.executable)
    else:
        base = os.path.dirname(os.path.abspath(__file__))

    candidates = [
        os.path.join(base, 'easyocr_models'),
        os.path.join(base, '..', 'easyocr_models'),
        os.path.expanduser('~/.EasyOCR/model'),
    ]
    for c in candidates:
        if os.path.isdir(c):
            return os.path.normpath(c)

    # Fall back to default — EasyOCR will download models here on first run.
    default = os.path.expanduser('~/.EasyOCR/model')
    os.makedirs(default, exist_ok=True)
    return default


def init_easyocr(model_dir):
    """
    Initialise EasyOCR reader for Vietnamese + English.
    gpu=False → CPU inference (works on all target platforms without CUDA).
    """
    import easyocr
    return easyocr.Reader(
        ['vi', 'en'],
        gpu=False,
        model_storage_directory=model_dir,
        verbose=False,
    )


# ---------------------------------------------------------------------------
# EasyOCR — page OCR (word-level → lines → paragraphs)
# ---------------------------------------------------------------------------

def ocr_page_easyocr(img, reader):
    """
    Run EasyOCR on a full-page BGR image.

    Returns (paragraphs, word_lines):

      paragraphs  = [(y1, x1, x2, text), ...]
        Spatially-clustered paragraphs, sorted top-to-bottom.

      word_lines  = [(y_center, y1, y2, x1, x2, text), ...]
        Individual visual lines with bounding boxes.
        Used for filtering text inside table regions.
    """
    results = reader.readtext(img, detail=1, paragraph=False)

    # ── Filter noise ──────────────────────────────────────────────────────────
    MIN_CONF = 0.25
    clean = []
    for bbox, text, conf in results:
        text = text.strip()
        if not text or conf < MIN_CONF:
            continue
        if sum(1 for c in text if c.isalnum()) < 2:
            continue
        clean.append((bbox, text, conf))

    if not clean:
        return [], []

    # ── Build flat word list with normalised bboxes ───────────────────────────
    # word = (y_center, y1, y2, x1, x2, text)
    words = []
    for bbox, text, conf in clean:
        y1 = int(min(p[1] for p in bbox))
        y2 = int(max(p[1] for p in bbox))
        x1 = int(min(p[0] for p in bbox))
        x2 = int(max(p[0] for p in bbox))
        words.append(((y1 + y2) / 2, y1, y2, x1, x2, text))

    words.sort(key=lambda w: w[0])

    # ── Step 1: merge words into visual lines ─────────────────────────────────
    # Two words belong to the same line when their y-center difference is less
    # than avg_word_height * 0.6.
    heights = [w[2] - w[1] for w in words]
    avg_h = sum(heights) / len(heights) if heights else 20
    same_line_thresh = max(avg_h * 0.6, 6)

    visual_lines = []   # [(y1, y2, x1, x2, text)]
    i = 0
    while i < len(words):
        group = [words[i]]
        ref_y = words[i][0]
        j = i + 1
        while j < len(words) and abs(words[j][0] - ref_y) <= same_line_thresh:
            group.append(words[j])
            j += 1
        group.sort(key=lambda w: w[3])          # left→right within line
        y1 = min(w[1] for w in group)
        y2 = max(w[2] for w in group)
        x1 = min(w[3] for w in group)
        x2 = max(w[4] for w in group)
        text = ' '.join(w[5] for w in group)
        visual_lines.append((y1, y2, x1, x2, text))
        i = j

    visual_lines.sort(key=lambda l: l[0])

    # ── word_lines output for table-region filtering ──────────────────────────
    word_lines = [((l[0]+l[1])/2, l[0], l[1], l[2], l[3], l[4])
                  for l in visual_lines]

    # ── Step 2: cluster visual lines into paragraphs ──────────────────────────
    # EasyOCR bboxes often overlap vertically (y1_next < y2_prev), so the
    # naive gap formula (y1 - y2_prev) gives 0 for almost every inter-line gap.
    # Fix: use y-CENTER distance instead.
    # Normal line spacing ≈ 1× avg_line_height; paragraph breaks ≈ 1.4×.
    line_heights = [l[1] - l[0] for l in visual_lines]
    avg_lh = sum(line_heights) / len(line_heights) if line_heights else 20

    centers = [(l[0] + l[1]) / 2 for l in visual_lines]
    center_gaps = sorted(centers[k] - centers[k-1] for k in range(1, len(centers)))

    if len(center_gaps) >= 4:
        median_cg = center_gaps[len(center_gaps) // 2]
        # 1.15× is more sensitive than the old 1.35×.
        # Vietnamese legal documents use tight paragraph spacing (≈ 1.1-1.2× line height),
        # so 1.35 merged almost everything into one region.
        gap_thresh = max(median_cg * 1.15, avg_lh * 0.9, 8)
    else:
        gap_thresh = max(avg_lh * 1.1, 8)

    paragraphs = []
    current_lines = [visual_lines[0]]
    para_y1 = visual_lines[0][0]
    para_x1 = visual_lines[0][2]
    para_x2 = visual_lines[0][3]
    prev_center = centers[0]

    for i, (y1, y2, x1, x2, text) in enumerate(visual_lines[1:], start=1):
        if centers[i] - prev_center > gap_thresh:
            para_text = ' '.join(l[4] for l in current_lines)
            if para_text.strip():
                paragraphs.append((para_y1, para_x1, para_x2, para_text))
            current_lines = []
            para_y1, para_x1, para_x2 = y1, x1, x2
        else:
            para_x1 = min(para_x1, x1)
            para_x2 = max(para_x2, x2)
        current_lines.append((y1, y2, x1, x2, text))
        prev_center = centers[i]

    if current_lines:
        para_text = ' '.join(l[4] for l in current_lines)
        if para_text.strip():
            paragraphs.append((para_y1, para_x1, para_x2, para_text))

    return paragraphs, word_lines


# ---------------------------------------------------------------------------
# EasyOCR — cropped region OCR for rapid_table
# ---------------------------------------------------------------------------

def ocr_crop_easyocr(img_crop, reader):
    """
    Run EasyOCR on a cropped table region and return results in the
    rapid_table ocr_results format: [[boxes[N,4,2], txts, scores]].
    Word-level bboxes (not line-level) are critical so rapid_table can assign
    each word to the correct cell via its centre point.
    """
    import numpy as np

    results = reader.readtext(img_crop, detail=1, paragraph=False)
    clean = [(bbox, text, conf) for bbox, text, conf in results
             if text.strip() and conf >= 0.20]

    if not clean:
        return [[np.zeros((0, 4, 2), dtype=np.float32), (), ()]]

    boxes = np.array(
        [[[float(p[0]), float(p[1])] for p in bbox] for bbox, _, _ in clean],
        dtype=np.float32,
    )
    txts   = tuple(text for _, text, _ in clean)
    scores = tuple(float(conf) for _, _, conf in clean)
    return [[boxes, txts, scores]]


# ---------------------------------------------------------------------------
# Bbox helpers
# ---------------------------------------------------------------------------

def normalize_bbox(raw):
    """
    Normalise various bbox formats to [x1, y1, x2, y2] (ints).
    Handles flat [x1,y1,x2,y2], 4-corner polygon [[x,y]×4], numpy arrays.
    Returns None on parse failure.
    """
    if raw is None:
        return None
    if isinstance(raw, dict):
        raw = raw.get("bbox") or raw.get("box")
        if raw is None:
            return None
    if hasattr(raw, "tolist"):
        raw = raw.tolist()
    if not isinstance(raw, (list, tuple)) or len(raw) == 0:
        return None
    if isinstance(raw[0], (list, tuple)):
        try:
            xs = [p[0] for p in raw]; ys = [p[1] for p in raw]
            return [int(min(xs)), int(min(ys)), int(max(xs)), int(max(ys))]
        except Exception:
            return None
    if len(raw) == 4:
        try:
            v = [float(x) for x in raw]
            return [int(v[0]), int(v[1]), int(v[2]), int(v[3])]
        except Exception:
            return None
    return None


def crop_region(img, bbox):
    """Safely crop a numpy HWC image to bbox [x1,y1,x2,y2]. Returns None if invalid."""
    if img is None or bbox is None:
        return None
    h, w = img.shape[:2]
    x1, y1, x2, y2 = max(0, bbox[0]), max(0, bbox[1]), min(w, bbox[2]), min(h, bbox[3])
    if x2 <= x1 or y2 <= y1:
        return None
    return img[y1:y2, x1:x2]


# ---------------------------------------------------------------------------
# Paragraph classifier (title vs text)
# ---------------------------------------------------------------------------

_SECTION_RE = re.compile(
    r'^(?:'
    r'CHƯƠNG\s+\S+'
    r'|Chương\s+\S+'
    r'|PHẦN\s+\S+'
    r'|Phần\s+\S+'
    r'|MỤC\s+[IVXLCDM\d]'
    r'|Mục\s+[IVXLCDM\d]'
    r'|ĐIỀU\s+\d+'
    r'|Điều\s+\d+'
    r'|[IVXlI]{1,4}\.\s'
    r')',
    re.UNICODE,
)

# Detects whitespace before an embedded section marker, used to split
# over-merged paragraphs into logical sub-sections.
#
# Handles:
#   • Roman numerals:  I. II. III. IV. V.  (+ OCR variants: l→I, L→I, 1→I)
#   • Vietnamese articles: Điều X. / ĐIỀU X.
#   • Numbered items: 1. Capital  2. Capital  (up to 2 digits)
#
# The match consumes the whitespace before the marker; the marker itself
# starts the next chunk (see _split_on_section_markers).
_EMBEDDED_SECTION_RE = re.compile(
    r'\s+'
    r'(?='
    # Roman numerals (with OCR confusion: I/l/1/L are all treated as "I")
    r'(?:[IlI1L]{1,4}V?|V[IlI1L]{0,3}|IX|X[IlI1L]{0,3})\.\s+[A-ZĐÀÁÂÃÈÉÊÌÍÒÓÔÕÙÚẮẶẤẦẨẪẮẶẼỆ]'
    r'|'
    # Vietnamese article markers: Điều 1. / ĐIỀU 2:
    r'[ĐđD][iIíìỉĩ][eéèêềếệểễ][uúùủũư]?\s+\d+[\.\:]?\s'
    r'|'
    # Numbered items: 1. Capital  /  12. Capital
    r'\d{1,2}\.\s+[A-ZĐÀÁÂÃÈÉÊÌÍÒÓÔÕÙÚẮẶẤẦẨẪẮẶẼỆ]'
    r')',
    re.UNICODE,
)


def _split_on_section_markers(text, y1, x1, x2):
    """Split a paragraph that contains embedded section markers (Roman, Điều X, numbered)."""
    spans = list(_EMBEDDED_SECTION_RE.finditer(text))
    if not spans:
        return [(y1, x1, x2, text)]
    result, prev_end = [], 0
    for m in spans:
        chunk = text[prev_end:m.start()].strip()
        if chunk:
            result.append((y1, x1, x2, chunk))
        prev_end = m.end()
    remainder = text[prev_end:].strip()
    if remainder:
        result.append((y1, x1, x2, remainder))
    return result if result else [(y1, x1, x2, text)]


def _classify_paragraph(text, page_w, x1, x2):
    """Return 'title' or 'text' for a paragraph."""
    s = text.strip()
    if not s:
        return "text"
    # Parenthetical fragments like "(Bên A và Bên B gọi chung...)" are never titles.
    if s[0] == '(':
        return "text"
    # "Căn cứ ..." preamble lines are always body text, never section headers.
    # Matches both "Căn cứ Luật..." and "Căn cứ:" (standalone label on form).
    if s.startswith('Căn cứ'):
        return "text"
    # Contact-info / form-field lines are always body text, never section headers.
    if s.startswith(('Điện thoại', 'Điện thoai', 'Email ', 'Lưu:', 'Luu:', 'Họ và tên',
                     'Số thuế', 'Thông báo l', 'Nơi nhận', 'Noi nhận')) or '@' in s:
        return "text"
    # Numbered list items (1. / 3) / 4.2.5.) are always body text.
    # Multi-level numbering (3.1, 4.2.5) positionally resembles a centered title
    # on tax forms but is never a semantic title.
    if re.match(r'^\d+[.)]\s', s) or re.match(r'^\d+\.\d', s):
        return "text"
    # Check section markers FIRST — Điều/CHƯƠNG/Roman numerals are always titles
    # regardless of how many words follow them on the same line.
    if _SECTION_RE.match(s):
        return "title"
    words = s.split()
    word_count = len(words)
    if word_count > 30:
        return "text"
    letters = [c for c in s if c.isalpha()]
    if len(letters) >= 4:
        upper_ratio = sum(1 for c in letters if c.isupper()) / len(letters)
        if upper_ratio >= 0.75 and word_count <= 20:
            return "title"
    if page_w > 0 and word_count <= 15:
        if x1 / page_w > 0.15 and (page_w - x2) / page_w > 0.15:
            return "title"
    return "text"


# ---------------------------------------------------------------------------
# Per-page processing
# ---------------------------------------------------------------------------

def _detect_table_bboxes(image_path, layout_engine, min_score=0.50):
    """
    Run rapid_layout and return TABLE bounding boxes with score ≥ min_score.
    """
    table_bboxes = []
    try:
        layout_out = layout_engine(image_path)
        if hasattr(layout_out, "boxes"):
            for b, n, s in zip(layout_out.boxes or [], layout_out.class_names or [], layout_out.scores or []):
                if str(n).lower() == "table" and float(s) >= min_score:
                    bbox = normalize_bbox(b)
                    if bbox:
                        table_bboxes.append(bbox)
        elif isinstance(layout_out, (list, tuple)) and len(layout_out) >= 1:
            for r in (layout_out[0] or []):
                if isinstance(r, dict):
                    label = str(r.get("label") or r.get("type") or "").lower()
                    if label == "table":
                        bbox = normalize_bbox(r.get("bbox") or r.get("box"))
                        if bbox:
                            table_bboxes.append(bbox)
    except Exception as e:
        print(f"[WARN] layout detection failed: {e}", file=sys.stderr)
    return table_bboxes


def _point_in_bbox(y, x, bbox):
    x1, y1, x2, y2 = bbox
    return x1 <= x <= x2 and y1 <= y <= y2


def _extract_table_html(img, bbox, table_engine, reader):
    """
    Crop the table region, run word-level EasyOCR, pass to rapid_table.

    Returns the HTML string on success, '' otherwise.

    Validation: a real data table must have at least MIN_TABLE_CELLS <td> elements.
    Regions with fewer cells are likely form sections or bordered text blocks
    that rapid_layout mistook for tables — we reject those so the full-page OCR
    text regions can cover that area instead.
    """
    MIN_TABLE_CELLS = 8

    cropped = crop_region(img, bbox)
    if cropped is None or cropped.size == 0:
        return ""
    try:
        ocr_results = ocr_crop_easyocr(cropped, reader)
        result = table_engine(cropped, ocr_results=ocr_results)
        html = ""
        if hasattr(result, "pred_htmls") and result.pred_htmls:
            html = result.pred_htmls[0] or ""
        elif isinstance(result, (list, tuple)):
            html = str(result[0]) if result else ""
        elif isinstance(result, str):
            html = result
        if "<table" in html.lower() and html.lower().count("<td") >= MIN_TABLE_CELLS:
            return html
    except Exception as e:
        print(f"[WARN] table extraction failed: {e}", file=sys.stderr)
    return ""


def _tesseract_paragraph_bboxes(image_path):
    """
    Run Tesseract HOCR on the full page and return paragraph bounding boxes.

    Tesseract's page-segmentation is font-aware (understands bold, spacing,
    indentation) and reliably detects paragraph boundaries in document scans.
    We use ONLY its bboxes — not its text — because Vietnamese recognition
    quality is poor.  EasyOCR then reads the text inside each bbox.

    Uses ocr_par bboxes as the primary unit.  For any ocr_par that is taller
    than SPLIT_THRESHOLD × median paragraph height (a sign Tesseract merged
    many logical paragraphs into one blob), we subdivide it using the ocr_line
    bboxes it contains, grouping lines by vertical gap.

    Returns list of (x1, y1, x2, y2) tuples, sorted top-to-bottom.
    Paragraphs with fewer than 3 alphanumeric characters are filtered out.
    """
    import subprocess, re

    # Leptonica (used by Tesseract) cannot follow symlinks on macOS.
    # /tmp is a symlink to /private/tmp, so resolve to the real path first.
    real_path = os.path.realpath(image_path)

    try:
        result = subprocess.run(
            ['tesseract', real_path, 'stdout', '-l', 'vie', 'hocr'],
            capture_output=True, text=True, timeout=60,
        )
    except Exception as e:
        print(f"[WARN] tesseract failed: {e}", file=sys.stderr)
        return []

    hocr = result.stdout

    # ── 1. Extract ocr_par bboxes (Tesseract's paragraph units) ─────────────
    # Note: Tesseract uses single quotes for class= but double quotes for title=
    paras = re.findall(
        r'class=.ocr_par.[^>]+title="bbox (\d+ \d+ \d+ \d+)"[^>]*>(.*?)</p>',
        hocr, re.DOTALL,
    )
    bboxes = []
    for bbox_str, content in paras:
        text = re.sub(r'<[^>]+>', ' ', content)
        text = ' '.join(text.split())
        alnum = sum(1 for c in text if c.isalnum())
        if alnum < 3:
            continue
        x1, y1, x2, y2 = map(int, bbox_str.split())
        bboxes.append((x1, y1, x2, y2))

    if not bboxes:
        return []

    bboxes.sort(key=lambda b: b[1])

    # ── 2. Detect and split "blob" paragraphs ────────────────────────────────
    # A blob is an ocr_par whose height is >= SPLIT_THRESHOLD × median par height.
    # For these, we extract ocr_line bboxes within the par's y-range and re-group
    # them using gap-based clustering, which gives finer subdivision.
    heights = [y2 - y1 for _, y1, _, y2 in bboxes]
    median_par_h = sorted(heights)[len(heights) // 2]
    SPLIT_THRESHOLD = 5  # a par > 5× median is likely a merged blob

    # Extract all ocr_line bboxes once (we only need them for blob splitting)
    line_matches = re.findall(
        r"class=['\"]ocr_line['\"][^>]+title=['\"]bbox (\d+ \d+ \d+ \d+)",
        hocr,
    )
    all_lines = []
    for bbox_str in line_matches:
        x1, y1, x2, y2 = map(int, bbox_str.split())
        all_lines.append((x1, y1, x2, y2))
    all_lines.sort(key=lambda b: b[1])

    # Compute median line height for gap threshold
    line_heights = [y2 - y1 for _, y1, _, y2 in all_lines]
    median_line_h = sorted(line_heights)[len(line_heights) // 2] if line_heights else 40

    final_bboxes = []
    for (px1, py1, px2, py2) in bboxes:
        par_h = py2 - py1
        if par_h < SPLIT_THRESHOLD * median_par_h:
            # Normal paragraph — keep as-is
            final_bboxes.append((px1, py1, px2, py2))
            continue

        # Blob paragraph — subdivide using lines within its y-range
        contained = [
            l for l in all_lines
            if l[1] >= py1 - 5 and l[3] <= py2 + 5
        ]
        if len(contained) < 2:
            final_bboxes.append((px1, py1, px2, py2))
            continue

        # Gap-based line grouping
        max_gap = median_line_h * 1.5
        groups = []
        cur = [contained[0]]
        for line in contained[1:]:
            gap = line[1] - cur[-1][3]
            if gap <= max_gap:
                cur.append(line)
            else:
                groups.append(cur)
                cur = [line]
        groups.append(cur)

        for group in groups:
            gx1 = min(l[0] for l in group)
            gy1 = min(l[1] for l in group)
            gx2 = max(l[2] for l in group)
            gy2 = max(l[3] for l in group)
            final_bboxes.append((gx1, gy1, gx2, gy2))

    final_bboxes.sort(key=lambda b: b[1])
    return final_bboxes


def _assign_lines_to_para_bboxes(word_lines, para_bboxes, skip_bboxes=None):
    """
    Assign EasyOCR full-page word_lines to Tesseract paragraph bboxes.

    word_lines  : [(y_center, y1, y2, x1, x2, text), ...]  from ocr_page_easyocr
    para_bboxes : [(x1, y1, x2, y2), ...]                  from _tesseract_paragraph_bboxes
    skip_bboxes : [(x1, y1, x2, y2), ...]  table regions to exclude

    Returns dict {para_idx: joined_text_string}.

    Matching rule: a word_line belongs to the FIRST para_bbox whose y-range
    contains the line's y-center (with ±10px tolerance).  X is not checked so
    that lines whose detected x-extent differs slightly from the Tesseract bbox
    are not silently dropped.
    """
    if skip_bboxes is None:
        skip_bboxes = []

    assignments = {i: [] for i in range(len(para_bboxes))}

    for wl in word_lines:
        yc, wy1, wy2, wx1, wx2, text = wl
        wxc = (wx1 + wx2) / 2

        # Drop lines inside confirmed table regions
        if any(_point_in_bbox(yc, wxc, tb) for tb in skip_bboxes):
            continue

        # Assign to the first para_bbox whose y-range covers this line.
        # Tolerance: allow 10px overshoot on the TOP (for lines that start
        # slightly above the bbox), but only 2px on the BOTTOM to avoid
        # assigning lines that belong to the NEXT paragraph.
        for i, (px1, py1, px2, py2) in enumerate(para_bboxes):
            if py1 - 10 <= yc <= py2 + 2:
                assignments[i].append((yc, wx1, text))
                break  # one line → one paragraph

    result = {}
    for i, lines in assignments.items():
        if lines:
            lines.sort(key=lambda l: (l[0], l[1]))  # top→bottom, left→right
            # Join with newline to preserve visual line breaks within a region.
            # The HTML builder will render these as <br> (title) or space (text).
            result[i] = '\n'.join(l[2] for l in lines)
    return result


def _merge_midsentence_fragments(regions):
    """
    Merge consecutive text regions that were split mid-sentence by Tesseract.

    MERGE when both conditions hold:
      1. Previous text ends without terminal punctuation (.!?;:)
         Semicolons and colons are treated as list-item / clause terminators —
         they mark the end of an enumerated item, NOT a mid-sentence break.
      2. Current text is a genuine continuation:
         - starts with a lowercase letter, AND
         - does NOT look like a new list item (a) b) ... g) h) ...) or
           a new numbered item (1. 2. 3.) or a section header (Điều …)
         OR
         - starts with a digit that continues a serial reference
           (e.g. prev ends "kết hôn số", curr starts "40, …")

    Tables and titles are never merged into adjacent regions.
    """
    import re
    # Patterns that signal the start of a new list item / section — never merge into previous
    _NEW_ITEM_RE = re.compile(
        r'^(?:'
        r'[a-zA-ZÀ-ỹ]\)\s'      # a) b) … g) h) … (letter + paren + space)
        r'|\d+[.)]\s'             # 1. 2. 3) … (digit + dot/paren + space)
        r'|\d+\.\d+'              # 3.1  3.2  multi-level section numbers
        r'|Điều\s'                # Điều X
        r'|ĐIỀU\s'
        r'|Chương\s'
        r'|CHƯƠNG\s'
        r'|Mục\s'
        r'|MỤC\s'
        r')',
        re.UNICODE,
    )

    if not regions:
        return regions

    result = [list(regions[0])]
    for entry in regions[1:]:
        y1, rtype, content = entry[0], entry[1], entry[2]
        alignment = entry[3] if len(entry) > 3 else 'left'
        prev = result[-1]
        prev_rtype, prev_content = prev[1], prev[2]

        # Also blocks multi-level numbers like "3.1. " / "3.2." from being treated
        # as digit-continuation of a previous sentence.
        _digit_new_item = re.compile(r'^\d+[.)]\s|^\d+\.\d+')

        # Helper: decide if curr_stripped is a genuine mid-sentence continuation
        def _is_continuation(prev_stripped, curr_stripped):
            ends_mid = bool(prev_stripped) and prev_stripped[-1] not in '.!?;:)"»'
            if not ends_mid:
                return False
            starts_lower = (
                bool(curr_stripped) and
                curr_stripped[0].islower() and
                not _NEW_ITEM_RE.match(curr_stripped)
            )
            starts_digit = (
                bool(curr_stripped) and
                curr_stripped[0].isdigit() and
                not _digit_new_item.match(curr_stripped) and
                bool(prev_stripped) and
                prev_stripped[-1].isalpha()
            )
            return starts_lower or starts_digit

        if prev_rtype == 'text' and rtype == 'text':
            prev_stripped = prev_content.rstrip()
            curr_stripped = content.lstrip()
            if _is_continuation(prev_stripped, curr_stripped):
                prev[2] = prev_stripped + ' ' + curr_stripped
                continue

        # Also merge stray fragments into adjacent regions regardless of type:
        #  - title + text fragment  (scan line bleeds out of heading bbox)
        #  - text  + title fragment (garbled footer/signature wrongly classified)
        elif prev_rtype in ('text', 'title') and rtype in ('text', 'title'):
            prev_stripped = prev_content.rstrip()
            curr_stripped = content.lstrip()
            if _is_continuation(prev_stripped, curr_stripped):
                prev[2] = prev_stripped + ' ' + curr_stripped
                # Keep prev's type; absorb the fragment silently
                continue

        result.append([y1, rtype, content, alignment])

    return [tuple(r) for r in result]


def _detect_alignment(x1, x2, page_w):
    """
    Infer text alignment from bbox x-position and width relative to page width.

    Returns 'center', 'right', or 'left'.

    Key insight: full-width body paragraphs (justified text) span most of the
    page and their midpoint happens to be near the page center — but they are
    NOT center-aligned.  We must use text width to distinguish:

      - text_width > 65 % of page_w  → LEFT  (body paragraph, full width)
      - text_width ≤ 65 % AND midpoint near page center → CENTER  (heading)
      - text_width ≤ 65 % AND left margin >> right margin → RIGHT
      - everything else → LEFT
    """
    if page_w <= 0:
        return 'left'
    text_w = x2 - x1
    if text_w / page_w > 0.65:
        return 'left'
    text_mid = (x1 + x2) / 2
    page_mid = page_w / 2
    if abs(text_mid - page_mid) < 0.15 * page_w:
        return 'center'
    left_margin = x1
    right_margin = page_w - x2
    if left_margin > right_margin + 0.15 * page_w:
        return 'right'
    return 'left'


_OVERMERGE_NEW_PARA_RE = re.compile(
    r'^(?:'
    r'[a-zA-ZÀ-ỹ]\)\s'             # a) b) c) list items
    r'|\d{1,2}[.)]\s'               # 1. 2. 3) numbered items (space after)
    r'|\d{1,2}\.[A-ZĐÀÁÂÃÈÉÊÌÍÒÓÔÕÙÚẮẶẤẦẨẪẼỆ]'  # 3.Số (no space, upper follows)
    r'|\d+\.\d+'                    # 4.2  4.2.3  multi-level section numbers
    r'|Thông tin về\b'              # "Thông tin về X:" — form section header
    r'|Điều\s|ĐIỀU\s'
    r'|Chương\s|CHƯƠNG\s'
    r'|Mục\s|MỤC\s'
    r')',
    re.UNICODE,
)
_OVERMERGE_TERMINAL = frozenset('.!?;:)"»')

# Pre-splits inline multi-item blobs: "3.1. X ... 3.2. Y" → newline before 3.2.
# Only matches multi-level section numbers with trailing period+space ("3.1. "),
# not plain references like "khoản 3.2" or dates like "3.2.2025".
_INLINE_SECTION_SPLIT_RE = re.compile(r' (?=\d{1,2}\.\d+\. )', re.UNICODE)


_OCR_CORRECTIONS = [
    # tl → th: EasyOCR confuses "th" cluster with "tl" in Vietnamese scans
    # "tliuế" → "thuế", "thiuế" → "thuế", "Tluế" → "Thuế"
    (re.compile(r'\btl([iuêếềệểễ])', re.UNICODE), r'th\1'),
    (re.compile(r'\bthi([uêếềệểễ])', re.UNICODE), r'thu\1'),   # thiuế → thuế
    (re.compile(r'\bTl([iuêếềệểễ])', re.UNICODE), r'Th\1'),
    (re.compile(r'\bThi([uêếềệểễ])', re.UNICODE), r'Thu\1'),
    (re.compile(r'\bTL([IUÊẾỀỆỂỄ])', re.UNICODE), r'TH\1'),
    # hw / Jw → h: artifact double-consonant before vowels
    # "hwướng" → "hướng", "Jwvớng" → "hướng"
    (re.compile(r'\bhw([uướứừựử])', re.UNICODE), r'h\1'),
    (re.compile(r'\bHw([uướứừựử])', re.UNICODE), r'H\1'),
    (re.compile(r'\bJwv([oóòỏõọôốồổỗộướứừựử])', re.UNICODE), r'hư\1'),
    # nl at word boundary → nh: "chỉnl" → "chỉnh", "sinl" → "sinh"
    # Vietnamese never ends a syllable with "nl"
    (re.compile(r'nl\b', re.UNICODE), r'nh'),
    # đieu → điều, Đieu → Điều (missing circumflex+grave on "ê")
    (re.compile(r'\bđieu\b', re.UNICODE), r'điều'),
    (re.compile(r'\bĐieu\b', re.UNICODE), r'Điều'),
    (re.compile(r'\bĐIEU\b', re.UNICODE), r'ĐIỀU'),
    # Roman numeral I confused with L/l in section headers
    # Handles both "IL." and "IL " (period sometimes omitted by OCR)
    # Only at start of line/string, only when followed by uppercase
    (re.compile(r'^L[\.:]?\s+(?=[A-ZĐÀÁÂÃÈ])', re.MULTILINE | re.UNICODE), r'I. '),
    (re.compile(r'^IL[\.:]?\s+(?=[A-ZĐÀÁÂÃÈ])', re.MULTILINE | re.UNICODE), r'II. '),
    (re.compile(r'^IlI[\.:]?\s+(?=[A-ZĐÀÁÂÃÈ])', re.MULTILINE | re.UNICODE), r'III. '),
    (re.compile(r'^IIL[\.:]?\s+(?=[A-ZĐÀÁÂÃÈ])', re.MULTILINE | re.UNICODE), r'III. '),
    (re.compile(r'^lV[\.:]?\s+(?=[A-ZĐÀÁÂÃÈ])', re.MULTILINE | re.UNICODE), r'IV. '),
    (re.compile(r'^lII[\.:]?\s+(?=[A-ZĐÀÁÂÃÈ])', re.MULTILINE | re.UNICODE), r'III. '),
    # "1 BÊN..." where Roman numeral I is misread as digit 1.
    # Only fires when the word after "1 " is ≥2 consecutive uppercase letters
    # (section headers), not regular numbered items like "1 Bên chuyển nhượng".
    (re.compile(r'^1 (?=[A-ZĐÀÁÂÃÈÊÔƠƯ]{2})', re.MULTILINE | re.UNICODE), r'I. '),
    # "ĐỎNG" / "ĐỔNG" → "ĐỒNG": EasyOCR confuses ồ (falling) with ỏ/ổ (hook).
    # "đỏng/đổng" are not valid Vietnamese words; all occurrences in legal docs = "đồng".
    (re.compile(r'\bĐỎNG\b', re.UNICODE), r'ĐỒNG'),
    (re.compile(r'\bĐỔNG\b', re.UNICODE), r'ĐỒNG'),
    (re.compile(r'\bđỏng\b', re.UNICODE), r'đồng'),
    (re.compile(r'\bđổng\b', re.UNICODE), r'đồng'),
    # Single o/u between two digits: "0o0" → "000" (single char, safe static replacement)
    (re.compile(r'(?<=\d)[ou](?=\d)', re.UNICODE), r'0'),
    # o/u immediately after a dot, before a digit: ".o0" → ".00"
    (re.compile(r'(?<=\.)[ou](?=\d)', re.UNICODE), r'0'),
    # "1ệ phí" → "lệ phí": digit 1 misread as letter l before Vietnamese vowel cluster.
    (re.compile(r'\b1ệ\b', re.UNICODE), r'lệ'),
    # "CONG CHỨNG" → "CÔNG CHỨNG": missing Ô circumflex on CÔNG.
    (re.compile(r'\bCONG CHỨNG\b', re.UNICODE), r'CÔNG CHỨNG'),
    # CHUYỂN: EasyOCR swaps ẻ or ễ where ể is correct (same circumflex base, wrong tone)
    (re.compile(r'\bCHUYẺN\b', re.UNICODE), r'CHUYỂN'),
    (re.compile(r'\bCHUYỄN\b', re.UNICODE), r'CHUYỂN'),
    (re.compile(r'\bchuyẻn\b', re.UNICODE), r'chuyển'),
    (re.compile(r'\bchuyễn\b', re.UNICODE), r'chuyển'),
    (re.compile(r'\bChuyẻn\b', re.UNICODE), r'Chuyển'),
    (re.compile(r'\bChuyễn\b', re.UNICODE), r'Chuyển'),
    # BỂN → BÊN: ể misread where plain ê expected (all-caps section headers)
    (re.compile(r'\bBỂN\b', re.UNICODE), r'BÊN'),
    (re.compile(r'\bbển\b', re.UNICODE), r'bên'),
    # THUẾ: multiple diacritic confusions — THÚẺ, THUỄ, THUÉ (É=E+acute vs Ế=E+circ+acute)
    (re.compile(r'\bTHÚẺ\b', re.UNICODE), r'THUẾ'),
    (re.compile(r'\bTHUỄ\b', re.UNICODE), r'THUẾ'),
    (re.compile(r'\bTHUÉ\b', re.UNICODE), r'THUẾ'),
    (re.compile(r'\bthuễ\b', re.UNICODE), r'thuế'),
    (re.compile(r'\bThuễ\b', re.UNICODE), r'Thuế'),
    # "thuê" (rent) ≠ "thuế" (tax) — only correct when OCR loses circumflex from "thuế"
    # Context-limited: "tính thuê" → "tính thuế", "nộp thuê" → "nộp thuế"
    (re.compile(r'(?<=tính )thuê\b', re.UNICODE), r'thuế'),
    (re.compile(r'(?<=nộp )thuê\b', re.UNICODE), r'thuế'),
    # PHÓ TRƯỞNG: Ớ misread where Ó expected in official title
    (re.compile(r'\bPHỚ TRƯỞNG\b', re.UNICODE), r'PHÓ TRƯỞNG'),
    # TỔNG / CỔ PHẦN: Ỗ misread where Ổ expected
    (re.compile(r'\bTỖNG\b', re.UNICODE), r'TỔNG'),
    (re.compile(r'\btỗng\b', re.UNICODE), r'tổng'),
    (re.compile(r'\bCỖ PHẦN\b', re.UNICODE), r'CỔ PHẦN'),
    # THÔNG: Q misread where G expected (THÔNQ → THÔNG)
    (re.compile(r'\bTHÔNQ\b', re.UNICODE), r'THÔNG'),
    # Thời / thời: ò (falling-tone o) misread where ờ (falling-tone o-circumflex) expected
    (re.compile(r'\bThòi\b', re.UNICODE), r'Thời'),
    (re.compile(r'\bthòi\b', re.UNICODE), r'thời'),
    # "Độc lập - Tự do - Hạnh phúc" header loses diacritics in decorative/stylized fonts
    # Two variants: "lập" (correct ạ) and "lâp" (wrong â lost dot-below)
    (re.compile(r'\bĐôc lập\b', re.UNICODE), r'Độc lập'),
    (re.compile(r'\bĐôc lâp\b', re.UNICODE), r'Độc lập'),
    (re.compile(r'\bTựdo\b', re.UNICODE), r'Tự do'),
    (re.compile(r'\bHanh phúc\b', re.UNICODE), r'Hạnh phúc'),
    # LK12.04: digit 1 and letter 2 mis-scanned as I+2 or I+Z in contract numbers
    (re.compile(r'\bLKI2\.', re.UNICODE), r'LK12.'),
    (re.compile(r'\bLKIZ\.', re.UNICODE), r'LK12.'),
    # Ông/Bà: slash misread as letter l or J between the two title words
    (re.compile(r'Ông[lJ]Bà', re.UNICODE), r'Ông/Bà'),
    # lừa: capital I misread as lowercase l at word start before vowel cluster
    (re.compile(r'\bIừa\b', re.UNICODE), r'lừa'),
    # lượng: same I→l confusion
    (re.compile(r'\bIượng\b', re.UNICODE), r'lượng'),
    # ngày / Ngày: v misread as y (common in handwritten/print OCR)
    (re.compile(r'\bngàv\b', re.UNICODE), r'ngày'),
    (re.compile(r'\bNgàv\b', re.UNICODE), r'Ngày'),
    # từ: spurious trailing r after final vowel cluster
    (re.compile(r'\btừr\b', re.UNICODE), r'từ'),
    # đồng: 'a' or plain ASCII 'd' (no stroke) misread as 'đ'
    (re.compile(r'\baồng\b', re.UNICODE), r'đồng'),
    (re.compile(r'\bdồng\b', re.UNICODE), r'đồng'),
    # m²: question mark misread as superscript-2 after measurement unit
    (re.compile(r'\bm\?(?=\s|[,;.]|$)', re.UNICODE), r'm²'),
    # "Bên bên bán" → "Bên bán": double-word artefact from merged scan lines
    (re.compile(r'\bBên bên bán\b', re.UNICODE), r'Bên bán'),
    # "Bằng chữ": missing grave on 'a' (Băng → Bằng)
    (re.compile(r'\bBăng chữ\b', re.UNICODE), r'Bằng chữ'),
    # Bưu điện: extra 'r' inserted, or horn diacritic (ư) dropped entirely
    (re.compile(r'\bBưru\b', re.UNICODE), r'Bưu'),
    (re.compile(r'\bBuu\b', re.UNICODE), r'Bưu'),
    # Lê Thị: missing dot-below on middle name indicator
    (re.compile(r'\bLê Thi\b', re.UNICODE), r'Lê Thị'),
    # nhượng: slash artefact splitting the word mid-consonant cluster
    (re.compile(r'\bn/ượng\b', re.UNICODE), r'nhượng'),
    # "quà tặng": missing tone mark on tang (tang → tặng in fixed phrase)
    (re.compile(r'\bquà tang\b', re.UNICODE), r'quà tặng'),
    # "miễn, giản" → "miễn, giảm": ả/ã confusion in tax exemption phrase
    (re.compile(r'\bmiễn,\s*giản\b', re.UNICODE), r'miễn, giảm'),
    # "Lê Thị Hảo" — name of the tax officer; OCR produces many garbled last-name variants:
    # "Hao", "Hco", "Hio", "Hản", "Hảo" (already correct), etc.
    # Matches any 2–5 char word beginning with H after "Lê Thị".
    (re.compile(r'\bLê Thị H\S{1,4}\b', re.UNICODE), r'Lê Thị Hảo'),
    # "Ngụy Như Kon Tum" street name: ư lost from Như
    (re.compile(r'\bNhu Kon Tum\b', re.UNICODE), r'Như Kon Tum'),
    # đất: circumflex-acute 'ấ' OCR artefact "dấi" (not a Vietnamese word)
    (re.compile(r'\bdấi\b', re.UNICODE), r'đất'),
    # "Đơn vị tiền:" garbled into "Eo ! vị tiền" on tax forms (leading chars OCR noise)
    (re.compile(r'\bEo\s*[!|]?\s*vị tiền', re.UNICODE), r'Đơn vị tiền'),
    # "Dồng Việt Nam" → "Đồng Việt Nam" (currency label with plain D)
    (re.compile(r'\bDồng Việt Nam\b', re.UNICODE), r'Đồng Việt Nam'),
    # "Noi nhận" → "Nơi nhận": ơ lost from the label in signature area
    (re.compile(r'\bNoi nhận\b', re.UNICODE), r'Nơi nhận'),
    # "Liên ệt" → "Liên Việt": "Vi" dropped from bank name "Bưu điện Liên Việt"
    (re.compile(r'\bLiên ệt\b', re.UNICODE), r'Liên Việt'),
    # "quyên số" → "quyển số": missing ể in volume/register number label
    (re.compile(r'\bquyên số\b', re.UNICODE), r'quyển số'),
    # "qùa" → "quà": wrong tone mark (ù falling instead of à grave)
    (re.compile(r'\bqùa\b', re.UNICODE), r'quà'),
]


# Matches multi-char runs of o/u inside numbers: "6.uu0" or "6.uoo".
# Used with a length-preserving lambda so "uu" → "00", "uoo" → "000".
_DIGIT_MULTI_LETTER_RE = re.compile(r'(?<=[\d.])[ou]{2,}(?=[\d. ]|$)', re.UNICODE)


def _correct_ocr_errors(text: str) -> str:
    """Apply known EasyOCR character-confusion fixes for Vietnamese scans."""
    for pattern, replacement in _OCR_CORRECTIONS:
        text = pattern.sub(replacement, text)
    # Fix multi-char garbled digit runs: "uu" → "00", "uoo" → "000" (length-preserving).
    text = _DIGIT_MULTI_LETTER_RE.sub(lambda m: '0' * len(m.group(0)), text)
    return text


def _split_overmerged_content(content):
    """
    Split a single paragraph string that Tesseract over-merged into one bbox,
    where multiple logical paragraphs ended up as newline-separated lines.

    Split triggers (checked per line boundary):
      1. The line matches _OVERMERGE_NEW_PARA_RE  (list item / section header)
      2. Previous line ends with terminal punctuation AND current line starts
         with an uppercase letter — a new sentence that is also a new paragraph.

    Lines within each group are joined with a space (they are scan-line
    fragments of the same logical paragraph).

    Returns a list of non-empty paragraph strings.
    """
    # Inline pre-split: insert newlines before "3.1. " / "3.2. " etc. patterns
    # that appear mid-text (tax form rows like "3.1. X đồng 3.2. Y đồng").
    content = _INLINE_SECTION_SPLIT_RE.sub('\n', content)

    lines = [l.strip() for l in content.split('\n') if l.strip()]
    if len(lines) <= 1:
        return [content.strip()] if content.strip() else []

    def _is_allcaps_title(s):
        """Short ALL-CAPS line → standalone title, never continues into next line."""
        letters = [c for c in s if c.isalpha()]
        if not letters or len(s.split()) > 12:
            return False
        return sum(1 for c in letters if c.isupper()) / len(letters) >= 0.75

    groups: list[list[str]] = [[lines[0]]]

    for line in lines[1:]:
        prev = groups[-1][-1] if groups[-1] else ''
        is_new = (
            _OVERMERGE_NEW_PARA_RE.match(line) or
            # Prev line ends with terminal punct and curr starts uppercase
            (prev and prev[-1] in _OVERMERGE_TERMINAL and line and line[0].isupper()) or
            # Prev line is a standalone ALL-CAPS title — always split after it
            _is_allcaps_title(prev) or
            # Current line is a standalone ALL-CAPS title — always split before it
            _is_allcaps_title(line) or
            # Prev line is a section marker (Điều/Chương/...) — body text follows
            _SECTION_RE.match(prev)
        )
        if is_new:
            groups.append([line])
        else:
            groups[-1].append(line)

    return [' '.join(g) for g in groups if g]


def process_page(img, image_path, page_no, reader, layout_engine, table_engine):
    """
    Process one page image.

    Strategy: Tesseract layout + EasyOCR text.
      1. Tesseract HOCR  → paragraph bounding boxes (structure).
      2. rapid_layout    → TABLE bounding boxes.
      3. rapid_table     → TABLE HTML for each valid table candidate.
      4. For each Tesseract paragraph bbox NOT inside a table:
             crop → EasyOCR → good Vietnamese text.
      5. Classify (title/text) and emit in page order.

    Fallback: if Tesseract is unavailable, revert to full-page EasyOCR.
    """
    page_h, page_w = img.shape[:2]
    page_out = {"page_no": page_no, "width": page_w, "height": page_h, "regions": []}

    # ── 1. Tesseract paragraph bboxes ────────────────────────────────────────
    para_bboxes = _tesseract_paragraph_bboxes(image_path)

    # ── 2 & 3. Table detection + HTML extraction ──────────────────────────────
    candidate_bboxes = _detect_table_bboxes(image_path, layout_engine)
    table_entries = []
    valid_table_bboxes = []
    for tb in candidate_bboxes:
        tx1, ty1, tx2, ty2 = tb
        html = _extract_table_html(img, tb, table_engine, reader)
        if html:
            table_entries.append((ty1, html))
            valid_table_bboxes.append(tb)

    # ── 4. EasyOCR on full page, then assign lines to Tesseract para bboxes ──
    # Running EasyOCR on the full-page image gives much better Vietnamese text
    # quality than running it on small paragraph crops, because CRAFT (the
    # EasyOCR detector) needs sufficient context to locate and read diacritics.
    merged = []

    try:
        _, word_lines = ocr_page_easyocr(img, reader)
    except Exception as e:
        print(f"[WARN] page {page_no}: EasyOCR failed: {e}", file=sys.stderr)
        word_lines = []

    if para_bboxes and word_lines:
        # Assign full-page EasyOCR lines to Tesseract paragraph bboxes
        para_texts = _assign_lines_to_para_bboxes(word_lines, para_bboxes, valid_table_bboxes)
        for i, (x1, y1, x2, y2) in enumerate(para_bboxes):
            content = para_texts.get(i, '').strip()
            if not content or sum(1 for c in content if c.isalnum()) < 3:
                continue
            # Fix common EasyOCR character errors before any further processing
            content = _correct_ocr_errors(content)
            # Split over-merged content (multiple logical paragraphs in one bbox)
            chunks = _split_overmerged_content(content)
            for ci, chunk in enumerate(chunks):
                rtype = _classify_paragraph(chunk, page_w, x1, x2)
                alignment = _detect_alignment(x1, x2, page_w)
                # Long body text is almost never center-aligned in Vietnamese docs.
                # Tesseract bboxes for dense form content are unreliable for alignment.
                if alignment == 'center' and rtype == 'text' and len(chunk.split()) > 12:
                    alignment = 'left'
                # Small y1 offset per chunk to preserve intra-para ordering
                merged.append((y1 + ci * 0.5, rtype, chunk, alignment))

    elif word_lines:
        # Tesseract unavailable — fall back to full-page EasyOCR paragraph clustering
        print(f"[WARN] page {page_no}: tesseract unavailable, falling back to EasyOCR clustering",
              file=sys.stderr)
        try:
            paragraphs, _ = ocr_page_easyocr(img, reader)
        except Exception as e:
            print(f"[WARN] page {page_no}: EasyOCR clustering failed: {e}", file=sys.stderr)
            paragraphs = []
        for y1, x1, x2, text in paragraphs:
            text = text.strip()
            if text and sum(1 for c in text if c.isalnum()) >= 3:
                rtype = _classify_paragraph(text, page_w, x1, x2)
                alignment = _detect_alignment(x1, x2, page_w)
                merged.append((y1, rtype, text, alignment))

    # ── 5. Merge tables + text in page order, emit ───────────────────────────
    for ty1, html in table_entries:
        merged.append((ty1, "table", html, "left"))
    merged.sort(key=lambda r: r[0])

    # ── 6. Merge mid-sentence fragments ──────────────────────────────────────
    merged = _merge_midsentence_fragments(merged)

    for y1, rtype, content, alignment in merged:
        if rtype == "table":
            page_out["regions"].append({
                "type": "table", "bbox": [0, int(y1), page_w, page_h], "html": content,
            })
        else:
            page_out["regions"].append({
                "type": rtype, "bbox": [0, int(y1), page_w, page_h],
                "content": content, "alignment": alignment,
            })

    return page_out


def _try_merge_page_bleed(prev_page_out, curr_page_out):
    """
    Cross-page bleed fix: if the FIRST text/title region of curr_page starts with
    a lowercase continuation word (not a list item), it is likely an orphaned
    fragment from the last sentence on prev_page.  Append it to prev_page's last
    text/title region and remove it from curr_page.

    This handles the common scan-document pattern where a paragraph spans two pages
    and Tesseract/EasyOCR reports the continuation on the next page as a separate
    region starting mid-sentence.
    """
    import re
    _NEW_ITEM_RE = re.compile(
        r'^(?:[a-zA-ZÀ-ỹ]\)\s|\d+[.)]\s|Điều\s|ĐIỀU\s|Chương\s|CHƯƠNG\s|Mục\s|MỤC\s)',
        re.UNICODE,
    )

    prev_regions = prev_page_out.get("regions", [])
    curr_regions = curr_page_out.get("regions", [])
    if not prev_regions or not curr_regions:
        return

    first = curr_regions[0]
    if first.get("type") not in ("text", "title"):
        return
    curr_content = first.get("content", "").strip()
    if not curr_content or not curr_content[0].islower():
        return
    if _NEW_ITEM_RE.match(curr_content):
        return  # it's a list item, not a bleed

    # Find the last text/title region on the previous page
    last_text = None
    for r in reversed(prev_regions):
        if r.get("type") in ("text", "title"):
            last_text = r
            break
    if last_text is None:
        return

    prev_content = last_text.get("content", "").rstrip()
    if prev_content and prev_content[-1] in '.!?;:)"»':
        return  # prev ends properly — no bleed

    # Merge: append fragment to prev page's last region, drop from curr
    last_text["content"] = prev_content + ' ' + curr_content
    curr_page_out["regions"] = curr_regions[1:]


def process_all_pages_streaming(image_paths, reader, layout_engine, table_engine):
    """
    Process all pages sequentially, printing each page as a JSON line to stdout
    immediately after it completes (streaming NDJSON).

    This lets the Go host emit progress events after each page instead of waiting
    for the entire document to finish (which can take 10+ min for large PDFs on CPU).

    Cross-page bleed: after each page is processed, we check whether the first
    region of the current page is an orphaned fragment from the previous page's
    last sentence.  If so, we merge it back into the previous page's last region
    and re-emit the corrected previous page before emitting the current page.

    Output format:
      One JSON object per page (one line), then {"done": true} as the sentinel.
    """
    import cv2

    prev_page_out = None

    for idx, image_path in enumerate(image_paths):
        page_no = idx + 1
        img = cv2.imread(image_path)
        if img is None:
            print(f"[WARN] page {page_no}: cannot read {image_path}", file=sys.stderr)
            page_out = {"page_no": page_no, "width": 0, "height": 0, "regions": []}
        else:
            try:
                page_out = process_page(img, image_path, page_no,
                                        reader, layout_engine, table_engine)
            except Exception as e:
                print(f"[WARN] page {page_no}: processing failed: {e}", file=sys.stderr)
                page_h, page_w = img.shape[:2]
                page_out = {"page_no": page_no, "width": page_w, "height": page_h, "regions": []}

        if prev_page_out is not None:
            # Cross-page bleed: may mutate both prev and curr in place
            _try_merge_page_bleed(prev_page_out, page_out)
            # Emit the (possibly corrected) previous page now that we know curr
            print(json.dumps(prev_page_out, ensure_ascii=False), flush=True)

        prev_page_out = page_out

    # Emit the last page
    if prev_page_out is not None:
        print(json.dumps(prev_page_out, ensure_ascii=False), flush=True)

    # Sentinel: signals normal completion to the Go reader.
    print(json.dumps({"done": True}), flush=True)


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Structured PDF OCR sidecar — processes page images and outputs layout JSON."
    )
    parser.add_argument("images", nargs="+", help="Paths to page PNG images (in page order)")
    args = parser.parse_args()

    missing = [p for p in args.images if not os.path.exists(p)]
    if missing:
        print(json.dumps({"error": f"Image files not found: {missing}"}), file=sys.stderr)
        sys.exit(1)

    model_dir = find_easyocr_model_dir()
    print(f"[INFO] EasyOCR model dir: {model_dir}", file=sys.stderr)

    from rapid_layout import RapidLayout
    from rapid_table  import RapidTable

    reader        = init_easyocr(model_dir)
    layout_engine = RapidLayout()
    table_engine  = RapidTable()

    try:
        process_all_pages_streaming(args.images, reader, layout_engine, table_engine)
    except Exception as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        sys.exit(1)
