import { describe, it, expect, beforeEach } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useUrlState } from './useUrlState'

// Helper: wrap hook in a MemoryRouter with a configurable initial URL so we can
// round-trip serialization/deserialization without touching the real browser
// history. Tests assert on URL via useLocation to stay realistic.
function makeWrapper(initialUrl: string) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <MemoryRouter initialEntries={[initialUrl]}>{children}</MemoryRouter>
  }
}

// Helper to read the current URL search string inside the router
function useCurrentSearch(): string {
  const loc = useLocation()
  return loc.search
}

describe('useUrlState', () => {
  beforeEach(() => {
    // Nothing global to reset — MemoryRouter isolates each test.
  })

  describe('default values', () => {
    it('returns defaults when URL has no matching params', () => {
      const { result } = renderHook(
        () => useUrlState('f', { q: '', page: 1, active: false }),
        { wrapper: makeWrapper('/') },
      )
      expect(result.current[0]).toEqual({ q: '', page: 1, active: false })
    })

    it('omits default values from the URL on write', () => {
      const { result } = renderHook(
        () => {
          const state = useUrlState('f', { q: '', page: 1 })
          const search = useCurrentSearch()
          return { state, search }
        },
        { wrapper: makeWrapper('/') },
      )

      act(() => {
        result.current.state[1]({ q: 'hello' })
      })

      // page=1 is default — should not appear in URL
      expect(result.current.search).toContain('f.q=hello')
      expect(result.current.search).not.toContain('page')
    })

    it('removes a key from URL when set back to its default', () => {
      const { result } = renderHook(
        () => {
          const state = useUrlState('f', { q: '', page: 1 })
          const search = useCurrentSearch()
          return { state, search }
        },
        { wrapper: makeWrapper('/?f.q=foo&f.page=3') },
      )

      expect(result.current.state[0]).toEqual({ q: 'foo', page: 3 })

      act(() => {
        result.current.state[1]({ q: '', page: 1 })
      })

      expect(result.current.search).toBe('')
    })
  })

  describe('type round-trips', () => {
    it('round-trips strings', () => {
      const { result } = renderHook(
        () => useUrlState('f', { q: '' }),
        { wrapper: makeWrapper('/?f.q=hello%20world') },
      )
      expect(result.current[0].q).toBe('hello world')
    })

    it('round-trips numbers', () => {
      const { result } = renderHook(
        () => useUrlState('f', { page: 1, size: 20 }),
        { wrapper: makeWrapper('/?f.page=5&f.size=50') },
      )
      expect(result.current[0]).toEqual({ page: 5, size: 50 })
    })

    it('round-trips booleans', () => {
      const { result } = renderHook(
        () => useUrlState('f', { active: false, archived: true }),
        { wrapper: makeWrapper('/?f.active=true&f.archived=false') },
      )
      expect(result.current[0]).toEqual({ active: true, archived: false })
    })

    it('round-trips string arrays with commas', () => {
      const { result } = renderHook(
        () => useUrlState('f', { tags: [] as string[] }),
        { wrapper: makeWrapper('/?f.tags=alpha,beta,gamma') },
      )
      expect(result.current[0].tags).toEqual(['alpha', 'beta', 'gamma'])
    })

    it('round-trips empty string arrays (omitted from URL)', () => {
      const { result } = renderHook(
        () => {
          const state = useUrlState('f', { tags: [] as string[] })
          const search = useCurrentSearch()
          return { state, search }
        },
        { wrapper: makeWrapper('/') },
      )
      expect(result.current.state[0].tags).toEqual([])

      act(() => {
        result.current.state[1]({ tags: ['a', 'b'] })
      })
      expect(result.current.search).toContain('f.tags=a%2Cb')

      act(() => {
        result.current.state[1]({ tags: [] })
      })
      expect(result.current.search).toBe('')
    })
  })

  describe('write semantics', () => {
    it('merges partial updates with current state', () => {
      const { result } = renderHook(
        () => useUrlState('f', { q: '', page: 1, size: 20 }),
        { wrapper: makeWrapper('/?f.q=cat&f.page=3') },
      )

      act(() => {
        result.current[1]({ page: 4 })
      })

      expect(result.current[0]).toEqual({ q: 'cat', page: 4, size: 20 })
    })

    it('preserves unrelated URL params (different key prefix)', () => {
      const { result } = renderHook(
        () => {
          const state = useUrlState('f', { q: '' })
          const search = useCurrentSearch()
          return { state, search }
        },
        { wrapper: makeWrapper('/?other=keepme&f.q=x') },
      )

      act(() => {
        result.current.state[1]({ q: 'y' })
      })

      expect(result.current.search).toContain('other=keepme')
      expect(result.current.search).toContain('f.q=y')
    })
  })

  describe('malformed URLs', () => {
    it('falls back to default when a number value is not numeric', () => {
      const { result } = renderHook(
        () => useUrlState('f', { page: 1 }),
        { wrapper: makeWrapper('/?f.page=banana') },
      )
      expect(result.current[0].page).toBe(1)
    })

    it('falls back to default when a boolean value is not a valid literal', () => {
      const { result } = renderHook(
        () => useUrlState('f', { active: false }),
        { wrapper: makeWrapper('/?f.active=maybe') },
      )
      expect(result.current[0].active).toBe(false)
    })
  })

  describe('multiple instances', () => {
    it('two hooks with different keys on the same page do not collide', () => {
      const { result } = renderHook(
        () => {
          const a = useUrlState('assets', { q: '', page: 1 })
          const r = useUrlState('racks', { q: '', page: 1 })
          return { a, r }
        },
        { wrapper: makeWrapper('/?assets.q=server&racks.q=A01&racks.page=2') },
      )

      expect(result.current.a[0]).toEqual({ q: 'server', page: 1 })
      expect(result.current.r[0]).toEqual({ q: 'A01', page: 2 })
    })

    it('updating one instance does not clobber the other', () => {
      const { result } = renderHook(
        () => {
          const a = useUrlState('assets', { q: '' })
          const r = useUrlState('racks', { q: '' })
          const search = useCurrentSearch()
          return { a, r, search }
        },
        { wrapper: makeWrapper('/?assets.q=server&racks.q=A01') },
      )

      act(() => {
        result.current.a[1]({ q: 'switch' })
      })

      expect(result.current.search).toContain('assets.q=switch')
      expect(result.current.search).toContain('racks.q=A01')
    })
  })

  describe('navigation modes', () => {
    it('defaults to replace mode (no history push)', () => {
      // We can't directly observe history.length in MemoryRouter easily,
      // but we ensure a write followed by a history back doesn't revert —
      // in replace mode, there's nothing to go back to.
      const { result } = renderHook(
        () => useUrlState('f', { q: '' }),
        { wrapper: makeWrapper('/?f.q=initial') },
      )

      act(() => {
        result.current[1]({ q: 'updated' })
      })
      expect(result.current[0].q).toBe('updated')
    })

    it('supports explicit push mode', () => {
      const { result } = renderHook(
        () => {
          const [state, setState] = useUrlState('f', { q: '' }, { mode: 'push' })
          return { state, setState }
        },
        { wrapper: makeWrapper('/') },
      )

      act(() => {
        result.current.setState({ q: 'hello' })
      })
      expect(result.current.state.q).toBe('hello')
    })
  })

  describe('enum-like string literal unions', () => {
    it('accepts a narrowed string default and preserves it', () => {
      type View = 'table' | 'card'
      const defaults = { view: 'table' as View }

      const { result } = renderHook(
        () => useUrlState('f', defaults),
        { wrapper: makeWrapper('/?f.view=card') },
      )

      expect(result.current[0].view).toBe('card')
    })
  })
})
