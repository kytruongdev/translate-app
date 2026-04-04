"""
OCR Sidecar — Structured PDF layout analysis.

Usage:
    paddleocr-darwin-arm64 page-0001.png page-0002.png ... page-000N.png

Prints a single JSON object to stdout:
    {"pages": [...]}

Exits with code 1 and writes {"error": "..."} to stderr on fatal error.
Individual page/region failures are non-fatal — logged to stderr, processing continues.

OCR engine: Tesseract (vie+eng) via pytesseract.
Layout/table structure: rapid_layout + rapid_table.
"""

import sys
import json
import os
import math
import re
import argparse
import multiprocessing

# Required for PyInstaller multiprocessing support on macOS/Windows.
if __name__ == "__main__":
    multiprocessing.freeze_support()


# ---------------------------------------------------------------------------
# Tesseract discovery
# ---------------------------------------------------------------------------

def find_tesseract_cmd():
    """
    Locate the tesseract binary. Search order:
      1. Same directory as this script/frozen executable  (production app bundle)
      2. bin/ sibling of this script's parent dir         (dev: backend/bin/tesseract)
      3. Common Homebrew paths                            (dev Mac)
      4. System PATH
    """
    import shutil

    if getattr(sys, 'frozen', False):
        script_dir = os.path.dirname(sys.executable)
    else:
        script_dir = os.path.dirname(os.path.abspath(__file__))

    candidates = [
        os.path.join(script_dir, 'tesseract'),
        os.path.normpath(os.path.join(script_dir, '..', 'bin', 'tesseract')),
        os.path.join(script_dir, 'bin', 'tesseract'),
        '/opt/homebrew/bin/tesseract',
        '/usr/local/bin/tesseract',
        '/usr/bin/tesseract',
    ]
    for c in candidates:
        if os.path.isfile(c) and os.access(c, os.X_OK):
            return c
    return shutil.which('tesseract') or 'tesseract'


def find_tessdata_dir():
    """
    Locate the tessdata directory containing vie.traineddata.
    Returns the directory path, or None if not found (tesseract will use
    its built-in TESSDATA_PREFIX env var or system default).
    """
    if getattr(sys, 'frozen', False):
        script_dir = os.path.dirname(sys.executable)
    else:
        script_dir = os.path.dirname(os.path.abspath(__file__))

    candidates = [
        os.path.join(script_dir, 'tessdata'),
        os.path.normpath(os.path.join(script_dir, '..', 'bin', 'tessdata')),
        os.path.join(script_dir, 'bin', 'tessdata'),
    ]
    for c in candidates:
        if os.path.isdir(c) and os.path.isfile(os.path.join(c, 'vie.traineddata')):
            return c
    return None


# ---------------------------------------------------------------------------
# Tesseract OCR wrapper
# ---------------------------------------------------------------------------

def ocr_with_tesseract(img, tesseract_cmd, tessdata_dir, psm=3):
    """
    Run Tesseract OCR on a numpy BGR image.

    Returns (results, elapse) where:
      results = [(polygon, text, confidence), ...]
      polygon  = [[x,y],[x,y],[x,y],[x,y]]  (top-left → clockwise)
      text     = full text of the line
      confidence = float 0–1

    psm meanings (most useful):
      3  = Fully automatic page segmentation  (full pages)
      6  = Assume a single uniform block of text (cropped regions)
      11 = Sparse text — find as much text as possible (figure crops)
    """
    import time
    import pytesseract
    from PIL import Image

    pytesseract.pytesseract.tesseract_cmd = tesseract_cmd

    t0 = time.perf_counter()

    # Convert numpy BGR → PIL RGB
    import cv2
    rgb = cv2.cvtColor(img, cv2.COLOR_BGR2RGB)
    pil_img = Image.fromarray(rgb)

    config = f'--oem 1 --psm {psm} -l vie+eng'
    if tessdata_dir:
        config += f' --tessdata-dir "{tessdata_dir}"'

    data = pytesseract.image_to_data(pil_img, config=config,
                                     output_type=pytesseract.Output.DICT)

    # Group words into lines: same (block_num, par_num, line_num) = same line.
    lines = {}
    for i in range(len(data['text'])):
        conf = int(data['conf'][i])
        if conf <= 0:
            continue
        text = data['text'][i].strip()
        if not text:
            continue
        key = (data['block_num'][i], data['par_num'][i], data['line_num'][i])
        lines.setdefault(key, []).append({
            'text':  text,
            'conf':  conf,
            'left':  data['left'][i],
            'top':   data['top'][i],
            'w':     data['width'][i],
            'h':     data['height'][i],
        })

    results = []
    for key in sorted(lines.keys()):
        words = lines[key]
        left   = min(w['left']        for w in words)
        top    = min(w['top']         for w in words)
        right  = max(w['left']+w['w'] for w in words)
        bottom = max(w['top']+w['h']  for w in words)
        text   = ' '.join(w['text']   for w in words)
        conf   = sum(w['conf']        for w in words) / len(words) / 100.0
        poly   = [[left, top], [right, top], [right, bottom], [left, bottom]]
        results.append((poly, text, conf))

    # Sort top-to-bottom by the top-left y coordinate.
    results.sort(key=lambda r: r[0][0][1])

    return results, time.perf_counter() - t0


