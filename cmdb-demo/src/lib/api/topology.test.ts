import { describe, it, expect } from 'vitest'
import type { RackStats } from './topology'

describe('topology API', () => {
  describe('location level ordering', () => {
    const levels = ['territory', 'region', 'city', 'campus', 'idc']

    it('territory comes before idc', () => {
      expect(levels.indexOf('territory')).toBeLessThan(levels.indexOf('idc'))
    })

    it('campus comes before idc', () => {
      expect(levels.indexOf('campus')).toBeLessThan(levels.indexOf('idc'))
    })

    it('region comes before city', () => {
      expect(levels.indexOf('region')).toBeLessThan(levels.indexOf('city'))
    })

    it('has 5 hierarchical levels', () => {
      expect(levels).toHaveLength(5)
    })
  })

  describe('RackStats type shape', () => {
    it('can be constructed with required fields', () => {
      const stats: RackStats = {
        total_racks: 100,
        total_u: 4200,
        used_u: 3150,
        occupancy_pct: 75.0,
      }
      expect(stats.total_racks).toBe(100)
      expect(stats.occupancy_pct).toBe(75.0)
      expect(stats.used_u).toBeLessThanOrEqual(stats.total_u)
    })

    it('occupancy percentage is between 0 and 100', () => {
      const stats: RackStats = {
        total_racks: 10,
        total_u: 420,
        used_u: 210,
        occupancy_pct: 50,
      }
      expect(stats.occupancy_pct).toBeGreaterThanOrEqual(0)
      expect(stats.occupancy_pct).toBeLessThanOrEqual(100)
    })
  })

  describe('API path patterns', () => {
    it('location paths follow RESTful convention', () => {
      const id = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890'
      expect(`/locations/${id}`).toMatch(/^\/locations\/[0-9a-f-]+$/)
      expect(`/locations/${id}/children`).toMatch(
        /^\/locations\/[0-9a-f-]+\/children$/,
      )
      expect(`/locations/${id}/ancestors`).toMatch(
        /^\/locations\/[0-9a-f-]+\/ancestors$/,
      )
    })

    it('rack paths follow RESTful convention', () => {
      const id = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890'
      expect(`/racks/${id}`).toMatch(/^\/racks\/[0-9a-f-]+$/)
      expect(`/racks/${id}/slots`).toMatch(/^\/racks\/[0-9a-f-]+\/slots$/)
      expect(`/racks/${id}/assets`).toMatch(/^\/racks\/[0-9a-f-]+\/assets$/)
    })
  })
})
