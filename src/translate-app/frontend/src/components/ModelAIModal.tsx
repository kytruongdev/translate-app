import { useEffect, useState } from 'react'
import { useSettingsStore } from '@/stores/settings/settingsStore'
import { STYLE_OPTIONS } from '@/constants/inputOptions'
import type { ActiveProvider } from '@/types/settings'
import type { TranslationStyle } from '@/types/session'

export function ModelAIModal({ onClose }: { onClose: () => void }) {
  const activeProvider = useSettingsStore((s) => s.activeProvider)
  const defaultStyle = useSettingsStore((s) => s.defaultStyle)
  const saveSettings = useSettingsStore((s) => s.saveSettings)

  const [provider, setProvider] = useState<ActiveProvider>(activeProvider)
  const [style, setStyle] = useState<TranslationStyle>(defaultStyle)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    const onEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onEsc)
    return () => window.removeEventListener('keydown', onEsc)
  }, [onClose])

  async function onSave() {
    setSaving(true)
    try {
      const activeModel =
        provider === 'ollama' ? 'qwen2.5:7b' : 'gpt-4o-mini'
      await saveSettings({
        activeProvider: provider,
        activeModel,
        defaultStyle: style,
      })
      onClose()
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="model-ai-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <div className="modal-title">Model AI</div>
        </div>
        <div className="modal-body">
          <label className="settings-row compact">
            <div>
              <div className="settings-row-label">Chọn Model</div>
            </div>
            <select
              className="retranslate-select"
              value={provider}
              onChange={(e) => setProvider(e.target.value as ActiveProvider)}
            >
              <option value="openai">Online (ChatGPT 4o-mini)</option>
              <option value="ollama">Offline (Qwen2.5:7b)</option>
            </select>
          </label>
          <div className="settings-row compact column">
            <div className="settings-row-label">Kiểu dịch mặc định</div>
            <div className="style-radio-list" role="group" aria-label="Kiểu dịch mặc định">
              {STYLE_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  type="button"
                  className={`style-radio-item${style === opt.value ? ' active' : ''}`}
                  onClick={() => setStyle(opt.value as TranslationStyle)}
                >
                  <span className="style-radio-dot" aria-hidden="true" />
                  <span className="style-radio-text">
                    <span className="style-radio-label">{opt.label}</span>
                    <span className="style-radio-desc">{opt.description}</span>
                  </span>
                </button>
              ))}
            </div>
          </div>
        </div>
        <div className="dialog-actions">
          <button type="button" className="popover-btn cancel" onClick={onClose} disabled={saving}>
            Hủy
          </button>
          <button type="button" className="popover-btn confirm" onClick={() => void onSave()} disabled={saving}>
            {saving ? 'Đang lưu…' : 'Lưu'}
          </button>
        </div>
      </div>
    </div>
  )
}
