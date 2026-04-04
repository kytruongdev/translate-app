# -*- mode: python ; coding: utf-8 -*-
# Build on Windows AMD64 (or Windows ARM64 VM with AMD64 MSYS2):
#   pyinstaller --clean paddleocr-windows-amd64.spec
from PyInstaller.utils.hooks import collect_all

datas = []
binaries = []
hiddenimports = ['tqdm', '_md5', '_sha1', '_sha256', 'numpy']

for pkg in ('cv2', 'rapid_layout', 'rapid_table', 'rapidocr_onnxruntime'):
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
    excludes=[],
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
    name='paddleocr-windows-amd64',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    upx_exclude=[],
    runtime_tmpdir=None,
    console=True,
    disable_windowed_traceback=False,
    argv_emulation=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
)