# ---------------------------------------------------------------------------
# Bbox helpers
# ---------------------------------------------------------------------------

def normalize_bbox(raw):
    """
    Normalize various bbox formats to [x1, y1, x2, y2] (ints).

    Handles:
      - [x1, y1, x2, y2]
      - [[x1,y1], [x2,y1], [x2,y2], [x1,y2]]  (4-corner polygon)
      - numpy arrays of either shape
      - dicts with key "bbox" or "box"
    Returns None if bbox cannot be parsed.
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
    # 4-corner polygon: [[x,y], [x,y], [x,y], [x,y]]
    if isinstance(raw[0], (list, tuple)):
        try:
            xs = [p[0] for p in raw]
            ys = [p[1] for p in raw]
            return [int(min(xs)), int(min(ys)), int(max(xs)), int(max(ys))]
        except Exception:
            return None
    # Flat [x1, y1, x2, y2]
    if len(raw) == 4:
        try:
            vals = [float(v) for v in raw]
            return [int(vals[0]), int(vals[1]), int(vals[2]), int(vals[3])]
        except Exception:
            return None
    return None


def crop_region(img, bbox):
    """Safely crop img (numpy HWC) by bbox. Returns None if out of bounds."""
    if img is None or bbox is None:
        return None
    h, w = img.shape[:2]
    x1, y1, x2, y2 = bbox
    x1, y1 = max(0, x1), max(0, y1)
    x2, y2 = min(w, x2), min(h, y2)
    if x2 <= x1 or y2 <= y1:
        return None
    return img[y1:y2, x1:x2]


# ---------------------------------------------------------------------------
# Figure classifier — whitelist + OCR check
# ---------------------------------------------------------------------------

def _is_seal_shape(img_bgr, bbox):
    """
    Heuristic: high circularity + aspect ratio close to 1:1.
    Typical for round official seals/stamps.
    """
    try:
        import cv2
        import numpy as np
        region = crop_region(img_bgr, bbox)
        if region is None or region.size == 0:
            return False
        rh, rw = region.shape[:2]
        if rw == 0 or rh == 0:
            return False
        if min(rw, rh) / max(rw, rh) < 0.70:
            return False
        gray = cv2.cvtColor(region, cv2.COLOR_BGR2GRAY)
        _, binary = cv2.threshold(gray, 0, 255, cv2.THRESH_BINARY_INV + cv2.THRESH_OTSU)
        contours, _ = cv2.findContours(binary, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
        if not contours:
            return False
        largest = max(contours, key=cv2.contourArea)
        area = cv2.contourArea(largest)
        perimeter = cv2.arcLength(largest, True)
        if perimeter < 10:
            return False
        circularity = 4.0 * math.pi * area / (perimeter * perimeter)
        return circularity > 0.60
    except Exception:
        return False


def _has_red_purple_dominant(img_bgr, bbox):
    """
    Heuristic: red or purple dominant color (typical for Vietnamese/Chinese official seals).
    """
    try:
        import cv2
        import numpy as np
        region = crop_region(img_bgr, bbox)
        if region is None or region.size == 0:
            return False
        hsv = cv2.cvtColor(region, cv2.COLOR_BGR2HSV)
        h_ch, s_ch, v_ch = hsv[:, :, 0], hsv[:, :, 1], hsv[:, :, 2]
        saturated = (s_ch > 80) & (v_ch > 60)
        red_mask    = saturated & ((h_ch <= 10) | (h_ch >= 160))
        purple_mask = saturated & (h_ch >= 130) & (h_ch <= 160)
        total = region.shape[0] * region.shape[1]
        if total == 0:
            return False
        ratio = (np.sum(red_mask) + np.sum(purple_mask)) / total
        return ratio > 0.04  # ≥4% red/purple pixels
    except Exception:
        return False


def classify_figure(img_bgr, bbox, page_h, page_w, tesseract_cmd, tessdata_dir):
    """
    Classify a figure region as 'decorative' or 'informational'.

    Whitelist (decorative — do not translate):
      1. Seal/stamp: circular shape + red/purple dominant color
      2. Signature: bottom area of page + wide horizontal bbox
      3. Logo: top area of page + small relative area

    Anything outside the whitelist that contains OCR-detectable text
    is classified as 'informational' (e.g. chart, diagram).

    Returns (figure_type: str, text_lines: list[str])
    """
    x1, y1, x2, y2 = bbox
    rw = x2 - x1
    rh = y2 - y1
    region_area = rw * rh
    page_area = page_h * page_w if page_h > 0 and page_w > 0 else 1

    # --- Whitelist check 1: Seal/stamp ---
    if _is_seal_shape(img_bgr, bbox) and _has_red_purple_dominant(img_bgr, bbox):
        return "decorative", []

    # --- Whitelist check 2: Signature ---
    if y1 > page_h * 0.65 and rh > 0 and (rw / rh) > 2.5:
        return "decorative", []

    # --- Whitelist check 3: Logo ---
    if y2 < page_h * 0.22 and region_area < page_area * 0.08:
        return "decorative", []

    # --- Non-whitelist: check for text via Tesseract (psm=11: sparse text) ---
    cropped = crop_region(img_bgr, bbox)
    if cropped is None or cropped.size == 0:
        return "decorative", []

    try:
        results, _ = ocr_with_tesseract(cropped, tesseract_cmd, tessdata_dir, psm=11)
        if results:
            text_lines = [r[1].strip() for r in results if r[1].strip()]
            if text_lines:
                return "informational", text_lines
    except Exception:
        pass

    return "decorative", []


# ---------------------------------------------------------------------------
# Paragraph type classifier (title vs text)
# ---------------------------------------------------------------------------

# Vietnamese legal/document section markers that indicate headings.
_SECTION_RE = re.compile(
    r'^(?:'
    r'CHƯƠNG\s+\S+'           # CHƯƠNG I, CHƯƠNG 1
    r'|Chương\s+\S+'
    r'|PHẦN\s+\S+'
    r'|Phần\s+\S+'
    r'|MỤC\s+\S+'
    r'|Mục\s+\S+'
    r'|ĐIỀU\s+\d+'
    r'|Điều\s+\d+'
    r'|[IVX]+\.\s'            # Roman numeral: "I. ", "II. "
    r')',
    re.UNICODE,
)


def _classify_paragraph(text, page_w, x1, x2):
    """
    Return 'title' or 'text' for a paragraph, using content and position heuristics.

    Title signals (Vietnamese documents):
      1. Matches Vietnamese/legal section patterns (Điều, Chương, etc.)
      2. High uppercase ratio (≥75%) with ≤20 words  →  ALL-CAPS heading
      3. Visually centred: both left+right margins >15% of page width, ≤15 words
    """
    s = text.strip()
    if not s:
        return "text"

    words = s.split()
    word_count = len(words)

    if word_count > 30:
        return "text"

    if _SECTION_RE.match(s):
        return "title"

    letters = [c for c in s if c.isalpha()]
    if len(letters) >= 4:
        upper_ratio = sum(1 for c in letters if c.isupper()) / len(letters)
        if upper_ratio >= 0.75 and word_count <= 20:
            return "title"

    if page_w > 0 and word_count <= 15:
        left_margin  = x1 / page_w
        right_margin = (page_w - x2) / page_w
        if left_margin > 0.15 and right_margin > 0.15:
            return "title"

    return "text"


# ---------------------------------------------------------------------------
# Per-page processing
# ---------------------------------------------------------------------------

def _detect_table_regions(image_path, layout_engine):
    """
    Run layout detection and return confident TABLE bounding boxes.
    Returns list of [x1, y1, x2, y2]. Non-fatal: returns [] on any error.
    """
    try:
        layout_out = layout_engine(image_path)
        if hasattr(layout_out, "boxes"):
            boxes  = layout_out.boxes  or []
            names  = layout_out.class_names or []
            scores = layout_out.scores or []
        elif isinstance(layout_out, (list, tuple)) and len(layout_out) == 2:
            raw = layout_out[0] or []
            boxes, names, scores = [], [], []
            for r in raw:
                if isinstance(r, dict):
                    bbox  = normalize_bbox(r.get("bbox") or r.get("box"))
                    label = str(r.get("label") or r.get("type") or "").lower()
                    if bbox and label == "table":
                        boxes.append(bbox)
                        names.append(label)
                        scores.append(1.0)
            return boxes
        else:
            return []

        table_bboxes = []
        for b, n, s in zip(boxes, names, scores):
            # Threshold 0.50: pp_layout_cdla was trained on Chinese academic papers
            # and scores lower on Vietnamese legal docs.
            if str(n).lower() == "table" and float(s) >= 0.50:
                bbox = normalize_bbox(b)
                if bbox:
                    table_bboxes.append(bbox)
        return table_bboxes
    except Exception as e:
        print(f"[WARN] layout table detection failed: {e}", file=sys.stderr)
        return []


def _point_in_bbox(y, x, bbox):
    """Return True if (y, x) center point is inside bbox [x1, y1, x2, y2]."""
    x1, y1, x2, y2 = bbox
    return x1 <= x <= x2 and y1 <= y <= y2


def _extract_table_html(img, bbox, table_engine, tesseract_cmd, tessdata_dir):
    """
    Crop the table region and extract HTML via rapid_table.

    We pre-run Tesseract (psm=6: uniform text block) on the cropped table region
    and pass the results to RapidTable via the `ocr_results` parameter, bypassing
    rapid_table's internal OCR engine dependency entirely.

    Returns HTML string containing a <table> element, else ''.
    """
    import numpy as np

    cropped = crop_region(img, bbox)
    if cropped is None or cropped.size == 0:
        return ""
    try:
        # psm=6: treat the crop as a uniform block of text (better for table cells)
        raw_lines, _ = ocr_with_tesseract(cropped, tesseract_cmd, tessdata_dir, psm=6)

        # Convert to rapid_table ocr_results format:
        #   list[tuple(boxes_array[N,4,2], txts_tuple, scores_tuple)]
        if raw_lines:
            boxes  = np.array([r[0] for r in raw_lines], dtype=np.float32)  # [N,4,2]
            txts   = tuple(r[1] for r in raw_lines)
            scores = tuple(float(r[2]) for r in raw_lines)
        else:
            boxes  = np.zeros((0, 4, 2), dtype=np.float32)
            txts   = ()
            scores = ()

        ocr_results = [[boxes, txts, scores]]
        result = table_engine(cropped, ocr_results=ocr_results)

        html = ""
        if hasattr(result, "pred_htmls") and result.pred_htmls:
            html = result.pred_htmls[0] or ""
        elif isinstance(result, (list, tuple)):
            html = str(result[0]) if result else ""
        elif isinstance(result, str):
            html = result

        if "<table" in html.lower():
            return html
    except Exception as e:
        print(f"[WARN] table extraction failed: {e}", file=sys.stderr)
    return ""


def process_page(img, image_path, page_no, tesseract_cmd, tessdata_dir,
                 layout_engine, table_engine):
    """
    Process one page image.

    Strategy:
    1. Full-page Tesseract OCR (vie+eng, psm=3) → text lines with bboxes.
    2. rapid_layout → detect TABLE regions.
    3. For each TABLE: rapid_table (with Tesseract OCR results) → HTML.
       Fallback to plain OCR text if rapid_table fails.
    4. Cluster non-table OCR lines into paragraphs by vertical spacing.
    5. Interleave paragraphs + tables in top-to-bottom order.
    """
    page_h, page_w = img.shape[:2]
    page_out = {"page_no": page_no, "width": page_w, "height": page_h, "regions": []}

    # ── 1. Full-page OCR ──────────────────────────────────────────────────────
    all_lines = []  # [(y_center, y1, y2, x1, x2, text)]
    try:
        results, _ = ocr_with_tesseract(img, tesseract_cmd, tessdata_dir, psm=3)
        for poly, text, conf in results:
            if not text.strip():
                continue
            ys = [p[1] for p in poly]
            xs = [p[0] for p in poly]
            y1, y2 = min(ys), max(ys)
            x1, x2 = min(xs), max(xs)
            all_lines.append(((y1 + y2) / 2, y1, y2, x1, x2, text))
    except Exception as e:
        print(f"[WARN] page {page_no}: OCR failed: {e}", file=sys.stderr)

    all_lines.sort(key=lambda l: l[0])

    # ── 2. Detect table regions ───────────────────────────────────────────────
    table_bboxes = _detect_table_regions(image_path, layout_engine)

    # ── 3. Extract table HTML ─────────────────────────────────────────────────
    table_entries = []  # [(y1, content, is_html)]
    for tb in table_bboxes:
        tx1, ty1, tx2, ty2 = tb
        inner_lines = [l for l in all_lines if _point_in_bbox(l[0], (l[3]+l[4])/2, tb)]
        inner_lines.sort(key=lambda l: l[0])

        html = _extract_table_html(img, tb, table_engine, tesseract_cmd, tessdata_dir)
        if html:
            table_entries.append((ty1, html, True))
        elif inner_lines:
            fallback_text = " ".join(l[5] for l in inner_lines)
            if fallback_text.strip():
                table_entries.append((ty1, fallback_text, False))

    # ── 4. Cluster non-table lines into paragraphs ───────────────────────────
    text_lines = [l for l in all_lines
                  if not any(_point_in_bbox(l[0], (l[3]+l[4])/2, tb) for tb in table_bboxes)]

    paragraphs = []  # [(y1, x1, x2, text)]
    if text_lines:
        line_heights = [l[2] - l[1] for l in text_lines]
        avg_h = sum(line_heights) / len(line_heights) if line_heights else 20

        # Adaptive gap threshold: 25th-percentile inter-line gap × 1.8
        # This splits paragraphs without being thrown off by dense legal text.
        raw_gaps = sorted(
            max(0, text_lines[i][1] - text_lines[i-1][2])
            for i in range(1, len(text_lines))
        )
        if len(raw_gaps) >= 4:
            p25 = raw_gaps[len(raw_gaps) // 4]
            gap_threshold = max(p25 * 1.8, avg_h * 0.15, 2)
        else:
            gap_threshold = max(avg_h * 0.3, 3)

        current   = []
        para_y1   = text_lines[0][1]
        para_x1   = text_lines[0][3]
        para_x2   = text_lines[0][4]
        prev_y2   = text_lines[0][2]

        for y_ctr, y1, y2, x1, x2, text in text_lines:
            gap = y1 - prev_y2
            if current and gap > gap_threshold:
                paragraphs.append((para_y1, para_x1, para_x2, " ".join(current)))
                current  = []
                para_y1  = y1
                para_x1  = x1
                para_x2  = x2
            else:
                para_x1 = min(para_x1, x1)
                para_x2 = max(para_x2, x2)
            current.append(text)
            prev_y2 = y2

        if current:
            paragraphs.append((para_y1, para_x1, para_x2, " ".join(current)))

    # ── 5. Merge + sort by page order ────────────────────────────────────────
    merged = []
    for y1, px1, px2, text in paragraphs:
        if text.strip():
            rtype = _classify_paragraph(text, page_w, px1, px2)
            merged.append((y1, rtype, text))
    for y1, content, is_html in table_entries:
        merged.append((y1, "table" if is_html else "text", content))

    merged.sort(key=lambda r: r[0])

    for y1, rtype, content in merged:
        if rtype == "table":
            page_out["regions"].append({
                "type": "table", "bbox": [0, int(y1), page_w, page_h], "html": content,
            })
        else:
            page_out["regions"].append({
                "type": rtype, "bbox": [0, int(y1), page_w, page_h], "content": content,
            })

    return page_out


def process_all_pages(image_paths, tesseract_cmd, tessdata_dir, layout_engine, table_engine):
    """
    Process all pages. Each page failure is non-fatal.
    """
    import cv2

    pages = []
    for idx, image_path in enumerate(image_paths):
        page_no = idx + 1
        img = cv2.imread(image_path)
        if img is None:
            print(f"[WARN] page {page_no}: cannot read {image_path}", file=sys.stderr)
            pages.append({"page_no": page_no, "width": 0, "height": 0, "regions": []})
            continue

        try:
            page_out = process_page(img, image_path, page_no,
                                    tesseract_cmd, tessdata_dir,
                                    layout_engine, table_engine)
        except Exception as e:
            print(f"[WARN] page {page_no}: processing failed: {e}", file=sys.stderr)
            page_h, page_w = img.shape[:2]
            page_out = {"page_no": page_no, "width": page_w, "height": page_h, "regions": []}

        pages.append(page_out)

    return {"pages": pages}


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

    # Locate Tesseract before importing heavy deps
    tesseract_cmd = find_tesseract_cmd()
    tessdata_dir  = find_tessdata_dir()
    print(f"[INFO] tesseract: {tesseract_cmd}", file=sys.stderr)
    print(f"[INFO] tessdata:  {tessdata_dir}", file=sys.stderr)

    # Import heavy dependencies here (after freeze_support)
    import pytesseract as _pt
    _pt.pytesseract.tesseract_cmd = tesseract_cmd

    from rapid_layout import RapidLayout
    from rapid_table  import RapidTable

    layout_engine = RapidLayout()
    table_engine  = RapidTable()

    try:
        result = process_all_pages(args.images, tesseract_cmd, tessdata_dir,
                                   layout_engine, table_engine)
        print(json.dumps(result, ensure_ascii=False))
    except Exception as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        sys.exit(1)
