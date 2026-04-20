import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'

/**
 * Modal primitive — headless, accessible, portal-rendered.
 *
 * Single source of truth for:
 *   - role="dialog" + aria-modal + aria-labelledby wiring
 *   - focus trap + focus return
 *   - ESC to close (opt-out)
 *   - Backdrop click to close (opt-out)
 *   - Body scroll lock
 *   - Size variants (sm | md | lg | xl | full)
 *
 * Consumers pick their own chrome (background, padding, divider, etc.) via
 * Modal.Header, Modal.Body, Modal.Footer — all of which accept `className`.
 */

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl' | 'full'

interface ModalContextValue {
  titleId: string
  onOpenChange: (open: boolean) => void
}

const ModalContext = createContext<ModalContextValue | null>(null)

function useModalContext(): ModalContextValue {
  const ctx = useContext(ModalContext)
  if (!ctx) {
    throw new Error('Modal sub-components must be rendered inside <Modal>')
  }
  return ctx
}

interface ModalProps {
  /** Whether the modal is visible. When false, nothing is rendered. */
  open: boolean
  /** Fired when the user requests close (ESC, backdrop, close button). */
  onOpenChange: (open: boolean) => void
  /** Content — typically `Modal.Header`, `Modal.Body`, `Modal.Footer`. */
  children: ReactNode
  /** Width variant. Defaults to `md` (28rem) which matches existing modals. */
  size?: ModalSize
  /** Disable ESC-to-close. */
  closeOnEscape?: boolean
  /** Disable backdrop-click-to-close. */
  closeOnBackdropClick?: boolean
  /** Extra className for the dialog panel (rarely needed — prefer Body props). */
  panelClassName?: string
  /** Extra className for the backdrop. */
  backdropClassName?: string
  /** Initial focus target — defaults to first focusable descendant. */
  initialFocusRef?: React.RefObject<HTMLElement | null>
}

const SIZE_CLASS: Record<ModalSize, string> = {
  sm: 'w-96',
  md: 'w-[28rem]',
  lg: 'w-[32rem]',
  xl: 'w-[48rem]',
  full: 'w-[90vw] max-w-[80rem]',
}

function getFocusable(root: HTMLElement): HTMLElement[] {
  const selector = [
    'a[href]',
    'area[href]',
    'input:not([disabled]):not([type="hidden"])',
    'select:not([disabled])',
    'textarea:not([disabled])',
    'button:not([disabled])',
    'iframe',
    'object',
    'embed',
    '[contenteditable]',
    '[tabindex]:not([tabindex="-1"])',
  ].join(',')
  const nodes = root.querySelectorAll<HTMLElement>(selector)
  return Array.from(nodes).filter((el) => !el.hasAttribute('disabled') && el.tabIndex !== -1)
}

