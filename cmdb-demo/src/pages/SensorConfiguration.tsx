// Re-export shim: the real implementation lives under ./sensor-configuration/.
// Kept here so existing route imports (`./pages/SensorConfiguration`) continue
// to resolve without change after the phase 3.2 split.
export { default } from './sensor-configuration/SensorConfiguration'
