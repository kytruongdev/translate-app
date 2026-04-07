"""
OCR Preview — chạy OCR pipeline trên PDF và render HTML để inspect kết quả.

Usage:
    python3 ocr_preview.py <pdf_file> [output.html]

Hiển thị từng page với các regions được detect:
  - TEXT / TITLE: nội dung text thực tế
  - TABLE: HTML table từ rapid_table
  - TABLE(opencv): table detect bằng OpenCV (rapid_layout miss)

Không dịch, chỉ OCR + layout detection.
"""

import sys, os, json, re, subprocess
import fitz
import cv2
import numpy as np

# ── Locate sidecar / Python runner ───────────────────────────────────────────

def find_runner():
    """Return (executable, prefix_args) for the OCR sidecar."""
    cwd = os.path.dirname(os.path.abspath(__file__))

    # 1. Python + ocr_sidecar.py via .venv (preferred for dev — always up-to-date)
    script = os.path.join(cwd, "ocr_sidecar.py")
    venv_py = os.path.join(cwd, ".venv", "bin", "python3")
    if os.path.isfile(script):
        python = venv_py if os.path.isfile(venv_py) else "python3"
        return python, [script]

    return None, []


# ── Render PDF pages to PNGs ──────────────────────────────────────────────────

def render_pdf(pdf_path, dpi=200):
    import tempfile, pathlib
    doc = fitz.open(pdf_path)
    tmp = tempfile.mkdtemp(prefix="ocr_preview_")
    paths = []
    for i in range(doc.page_count):
        mat = fitz.Matrix(dpi / 72, dpi / 72)
        pix = doc[i].get_pixmap(matrix=mat, colorspace=fitz.csRGB)
        p = os.path.join(tmp, f"page-{i+1:04d}.png")
        pix.save(p)
        paths.append(p)
    doc.close()
    return paths, tmp


# ── Run OCR sidecar (streaming NDJSON) ───────────────────────────────────────

def run_ocr(image_paths):
    exe, prefix = find_runner()
    if exe is None:
        raise RuntimeError("OCR sidecar not found — run `make sidecar-mac` or activate .venv")

    args = [exe] + prefix + image_paths
    proc = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)

    pages = []
    for line in proc.stdout:
        line = line.strip()
        if not line:
            continue
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue
        if obj.get("done"):
            break
        if obj.get("error"):
            raise RuntimeError(f"Sidecar error: {obj['error']}")
        if obj.get("page_no"):
            pages.append(obj)
            print(f"  OCR page {obj['page_no']}/{len(image_paths)}", end="\r", flush=True)

    proc.wait()
    print()
    return pages


# ── OpenCV table detection (supplement to sidecar) ───────────────────────────

