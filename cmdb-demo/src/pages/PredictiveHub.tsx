// Re-export shim: the real implementation lives under ./predictive/.
// Kept here so existing route imports (`./pages/PredictiveHub`) continue to
// resolve without change after the phase 3.2 split.
export { default } from './predictive/PredictiveHub'
