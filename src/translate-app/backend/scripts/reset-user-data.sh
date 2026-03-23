#!/usr/bin/env bash
# Xóa toàn bộ dữ liệu cục bộ (phiên, tin nhắn, settings, file dịch đã lưu).
# ĐÓNG Translate App trước khi chạy — tránh SQLite lock.

set -euo pipefail

case "$(uname -s)" in
  Darwin)
    base="${HOME}/Library/Application Support/TranslateApp"
    ;;
  Linux)
    base="${HOME}/.config/TranslateApp"
    ;;
  MINGW*|MSYS*|CYGWIN*)
    base="${APPDATA:-$HOME/AppData/Roaming}/TranslateApp"
    ;;
  *)
    echo "OS không hỗ trợ tự đoán đường dẫn. Đặt TRANSLATE_APP_DATA_DIR rồi chạy lại." >&2
    exit 1
    ;;
esac

if [[ -n "${TRANSLATE_APP_DATA_DIR:-}" ]]; then
  base="${TRANSLATE_APP_DATA_DIR}"
fi

echo "Thư mục dữ liệu: $base"
rm -f "${base}/data.db" "${base}/data.db-shm" "${base}/data.db-wal" 2>/dev/null || true
rm -rf "${base}/files"
echo "Đã xóa data.db (và wal/shm nếu có) + thư mục files/. Lần mở app sau sẽ tạo DB mới."
