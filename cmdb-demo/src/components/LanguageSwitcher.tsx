import { useTranslation } from 'react-i18next'
import { useState, useRef, useEffect } from 'react'

const languages = [
  { code: 'en', label: 'EN' },
  { code: 'zh-CN', label: '简中' },
  { code: 'zh-TW', label: '繁中' },
]

export default function LanguageSwitcher() {
  const { i18n } = useTranslation()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const current = languages.find((l) => l.code === i18n.language) || languages[2]

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1 px-2 py-1 rounded bg-surface-container hover:bg-surface-container-high text-xs text-on-surface-variant transition-colors"
      >
        <span className="material-symbols-outlined text-[16px]">translate</span>
        {current.label}
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-1 bg-surface-container-high rounded shadow-lg z-50 overflow-hidden min-w-[100px]">
          {languages.map((lang) => (
            <button
              key={lang.code}
              onClick={() => {
                i18n.changeLanguage(lang.code)
                setOpen(false)
              }}
              className={`block w-full text-left px-3 py-2 text-xs transition-colors ${
                i18n.language === lang.code
                  ? 'bg-on-primary-container/20 text-primary'
                  : 'text-on-surface-variant hover:bg-surface-container-highest'
              }`}
            >
              {lang.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
