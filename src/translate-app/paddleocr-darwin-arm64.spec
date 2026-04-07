# -*- mode: python ; coding: utf-8 -*-
#
# PyInstaller spec — OCR sidecar (macOS arm64)
# Bundles: EasyOCR (vi+en) + rapid_layout + rapid_table + OpenCV
#
# EasyOCR model files (craft_mlt_25k.pth, latin_g2.pth) are NOT embedded here —
# they are distributed as a separate easyocr_models/ directory placed next to the
# binary at runtime.  See Makefile target sidecar-mac for how models are copied.
#
from PyInstaller.utils.hooks import collect_all

datas = []
binaries = []
hiddenimports = [
    'tqdm', '_md5', '_sha1', '_sha256',
    'numpy', 'PIL', 'PIL.Image',
    'scipy', 'scipy.ndimage', 'scipy.signal',
    'skimage', 'skimage.filters', 'skimage.morphology',
    'imageio',
    # stdlib modules that PyInstaller misses when bundling torchvision/easyocr
    'html', 'html.parser', 'html.entities',
    'http', 'http.client', 'http.cookiejar',
    'urllib', 'urllib.request', 'urllib.parse', 'urllib.error',
    'email', 'email.mime', 'email.mime.text',
    'xml', 'xml.etree', 'xml.etree.ElementTree',
]

for pkg in ('easyocr', 'torch', 'torchvision', 'cv2', 'rapid_layout', 'rapid_table'):
    tmp = collect_all(pkg)
    datas    += tmp[0]
    binaries += tmp[1]
    hiddenimports += tmp[2]

a = Analysis(
    ['ocr_sidecar.py'],
    pathex=[],
    binaries=binaries,
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    # Exclude modules we definitely don't need to keep binary smaller.
    excludes=[
        'torch.utils.tensorboard',
        'torchvision.datasets',
        'matplotlib',
        'pandas',
        'IPython',
        'jupyter',
        'notebook',
        'tkinter',
    ],
    noarchive=False,
    optimize=0,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.datas,
    [],
    name='paddleocr-darwin-arm64',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=False,
    upx_exclude=[],
    runtime_tmpdir=None,
    console=True,
    disable_windowed_traceback=False,
    argv_emulation=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
)
