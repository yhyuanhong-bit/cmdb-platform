// Re-export shim: the real implementation lives under ./rack-detail/.
// Kept here so existing route imports (`./pages/RackDetailUnified`) continue
// to resolve without change after the phase 3.2 split.
export { default } from './rack-detail/RackDetailUnified'
