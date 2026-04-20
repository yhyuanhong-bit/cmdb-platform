/**
 * Shared types and helpers for Rack Detail tab components.
 * Extracted verbatim from the original RackDetailUnified.tsx during the
 * phase 3.2 refactor. No behavior changes.
 */

// ---------------------------------------------------------------------------
// Local extended types (augment base API types with extra runtime fields)
// ---------------------------------------------------------------------------

/** RackSlot as returned by the list-slots endpoint — includes denormalised asset info */
export interface RackSlotDetail {
  id: string;
  rack_id: string;
  asset_id: string;
  start_u: number;
  end_u: number;
  height_u: number;
  side: string;
  asset_name?: string;
  asset_tag?: string;
  asset_type?: string;
  type?: string;
  bia_level?: string;
}

// ---------------------------------------------------------------------------
// Shared types & data (equipment slots — no API for sub-rack assets yet)
// ---------------------------------------------------------------------------

export interface Equipment {
  startU: number;
  endU: number;
  label: string;
  assetTag: string;
  type: "compute" | "network" | "storage" | "power" | "empty";
}

// Console U-slot data
export type SlotType = "pdu" | "compute" | "network" | "storage" | "ups" | "empty" | "warning";

export interface USlot {
  startU: number;
  endU: number;
  label: string;
  type: SlotType;
  warning?: boolean;
}

// Fix #17: removed hardcoded uSlots — console tab now uses API data exclusively via consoleSlots
export const uSlots: USlot[] = [];

export interface LiveAlert {
  severity: string;
  text: string;
  time: string;
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

export function getTypeColor(type: Equipment["type"]) {
  switch (type) {
    case "compute": return "bg-on-primary-container/25";
    case "network": return "bg-tertiary-container/60";
    case "storage": return "bg-secondary-container/60";
    case "power":   return "bg-surface-container-highest";
    default:        return "bg-surface-container-lowest";
  }
}

export function getTypeAccent(type: Equipment["type"]) {
  switch (type) {
    case "compute": return "bg-on-primary-container";
    case "network": return "bg-on-tertiary-container";
    case "storage": return "bg-secondary";
    case "power":   return "bg-on-surface-variant";
    default:        return "bg-surface-container-highest";
  }
}

export function getSlotColor(type: SlotType): string {
  switch (type) {
    case "pdu":     return "bg-surface-container-highest/80";
    case "compute": return "bg-on-primary-container/25";
    case "network": return "bg-primary/25";
    case "storage": return "bg-emerald-500/20";
    case "ups":     return "bg-secondary/20";
    case "warning": return "bg-tertiary/25";
    default:        return "bg-surface-container-low";
  }
}

export function getSlotAccent(type: SlotType): string {
  switch (type) {
    case "pdu":     return "bg-on-surface-variant";
    case "compute": return "bg-on-primary-container";
    case "network": return "bg-primary";
    case "storage": return "bg-emerald-500";
    case "ups":     return "bg-secondary";
    case "warning": return "bg-tertiary";
    default:        return "bg-surface-container-highest";
  }
}

// BIA tier colors for rack slots
export const SLOT_BIA_COLORS: Record<string, string> = {
  critical: 'bg-error-container text-on-error-container',
  important: 'bg-[#92400e] text-[#fbbf24]',
  normal: 'bg-[#1e3a5f] text-on-primary-container',
  minor: 'bg-surface-container-highest text-on-surface-variant',
}

export const TABS = [
  { id: "visualization", icon: "dns" },
  { id: "console",       icon: "terminal" },
  { id: "network",       icon: "lan" },
  { id: "maintenance",   icon: "build" },
] as const;

export type TabId = typeof TABS[number]["id"];