function ModalRoot({
  open,
  onOpenChange,
  children,
  size = 'md',
  closeOnEscape = true,
  closeOnBackdropClick = true,
  panelClassName,
  backdropClassName,
  initialFocusRef,
}: ModalProps) {
  const titleId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const previouslyFocused = useRef<HTMLElement | null>(null)

  // Snapshot previously-focused element when opening so we can restore later.
  useLayoutEffect(() => {
    if (open) {
      previouslyFocused.current = document.activeElement as HTMLElement | null
    }
  }, [open])

  // Body scroll lock while open.
  useEffect(() => {
    if (!open) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previous
    }
  }, [open])

  // Move focus inside on open; restore focus on close.
  useEffect(() => {
    if (!open) return
    const frame = requestAnimationFrame(() => {
      const panel = panelRef.current
      if (!panel) return
      if (initialFocusRef?.current) {
        initialFocusRef.current.focus()
        return
      }
      const first = getFocusable(panel)[0]
      if (first) {
        first.focus()
      } else {
        panel.focus()
      }
    })
    return () => cancelAnimationFrame(frame)
  }, [open, initialFocusRef])

  useEffect(() => {
    if (open) return
    // On close, return focus to the opener — unless it's gone from the DOM.
    const prev = previouslyFocused.current
    if (prev && document.contains(prev)) {
      prev.focus()
    }
  }, [open])

  // Global ESC handler while open.
  useEffect(() => {
    if (!open || !closeOnEscape) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onOpenChange(false)
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [open, closeOnEscape, onOpenChange])

  const onKeyDownPanel = useCallback(
    (e: ReactKeyboardEvent<HTMLDivElement>) => {
      if (e.key !== 'Tab') return
      const panel = panelRef.current
      if (!panel) return
      const focusables = getFocusable(panel)
      if (focusables.length === 0) {
        e.preventDefault()
        return
      }
      const first = focusables[0]
      const last = focusables[focusables.length - 1]
      const active = document.activeElement as HTMLElement | null
      if (e.shiftKey && active === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && active === last) {
        e.preventDefault()
        first.focus()
      }
    },
    []
  )

  const onBackdropMouseDown = useCallback(
    (e: ReactMouseEvent<HTMLDivElement>) => {
      if (!closeOnBackdropClick) return
      if (e.target !== e.currentTarget) return
      onOpenChange(false)
    },
    [closeOnBackdropClick, onOpenChange]
  )

  const ctx = useMemo<ModalContextValue>(
    () => ({ titleId, onOpenChange }),
    [titleId, onOpenChange]
  )

  if (!open) return null
  if (typeof document === 'undefined') return null

  const backdropStyle: CSSProperties = { zIndex: 50 }

  return createPortal(
    <div
      data-testid="modal-backdrop"
      onMouseDown={onBackdropMouseDown}
      className={`fixed inset-0 bg-black/50 flex items-center justify-center ${backdropClassName ?? ''}`}
      style={backdropStyle}
    >
      <ModalContext.Provider value={ctx}>
        <div
          ref={panelRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby={titleId}
          data-size={size}
          tabIndex={-1}
          onKeyDown={onKeyDownPanel}
          className={`${SIZE_CLASS[size]} max-h-[90vh] overflow-y-auto rounded-xl bg-[#1a1f2e] text-white shadow-2xl ${panelClassName ?? ''}`}
        >
          {children}
        </div>
      </ModalContext.Provider>
    </div>,
    document.body
  )
}

/* ---------- Compound slots ---------- */

interface ModalHeaderProps {
  title: ReactNode
  onClose?: () => void
  subtitle?: ReactNode
  className?: string
  /** Hide the close X. Default false. */
  hideClose?: boolean
}

function ModalHeader({ title, onClose, subtitle, className, hideClose }: ModalHeaderProps) {
  const { titleId, onOpenChange } = useModalContext()
  const handleClose = useCallback(() => {
    if (onClose) onClose()
    else onOpenChange(false)
  }, [onClose, onOpenChange])

  return (
    <div
      className={`flex items-start justify-between gap-4 px-6 pt-6 pb-4 ${className ?? ''}`}
    >
      <div className="flex-1 min-w-0">
        <h2 id={titleId} className="text-lg font-bold text-white leading-tight">
          {title}
        </h2>
        {subtitle ? (
          <p className="mt-1 text-sm text-gray-400">{subtitle}</p>
        ) : null}
      </div>
      {!hideClose ? (
        <button
          type="button"
          onClick={handleClose}
          aria-label="Close"
          className="shrink-0 rounded p-1 text-gray-400 hover:text-white hover:bg-white/5 focus:outline-none focus:ring-2 focus:ring-blue-500/50 transition-colors"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      ) : null}
    </div>
  )
}

interface ModalBodyProps {
  children: ReactNode
  className?: string
}

function ModalBody({ children, className }: ModalBodyProps) {
  return (
    <div className={`px-6 pb-4 space-y-4 ${className ?? ''}`}>{children}</div>
  )
}

interface ModalFooterProps {
  children: ReactNode
  className?: string
}

function ModalFooter({ children, className }: ModalFooterProps) {
  return (
    <div
      className={`flex gap-2 justify-end px-6 pt-4 pb-6 border-t border-white/5 ${className ?? ''}`}
    >
      {children}
    </div>
  )
}

interface ModalTitleProps {
  children: ReactNode
  className?: string
}

/** Standalone title for cases that don't use Modal.Header. */
function ModalTitle({ children, className }: ModalTitleProps) {
  const { titleId } = useModalContext()
  return (
    <h2 id={titleId} className={`text-lg font-bold text-white ${className ?? ''}`}>
      {children}
    </h2>
  )
}

interface ModalCompound {
  (props: ModalProps): ReactNode
  Header: typeof ModalHeader
  Body: typeof ModalBody
  Footer: typeof ModalFooter
  Title: typeof ModalTitle
}

export const Modal = ModalRoot as unknown as ModalCompound
;(Modal as ModalCompound).Header = ModalHeader
;(Modal as ModalCompound).Body = ModalBody
;(Modal as ModalCompound).Footer = ModalFooter
;(Modal as ModalCompound).Title = ModalTitle

export default Modal