def detect_tables_opencv(image_path, min_area=30000):
    """Return list of [x1,y1,x2,y2] bboxes where grid lines are clearly visible."""
    img = cv2.imread(image_path)
    if img is None:
        return []
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
    _, thresh = cv2.threshold(gray, 200, 255, cv2.THRESH_BINARY_INV)

    h_k = cv2.getStructuringElement(cv2.MORPH_RECT, (80, 2))
    v_k = cv2.getStructuringElement(cv2.MORPH_RECT, (2, 80))
    grid = cv2.add(
        cv2.morphologyEx(thresh, cv2.MORPH_OPEN, h_k),
        cv2.morphologyEx(thresh, cv2.MORPH_OPEN, v_k),
    )
    dilated = cv2.dilate(grid, np.ones((5, 5), np.uint8), iterations=3)
    contours, _ = cv2.findContours(dilated, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    bboxes = []
    H, W = img.shape[:2]
    for c in contours:
        if cv2.contourArea(c) >= min_area:
            x, y, w, h = cv2.boundingRect(c)
            bboxes.append([x, y, x + w, y + h])
    return sorted(bboxes, key=lambda b: b[1])


def _iou(a, b):
    """Intersection-over-union of two [x1,y1,x2,y2] boxes."""
    ix1 = max(a[0], b[0]); iy1 = max(a[1], b[1])
    ix2 = min(a[2], b[2]); iy2 = min(a[3], b[3])
    if ix2 <= ix1 or iy2 <= iy1:
        return 0.0
    inter = (ix2 - ix1) * (iy2 - iy1)
    area_a = (a[2]-a[0]) * (a[3]-a[1])
    area_b = (b[2]-b[0]) * (b[3]-b[1])
    return inter / (area_a + area_b - inter)


def extract_table_html_opencv(image_path, bbox):
    """Crop region, run EasyOCR + rapid_table, return HTML string."""
    import easyocr
    from rapid_table import RapidTable

    # Lazy-init globals to avoid reloading models per call
    global _reader, _table_engine
    if '_reader' not in globals():
        print("  Loading EasyOCR + RapidTable models…")
        _reader = easyocr.Reader(['vi', 'en'], gpu=False, verbose=False)
        _table_engine = RapidTable()

    img = cv2.imread(image_path)
    x1, y1, x2, y2 = bbox
    H, W = img.shape[:2]
    x1, y1 = max(0, x1), max(0, y1)
    x2, y2 = min(W, x2), min(H, y2)
    crop = img[y1:y2, x1:x2]
    if crop.size == 0:
        return ""

    results = _reader.readtext(crop, detail=1, paragraph=False)
    clean = [(b, t, c) for b, t, c in results if t.strip() and c >= 0.2]
    if not clean:
        return ""

    boxes = np.array([[[float(p[0]), float(p[1])] for p in b] for b, _, _ in clean], dtype=np.float32)
    ocr_results = [[boxes, tuple(t for _, t, _ in clean), tuple(float(c) for _, _, c in clean)]]

    try:
        result = _table_engine(crop, ocr_results=ocr_results)
        html = ""
        if hasattr(result, "pred_htmls") and result.pred_htmls:
            html = result.pred_htmls[0] or ""
        if html.lower().count("<td") < 4:
            return ""
        return html
    except Exception as e:
        return ""


# ── Merge sidecar regions + OpenCV tables ────────────────────────────────────

def augment_page_with_opencv(page_out, image_path):
    """
    Find table bboxes via OpenCV that the sidecar missed, extract HTML,
    and insert them into page_out['regions'] in y-order.
    Returns count of tables added.
    """
    opencv_bboxes = detect_tables_opencv(image_path)
    if not opencv_bboxes:
        return 0

    # Collect existing table bboxes from sidecar output
    existing = [r["bbox"] for r in page_out["regions"] if r["type"] == "table" and r.get("bbox")]

    added = 0
    for bbox in opencv_bboxes:
        # Skip if overlaps with an existing sidecar table
        if any(_iou(bbox, e) > 0.3 for e in existing):
            continue

        html = extract_table_html_opencv(image_path, bbox)
        if not html:
            continue

        page_out["regions"].append({
            "type": "table",
            "source": "opencv",   # mark origin for preview badge
            "bbox": bbox,
            "html": html,
        })
        existing.append(bbox)
        added += 1

    # Re-sort regions top-to-bottom by y1
    page_out["regions"].sort(key=lambda r: r["bbox"][1] if r.get("bbox") else 0)
    return added


# ── HTML rendering ────────────────────────────────────────────────────────────

_CSS = """
body{font-family:'Segoe UI',sans-serif;max-width:1000px;margin:0 auto;padding:20px;background:#f0f0f0}
h1{font-size:1.2em;margin-bottom:4px}
.meta{font-size:.85em;color:#666;margin-bottom:20px}
.page{background:white;padding:24px;margin-bottom:28px;border-radius:8px;box-shadow:0 2px 6px rgba(0,0,0,.12)}
.page-title{font-weight:bold;font-size:.95em;border-bottom:2px solid #333;padding-bottom:8px;margin-bottom:16px}
.region{padding:10px 12px;margin-bottom:10px;border-radius:4px;border:1px solid rgba(0,0,0,.1)}
.r-text{background:#f9f9f9}
.r-title{background:#fffbe6;border-color:#f5c518}
.r-table{background:#f0f8ff;border-color:#5b9bd5}
.r-table-cv{background:#f0fff4;border-color:#2ea44f}
.badge{display:inline-block;font-size:10px;font-weight:bold;letter-spacing:.5px;
       background:rgba(0,0,0,.12);padding:2px 7px;border-radius:3px;margin-bottom:6px}
.badge-cv{background:#2ea44f;color:white}
.content{font-size:13px;line-height:1.6;white-space:pre-wrap;margin:0}
.table-wrap{overflow-x:auto;margin-top:4px}
.table-wrap table{border-collapse:collapse;width:100%;font-size:12px}
.table-wrap td,.table-wrap th{border:1px solid #aaa;padding:5px 7px;vertical-align:top}
.align-center{text-align:center}
.align-right{text-align:right}
.stats{background:#fff;border-radius:6px;padding:10px 16px;margin-bottom:20px;font-size:.9em;color:#555}
.stats b{color:#222}
"""

def escape(s):
    return (s.replace("&","&amp;").replace("<","&lt;").replace(">","&gt;")
             .replace('"',"&quot;"))

def render_html(pages, pdf_name, opencv_added_per_page):
    total_regions = sum(len(p["regions"]) for p in pages)
    total_tables  = sum(1 for p in pages for r in p["regions"] if r["type"]=="table")
    total_cv      = sum(opencv_added_per_page)
    total_titles  = sum(1 for p in pages for r in p["regions"] if r["type"]=="title")
    total_texts   = sum(1 for p in pages for r in p["regions"] if r["type"]=="text")

    parts = [f"<!DOCTYPE html><html lang='vi'><head><meta charset='UTF-8'>",
             f"<title>OCR Preview — {escape(pdf_name)}</title>",
             f"<style>{_CSS}</style></head><body>",
             f"<h1>OCR Preview — {escape(pdf_name)}</h1>",
             f"<div class='stats'>"
             f"<b>{len(pages)}</b> pages &nbsp;·&nbsp; "
             f"<b>{total_regions}</b> regions &nbsp;·&nbsp; "
             f"<b>{total_titles}</b> title &nbsp;·&nbsp; "
             f"<b>{total_texts}</b> text &nbsp;·&nbsp; "
             f"<b>{total_tables}</b> table "
             f"(<b style='color:#2ea44f'>{total_cv} added by OpenCV</b>)"
             f"</div>"]

    for i, page in enumerate(pages):
        cv_count = opencv_added_per_page[i]
        parts.append(f"<div class='page'>")
        parts.append(f"<div class='page-title'>Page {page['page_no']} "
                     f"<span style='font-weight:normal;color:#888;font-size:.85em'>"
                     f"— {len(page['regions'])} regions"
                     + (f", <span style='color:#2ea44f'>{cv_count} from OpenCV</span>" if cv_count else "")
                     + "</span></div>")

        for ri, region in enumerate(page["regions"]):
            rtype  = region.get("type", "text")
            source = region.get("source", "sidecar")
            is_cv  = (source == "opencv")
            align  = region.get("alignment", "left")

            if rtype == "table":
                cls   = "r-table-cv" if is_cv else "r-table"
                badge = f"<span class='badge badge-cv'>[{ri}] TABLE (OpenCV)</span>" if is_cv \
                        else f"<span class='badge'>[{ri}] TABLE</span>"
                parts.append(f"<div class='region {cls}'>{badge}")
                parts.append(f"<div class='table-wrap'>{region.get('html','')}</div>")
                parts.append("</div>")

            elif rtype == "title":
                a_cls = f"align-{align}" if align in ("center","right") else ""
                parts.append(f"<div class='region r-title'>"
                              f"<span class='badge'>[{ri}] TITLE"
                              + (f" · {align}" if align != "left" else "") + "</span>"
                              f"<p class='content {a_cls}'>{escape(region.get('content',''))}</p>"
                              f"</div>")

            else:  # text
                a_cls = f"align-{align}" if align in ("center","right") else ""
                parts.append(f"<div class='region r-text'>"
                              f"<span class='badge'>[{ri}] TEXT"
                              + (f" · {align}" if align != "left" else "") + "</span>"
                              f"<p class='content {a_cls}'>{escape(region.get('content',''))}</p>"
                              f"</div>")

        parts.append("</div>")  # .page

    parts.append("</body></html>")
    return "\n".join(parts)


# ── Main ──────────────────────────────────────────────────────────────────────

def main():
    import shutil, tempfile

    if len(sys.argv) < 2:
        print("Usage: python3 ocr_preview.py <pdf_file> [output.html]")
        sys.exit(1)

    pdf_path = sys.argv[1]
    pdf_name = os.path.basename(pdf_path)
    out_path = sys.argv[2] if len(sys.argv) >= 3 else \
               os.path.splitext(pdf_name)[0] + "_ocr_preview.html"

    print(f"PDF: {pdf_name}")

    # 1. Render pages
    print("Rendering pages…")
    image_paths, tmp_dir = render_pdf(pdf_path, dpi=200)
    print(f"  {len(image_paths)} pages rendered to {tmp_dir}")

    # 2. Run OCR sidecar
    print("Running OCR sidecar…")
    try:
        pages = run_ocr(image_paths)
    except Exception as e:
        shutil.rmtree(tmp_dir, ignore_errors=True)
        print(f"ERROR: {e}")
        sys.exit(1)
    print(f"  OCR done: {sum(len(p['regions']) for p in pages)} regions total")

    # 3. OpenCV table augmentation
    print("Running OpenCV table detection…")
    opencv_added = []
    for page in pages:
        idx = page["page_no"] - 1
        img_path = image_paths[idx] if idx < len(image_paths) else None
        if img_path:
            n = augment_page_with_opencv(page, img_path)
            opencv_added.append(n)
            if n:
                print(f"  Page {page['page_no']}: +{n} table(s) from OpenCV")
        else:
            opencv_added.append(0)

    total_cv = sum(opencv_added)
    print(f"  OpenCV added {total_cv} table(s) total")

    # 4. Render HTML
    print("Rendering HTML…")
    html = render_html(pages, pdf_name, opencv_added)
    with open(out_path, "w", encoding="utf-8") as f:
        f.write(html)

    # 5. Cleanup
    shutil.rmtree(tmp_dir, ignore_errors=True)

    print(f"\nDone → {out_path}")
    print(f"  Open in browser to inspect OCR regions.")


if __name__ == "__main__":
    main()
