"""
OCR Sidecar — Structured PDF layout analysis.

Usage:
    paddleocr-darwin-arm64 page-0001.png page-0002.png ... page-000N.png

Prints a single JSON object to stdout:
    {"pages": [...]}

Exits with code 1 and writes {"error": "..."} to stderr on fatal error.
Individual page/region failures are non-fatal — logged to stderr, processing continues.
"""

import sys
import json
import os
import math
import argparse
import multiprocessing

# Required for PyInstaller multiprocessing support on macOS/Windows.
if __name__ == "__main__":
    multiprocessing.freeze_support()


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
        # Aspect ratio gate: width and height within 30% of each other
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
        red_mask   = saturated & ((h_ch <= 10) | (h_ch >= 160))
        purple_mask = saturated & (h_ch >= 130) & (h_ch <= 160)
        total = region.shape[0] * region.shape[1]
        if total == 0:
            return False
        ratio = (np.sum(red_mask) + np.sum(purple_mask)) / total
        return ratio > 0.04  # ≥4% red/purple pixels
    except Exception:
        return False


def classify_figure(img_bgr, bbox, page_h, page_w, ocr_engine):
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
    # Typically in the bottom 35% of the page, horizontally wide
    if y1 > page_h * 0.65 and rh > 0 and (rw / rh) > 2.5:
        return "decorative", []

    # --- Whitelist check 3: Logo ---
    # Top 22% of page, small area (< 8% of page)
    if y2 < page_h * 0.22 and region_area < page_area * 0.08:
        return "decorative", []

    # --- Non-whitelist: check for text via OCR ---
    cropped = crop_region(img_bgr, bbox)
    if cropped is None or cropped.size == 0:
        return "decorative", []

    try:
        results, _ = ocr_engine(cropped)
        if results:
            text_lines = [res[1].strip() for res in results if res[1].strip()]
            if text_lines:
                return "informational", text_lines
    except Exception:
        pass

    return "decorative", []


# ---------------------------------------------------------------------------
# Per-page processing
# ---------------------------------------------------------------------------

def process_all_pages(image_paths, ocr_engine, layout_engine, table_engine):
    """
    Process all pages and return the full result dict.
    Each page failure is non-fatal: appends an empty-regions page to output.
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

        page_h, page_w = img.shape[:2]
        page_out = {"page_no": page_no, "width": page_w, "height": page_h, "regions": []}

        try:
            layout_res, _ = layout_engine(image_path)
        except Exception as e:
            print(f"[WARN] page {page_no}: layout detection failed: {e}", file=sys.stderr)
            pages.append(page_out)
            continue

        if not layout_res:
            pages.append(page_out)
            continue

        for region in layout_res:
            try:
                _process_region(region, img, page_h, page_w, page_out, ocr_engine, table_engine)
            except Exception as e:
                print(f"[WARN] page {page_no}: region processing failed: {e}", file=sys.stderr)
                continue

        pages.append(page_out)

    return {"pages": pages}


def _process_region(region, img, page_h, page_w, page_out, ocr_engine, table_engine):
    """Process one layout region and append to page_out["regions"] if valid."""
    # Parse bbox and label from various rapid_layout output formats
    if isinstance(region, dict):
        raw_bbox = region.get("bbox") or region.get("box")
        label = str(region.get("label") or region.get("type") or "text").lower()
    elif isinstance(region, (list, tuple)) and len(region) >= 2:
        raw_bbox = region[0]
        label = str(region[1]).lower() if not isinstance(region[1], (list, tuple)) else "text"
    else:
        return

    bbox = normalize_bbox(raw_bbox)
    if bbox is None:
        return

    x1, y1, x2, y2 = bbox

    if label in ("text", "title"):
        cropped = crop_region(img, bbox)
        content = ""
        if cropped is not None and cropped.size > 0:
            try:
                results, _ = ocr_engine(cropped)
                if results:
                    content = " ".join(r[1].strip() for r in results if r[1].strip())
            except Exception:
                pass
        if content:
            page_out["regions"].append({"type": label, "bbox": bbox, "content": content})

    elif label == "table":
        cropped = crop_region(img, bbox)
        html = ""
        if cropped is not None and cropped.size > 0:
            try:
                table_result = table_engine(cropped)
                # rapid_table may return (html, cell_bboxes, elapse) or just html
                if isinstance(table_result, (list, tuple)):
                    html = str(table_result[0]) if table_result else ""
                elif isinstance(table_result, str):
                    html = table_result
            except Exception:
                pass
        if html:
            page_out["regions"].append({"type": "table", "bbox": bbox, "html": html})

    elif label == "figure":
        fig_type, text_lines = classify_figure(img, bbox, page_h, page_w, ocr_engine)
        entry = {"type": "figure", "figure_type": fig_type, "bbox": bbox}
        if fig_type == "informational" and text_lines:
            entry["text_lines"] = text_lines
        page_out["regions"].append(entry)

    elif label == "figure_caption":
        # Treat caption as a translatable text region
        cropped = crop_region(img, bbox)
        content = ""
        if cropped is not None and cropped.size > 0:
            try:
                results, _ = ocr_engine(cropped)
                if results:
                    content = " ".join(r[1].strip() for r in results if r[1].strip())
            except Exception:
                pass
        if content:
            page_out["regions"].append({"type": "text", "bbox": bbox, "content": content})

    # header, footer, reference, equation — skip for now (not relevant to target documents)


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

    # Import heavy dependencies here so multiprocessing.freeze_support() runs first
    from rapidocr_onnxruntime import RapidOCR
    from rapid_layout import RapidLayout
    from rapid_table import RapidTable

    ocr_engine    = RapidOCR()
    layout_engine = RapidLayout()
    table_engine  = RapidTable()

    try:
        result = process_all_pages(args.images, ocr_engine, layout_engine, table_engine)
        print(json.dumps(result, ensure_ascii=False))
    except Exception as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        sys.exit(1)
