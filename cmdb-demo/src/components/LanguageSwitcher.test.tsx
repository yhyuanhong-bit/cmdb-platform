import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from '../i18n'
import LanguageSwitcher from './LanguageSwitcher'

describe('LanguageSwitcher', () => {
  beforeEach(async () => {
    // Reset language to known baseline (fallback)
    await i18n.changeLanguage('zh-TW')
  })

  it('shows the current language label (繁中 by default)', () => {
    render(<LanguageSwitcher />)
    expect(screen.getByRole('button', { name: /繁中/ })).toBeInTheDocument()
  })

  it('opens a dropdown with all supported languages when clicked', async () => {
    const user = userEvent.setup()
    render(<LanguageSwitcher />)

    await user.click(screen.getByRole('button', { name: /繁中/ }))

    // All three language buttons visible inside the open menu
    expect(screen.getByRole('button', { name: 'EN' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '简中' })).toBeInTheDocument()
  })

  it('switches the language when a menu item is clicked', async () => {
    const user = userEvent.setup()
    render(<LanguageSwitcher />)

    await user.click(screen.getByRole('button', { name: /繁中/ }))
    await user.click(screen.getByRole('button', { name: 'EN' }))

    expect(i18n.language).toBe('en')
    // After switching, the trigger reflects the new label — the trigger's
    // accessible name also includes the leading "translate" icon glyph.
    expect(screen.getByRole('button', { name: /EN/ })).toBeInTheDocument()
  })

  it('closes the dropdown after selecting a language', async () => {
    const user = userEvent.setup()
    render(<LanguageSwitcher />)

    await user.click(screen.getByRole('button', { name: /繁中/ }))
    await user.click(screen.getByRole('button', { name: '简中' }))

    // "EN" was only visible when the menu was open — it should be gone now
    expect(screen.queryByRole('button', { name: 'EN' })).not.toBeInTheDocument()
  })
})
