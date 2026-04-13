import { describe, it, expect } from 'vitest'

/**
 * Tests for utility logic extracted from LocationContext.
 *
 * The module-level functions (deriveLevel, isValidLocationNode) are not
 * exported, so we replicate the logic here to validate the patterns used
 * by the context.  When these helpers are promoted to shared utils they
 * can be imported directly.
 */

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

function isValidLocationNode(node: unknown): boolean {
  return (
    node != null &&
    typeof node === 'object' &&
    'id' in node &&
    typeof (node as { id: unknown }).id === 'string' &&
    UUID_RE.test((node as { id: string }).id)
  )
}

type LocationLevel = 'global' | 'territory' | 'region' | 'city' | 'campus' | 'idc'

interface LocationPath {
  territory?: { id: string }
  region?: { id: string }
  city?: { id: string }
  campus?: { id: string }
  idc?: { id: string }
}

function deriveLevel(path: LocationPath): LocationLevel {
  if (path.idc) return 'idc'
  if (path.campus) return 'campus'
  if (path.city) return 'city'
  if (path.region) return 'region'
  if (path.territory) return 'territory'
  return 'global'
}

describe('LocationContext utilities', () => {
  describe('UUID validation', () => {
    it('accepts valid v4 UUID', () => {
      expect(UUID_RE.test('d0000000-0000-0000-0000-000000000001')).toBe(true)
    })

    it('accepts uppercase UUID', () => {
      expect(UUID_RE.test('D0000000-0000-0000-0000-000000000001')).toBe(true)
    })

    it('rejects non-UUID strings', () => {
      expect(UUID_RE.test('not-a-uuid')).toBe(false)
      expect(UUID_RE.test('')).toBe(false)
      expect(UUID_RE.test('12345')).toBe(false)
    })

    it('rejects UUID with wrong segment lengths', () => {
      expect(UUID_RE.test('d000000-0000-0000-0000-000000000001')).toBe(false)
      expect(UUID_RE.test('d00000000-0000-0000-0000-000000000001')).toBe(false)
    })
  })

  describe('isValidLocationNode', () => {
    it('returns true for node with valid UUID id', () => {
      expect(
        isValidLocationNode({ id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890' }),
      ).toBe(true)
    })

    it('returns false for null', () => {
      expect(isValidLocationNode(null)).toBe(false)
    })

    it('returns false for node with slug instead of UUID', () => {
      expect(isValidLocationNode({ id: 'asia-pacific' })).toBe(false)
    })

    it('returns false for node without id', () => {
      expect(isValidLocationNode({ name: 'Asia' })).toBe(false)
    })

    it('returns false for non-object', () => {
      expect(isValidLocationNode('string')).toBe(false)
      expect(isValidLocationNode(42)).toBe(false)
    })
  })

  describe('deriveLevel', () => {
    it('returns global for empty path', () => {
      expect(deriveLevel({})).toBe('global')
    })

    it('returns territory when only territory is set', () => {
      expect(deriveLevel({ territory: { id: 'x' } })).toBe('territory')
    })

    it('returns deepest level present', () => {
      expect(
        deriveLevel({
          territory: { id: 'a' },
          region: { id: 'b' },
          city: { id: 'c' },
        }),
      ).toBe('city')
    })

    it('returns idc as deepest level', () => {
      expect(
        deriveLevel({
          territory: { id: 'a' },
          region: { id: 'b' },
          city: { id: 'c' },
          campus: { id: 'd' },
          idc: { id: 'e' },
        }),
      ).toBe('idc')
    })

    it('returns campus when campus is deepest', () => {
      expect(
        deriveLevel({
          territory: { id: 'a' },
          region: { id: 'b' },
          city: { id: 'c' },
          campus: { id: 'd' },
        }),
      ).toBe('campus')
    })
  })
})
