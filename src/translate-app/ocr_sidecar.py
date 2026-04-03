import sys
import json
import os
import argparse
import multiprocessing

# MANDATORY for PyInstaller on Windows/Mac when using multiprocessing
if __name__ == '__main__':
    multiprocessing.freeze_support()

def process_page(image_path):
    # Move imports inside to avoid early crashes during multiprocessing spawn
    from rapidocr_onnxruntime import RapidOCR
    from rapid_table import RapidTable
    
    engine = RapidOCR()
    # table_engine = RapidTable() # Let's keep it simple first

    results, _ = engine(image_path)
    
    page_output = {
        "page_no": 0,
        "width": 0,
        "height": 0,
        "regions": []
    }

    if results:
        for res in results:
            box = res[0]
            text = res[1]
            bbox = [int(box[0][0]), int(box[0][1]), int(box[2][0]), int(box[2][1])]
            page_output["regions"].append({
                "type": "text",
                "bbox": bbox,
                "content": text
            })

    return page_output

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("image", help="Path to the page image")
    args = parser.parse_args()

    if not os.path.exists(args.image):
        print(json.dumps({"error": "Image not found"}), file=sys.stderr)
        sys.exit(1)

    try:
        result = process_page(args.image)
        print(json.dumps(result, ensure_ascii=False))
    except Exception as e:
        print(f"Error: {str(e)}", file=sys.stderr)
        sys.exit(1)
