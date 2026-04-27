import { Routes, Route, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import MainLayout from './layouts/MainLayout'
import AuthGuard from './components/AuthGuard'
import SyncingOverlay from './components/SyncingOverlay'

// Location hierarchy
const GlobalOverview = lazy(() => import('./pages/locations/GlobalOverview'))
const RegionOverview = lazy(() => import('./pages/locations/RegionOverview'))
const CityOverview = lazy(() => import('./pages/locations/CityOverview'))
const CampusOverview = lazy(() => import('./pages/locations/CampusOverview'))

// Dashboard
const Dashboard = lazy(() => import('./pages/Dashboard'))

// Assets (unified)
const AssetManagement = lazy(() => import('./pages/AssetManagementUnified'))
const AssetDetail = lazy(() => import('./pages/asset-detail/AssetDetailUnified'))
const AssetLifecycle = lazy(() => import('./pages/AssetLifecycle'))
const AssetLifecycleTimeline = lazy(() => import('./pages/AssetLifecycleTimeline'))
const Services = lazy(() => import('./pages/Services'))
const ServiceDetail = lazy(() => import('./pages/ServiceDetail'))
const AutoDiscovery = lazy(() => import('./pages/AutoDiscovery'))
const ComponentUpgradeRecommendations = lazy(() => import('./pages/ComponentUpgradeRecommendations'))
const EquipmentHealthOverview = lazy(() => import('./pages/EquipmentHealthOverview'))

// Racks / Locations (unified)
const RackManagement = lazy(() => import('./pages/RackManagement'))
const RackDetail = lazy(() => import('./pages/RackDetailUnified'))
const DataCenter3D = lazy(() => import('./pages/DataCenter3D'))
const FacilityMap = lazy(() => import('./pages/FacilityMap'))
const AddNewRack = lazy(() => import('./pages/AddNewRack'))

// Inventory
const HighSpeedInventory = lazy(() => import('./pages/HighSpeedInventory'))
const InventoryItemDetail = lazy(() => import('./pages/InventoryItemDetail'))

// Monitoring (unified energy)
const MonitoringAlerts = lazy(() => import('./pages/MonitoringAlerts'))
const IncidentDetail = lazy(() => import('./pages/IncidentDetail'))
const Problems = lazy(() => import('./pages/Problems'))
const ProblemDetail = lazy(() => import('./pages/ProblemDetail'))
const Changes = lazy(() => import('./pages/Changes'))
const ChangeDetail = lazy(() => import('./pages/ChangeDetail'))
const EnergyTariffs = lazy(() => import('./pages/EnergyTariffs'))
const EnergyBill = lazy(() => import('./pages/EnergyBill'))
const EnergyPue = lazy(() => import('./pages/EnergyPue'))
const EnergyAnomalies = lazy(() => import('./pages/EnergyAnomalies'))
const SystemHealth = lazy(() => import('./pages/SystemHealth'))
const SensorConfiguration = lazy(() => import('./pages/SensorConfiguration'))
const EnergyMonitor = lazy(() => import('./pages/EnergyMonitor'))
const AlertTopologyAnalysis = lazy(() => import('./pages/AlertTopologyAnalysis'))
const LocationDetection = lazy(() => import('./pages/LocationDetection'))

// Maintenance (unified)
const MaintenanceHub = lazy(() => import('./pages/MaintenanceHub'))
const MaintenanceTaskView = lazy(() => import('./pages/MaintenanceTaskView'))
const WorkOrder = lazy(() => import('./pages/WorkOrder'))
const AddMaintenanceTask = lazy(() => import('./pages/AddMaintenanceTask'))

// Predictive AI (unified)
const PredictiveHub = lazy(() => import('./pages/PredictiveHub'))
const PredictiveRefreshPage = lazy(() => import('./pages/PredictiveRefresh'))
const PredictiveCapex = lazy(() => import('./pages/PredictiveCapex'))
const MetricSourcesPage = lazy(() => import('./pages/MetricSources'))
const MetricsFreshnessPage = lazy(() => import('./pages/MetricsFreshness'))

// Audit
const AuditHistory = lazy(() => import('./pages/AuditHistory'))
const AuditEventDetail = lazy(() => import('./pages/AuditEventDetail'))

// Quality
const QualityDashboard = lazy(() => import('./pages/QualityDashboard'))

// System
const RolesPermissions = lazy(() => import('./pages/RolesPermissions'))
const SystemSettings = lazy(() => import('./pages/SystemSettings'))
const SyncManagement = lazy(() => import('./pages/SyncManagement'))
const TaskDispatch = lazy(() => import('./pages/TaskDispatch'))
const UserProfile = lazy(() => import('./pages/UserProfile'))

// Help / Knowledge
const TroubleshootingGuide = lazy(() => import('./pages/TroubleshootingGuide'))
const VideoLibrary = lazy(() => import('./pages/VideoLibrary'))
const VideoPlayer = lazy(() => import('./pages/VideoPlayer'))

// BIA (Business Impact Analysis)
const BIAOverview = lazy(() => import('./pages/bia/BIAOverview'))
const SystemGrading = lazy(() => import('./pages/bia/SystemGrading'))
const RtoRpoMatrices = lazy(() => import('./pages/bia/RtoRpoMatrices'))
const ScoringRules = lazy(() => import('./pages/bia/ScoringRules'))
const DependencyMap = lazy(() => import('./pages/bia/DependencyMap'))

// Onboarding
const Welcome = lazy(() => import('./pages/Welcome'))

const Login = lazy(() => import('./pages/Login'))

function Loading() {
  return (
    <div className="flex items-center justify-center h-64">
      <div className="flex flex-col items-center gap-3">
        <div className="w-8 h-8 border-2 border-primary/30 border-t-primary rounded-full animate-spin" />
        <span className="text-sm text-on-surface-variant">Loading...</span>
      </div>
    </div>
  )
}

export default function App() {
  return (
    <>
      <SyncingOverlay />
      <Suspense fallback={<Loading />}>
        <Routes>
        {/* Public routes */}
        <Route path="/login" element={<Login />} />
        <Route path="/welcome" element={<Welcome />} />

        {/* Protected routes */}
        <Route element={<AuthGuard><MainLayout /></AuthGuard>}>
          {/* Location hierarchy */}
          <Route path="/locations" element={<GlobalOverview />} />
          <Route path="/locations/:territorySlug" element={<RegionOverview />} />
          <Route path="/locations/:territorySlug/:regionSlug" element={<CityOverview />} />
          <Route path="/locations/:territorySlug/:regionSlug/:citySlug" element={<CampusOverview />} />

          {/* Dashboard */}
          <Route path="/dashboard" element={<Dashboard />} />

          {/* Assets */}
          <Route path="/assets" element={<AssetManagement />} />
          <Route path="/assets/detail" element={<AssetDetail />} />
          <Route path="/assets/:assetId" element={<AssetDetail />} />
          <Route path="/assets/lifecycle" element={<AssetLifecycle />} />
          <Route path="/assets/lifecycle/timeline/:assetId" element={<AssetLifecycleTimeline />} />
          <Route path="/assets/discovery" element={<AutoDiscovery />} />
          <Route path="/assets/upgrades" element={<ComponentUpgradeRecommendations />} />
          <Route path="/assets/equipment-health" element={<EquipmentHealthOverview />} />

          {/* Business Services (Wave 2) */}
          <Route path="/services" element={<Services />} />
          <Route path="/services/:id" element={<ServiceDetail />} />

          {/* Racks / Locations */}
          <Route path="/racks" element={<RackManagement />} />
          <Route path="/racks/detail" element={<RackDetail />} />
          <Route path="/racks/:id" element={<RackDetail />} />
          <Route path="/racks/3d" element={<DataCenter3D />} />
          <Route path="/racks/add" element={<AddNewRack />} />
          <Route path="/racks/facility-map" element={<FacilityMap />} />

          {/* Inventory */}
          <Route path="/inventory" element={<HighSpeedInventory />} />
          <Route path="/inventory/detail" element={<InventoryItemDetail />} />

          {/* Monitoring */}
          <Route path="/monitoring" element={<MonitoringAlerts />} />
          <Route path="/monitoring/incidents/:id" element={<IncidentDetail />} />
          <Route path="/monitoring/problems" element={<Problems />} />
          <Route path="/monitoring/problems/:id" element={<ProblemDetail />} />
          <Route path="/monitoring/changes" element={<Changes />} />
          <Route path="/monitoring/changes/:id" element={<ChangeDetail />} />
          <Route path="/monitoring/energy/tariffs" element={<EnergyTariffs />} />
          <Route path="/monitoring/energy/bill" element={<EnergyBill />} />
          <Route path="/monitoring/energy/pue" element={<EnergyPue />} />
          <Route path="/monitoring/energy/anomalies" element={<EnergyAnomalies />} />
          <Route path="/monitoring/health" element={<SystemHealth />} />
          <Route path="/monitoring/sensors" element={<SensorConfiguration />} />
          <Route path="/monitoring/energy" element={<EnergyMonitor />} />
          <Route path="/monitoring/topology" element={<AlertTopologyAnalysis />} />
          <Route path="/monitoring/location-detect" element={<LocationDetection />} />

          {/* Maintenance */}
          <Route path="/maintenance" element={<MaintenanceHub />} />
          <Route path="/maintenance/task" element={<MaintenanceTaskView />} />
          <Route path="/maintenance/task/:id" element={<MaintenanceTaskView />} />
          <Route path="/maintenance/workorder" element={<WorkOrder />} />
          <Route path="/maintenance/add" element={<AddMaintenanceTask />} />
          <Route path="/maintenance/dispatch" element={<TaskDispatch />} />

          {/* Predictive AI */}
          <Route path="/predictive" element={<PredictiveHub />} />
          <Route path="/predictive/refresh" element={<PredictiveRefreshPage />} />
          <Route path="/predictive/capex" element={<PredictiveCapex />} />

          {/* Audit */}
          <Route path="/audit" element={<AuditHistory />} />
          <Route path="/audit/detail" element={<AuditEventDetail />} />

          {/* Quality */}
          <Route path="/quality" element={<QualityDashboard />} />

          {/* System */}
          <Route path="/system" element={<RolesPermissions />} />
          <Route path="/system/settings" element={<SystemSettings />} />
          <Route path="/system/profile" element={<UserProfile />} />
          <Route path="/system/sync" element={<SyncManagement />} />
          <Route path="/system/metrics-sources" element={<MetricSourcesPage />} />
          <Route path="/system/metrics-freshness" element={<MetricsFreshnessPage />} />

          {/* BIA */}
          <Route path="/bia" element={<BIAOverview />} />
          <Route path="/bia/grading" element={<SystemGrading />} />
          <Route path="/bia/rto-rpo" element={<RtoRpoMatrices />} />
          <Route path="/bia/rules" element={<ScoringRules />} />
          <Route path="/bia/dependencies" element={<DependencyMap />} />

          {/* Help / Knowledge */}
          <Route path="/help/troubleshooting" element={<TroubleshootingGuide />} />
          <Route path="/help/videos" element={<VideoLibrary />} />
          <Route path="/help/videos/player" element={<VideoPlayer />} />
        </Route>

        {/* Default */}
        <Route path="*" element={<Navigate to="/locations" replace />} />
        </Routes>
      </Suspense>
    </>
  )
}
