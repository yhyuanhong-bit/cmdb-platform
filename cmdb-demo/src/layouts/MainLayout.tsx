import { useState, useEffect, useCallback } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import LanguageSwitcher from '../components/LanguageSwitcher'
import { useAuthStore } from '../stores/authStore'

interface SubItem {
  labelKey: string
  to: string
}

interface NavSection {
  icon: string
  key: string
  to: string
  children?: SubItem[]
}

const navSections: NavSection[] = [
  { to: '/locations', icon: 'public', key: 'nav.locations_overview' },
  { to: '/dashboard', icon: 'dashboard', key: 'nav.dashboard' },
  {
    to: '/assets',
    icon: 'inventory_2',
    key: 'nav.assets',
    children: [
      { labelKey: 'nav.sub.asset_list', to: '/assets' },
      { labelKey: 'nav.sub.lifecycle', to: '/assets/lifecycle' },
      { labelKey: 'nav.sub.auto_discovery', to: '/assets/discovery' },
      { labelKey: 'nav.sub.upgrade_recs', to: '/assets/upgrades' },
      { labelKey: 'nav.sub.equip_health', to: '/assets/equipment-health' },
    ],
  },
  {
    to: '/racks',
    icon: 'dns',
    key: 'nav.locations',
    children: [
      { labelKey: 'nav.sub.rack_mgmt', to: '/racks' },
      { labelKey: 'nav.sub.dc_3d', to: '/racks/3d' },
      { labelKey: 'nav.sub.facility_map', to: '/racks/facility-map' },
      { labelKey: 'nav.sub.add_rack', to: '/racks/add' },
    ],
  },
  {
    to: '/inventory',
    icon: 'fact_check',
    key: 'nav.inventory',
    children: [
      { labelKey: 'nav.sub.inventory_task', to: '/inventory' },
      { labelKey: 'nav.sub.inventory_detail', to: '/inventory/detail' },
    ],
  },
  {
    to: '/monitoring',
    icon: 'monitoring',
    key: 'nav.monitoring',
    children: [
      { labelKey: 'nav.sub.alert_list', to: '/monitoring' },
      { labelKey: 'nav.sub.problems', to: '/monitoring/problems' },
      { labelKey: 'nav.sub.changes', to: '/monitoring/changes' },
      { labelKey: 'nav.sub.energy_bill', to: '/monitoring/energy/bill' },
      { labelKey: 'nav.sub.energy_pue', to: '/monitoring/energy/pue' },
      { labelKey: 'nav.sub.energy_anomalies', to: '/monitoring/energy/anomalies' },
      { labelKey: 'nav.sub.energy_tariffs', to: '/monitoring/energy/tariffs' },
      { labelKey: 'nav.sub.sys_health', to: '/monitoring/health' },
      { labelKey: 'nav.sub.topology', to: '/monitoring/topology' },
      { labelKey: 'nav.sub.sensor_config', to: '/monitoring/sensors' },
      { labelKey: 'nav.sub.energy', to: '/monitoring/energy' },
      { labelKey: 'nav.sub.location_detect', to: '/monitoring/location-detect' },
    ],
  },
  {
    to: '/maintenance',
    icon: 'build',
    key: 'nav.maintenance',
    children: [
      { labelKey: 'nav.sub.maint_schedule', to: '/maintenance' },
      { labelKey: 'nav.sub.add_task', to: '/maintenance/add' },
      { labelKey: 'nav.sub.work_order', to: '/maintenance/workorder' },
      { labelKey: 'nav.sub.task_dispatch', to: '/maintenance/dispatch' },
    ],
  },
  { to: '/predictive', icon: 'psychology', key: 'nav.predictive_ai' },
  { to: '/bia', icon: 'assessment', key: 'nav.bia_modeler' },
  {
    to: '/audit',
    icon: 'history',
    key: 'nav.audit',
    children: [
      { labelKey: 'nav.sub.audit_history', to: '/audit' },
      { labelKey: 'nav.sub.event_detail', to: '/audit/detail' },
    ],
  },
  { to: '/quality', icon: 'verified', key: 'nav.data_quality' },
  {
    to: '/system',
    icon: 'settings',
    key: 'nav.system',
    children: [
      { labelKey: 'nav.sub.permissions', to: '/system' },
      { labelKey: 'nav.sub.sys_settings', to: '/system/settings' },
      { labelKey: 'nav.sub.user_profile', to: '/system/profile' },
      { labelKey: 'nav.sub.sync_management', to: '/system/sync' },
    ],
  },
  {
    to: '/help',
    icon: 'help_outline',
    key: 'nav.help',
    children: [
      { labelKey: 'nav.sub.troubleshoot', to: '/help/troubleshooting' },
      { labelKey: 'nav.sub.video_lib', to: '/help/videos' },
    ],
  },
]

const topTabs = [
  { to: '/dashboard', key: 'nav.dashboard' },
  { to: '/assets', key: 'nav.assets' },
  { to: '/maintenance', key: 'nav.maintenance' },
  { to: '/monitoring', key: 'nav.telemetry' },
  { to: '/predictive', key: 'nav.analytics' },
  { to: '/audit', key: 'nav.reports' },
]

function findExpandedSection(pathname: string): string | null {
  for (const section of navSections) {
    if (!section.children) continue
    if (pathname === section.to || pathname.startsWith(section.to + '/')) {
      return section.key
    }
  }
  return null
}

export default function MainLayout() {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()

  const [openSection, setOpenSection] = useState<string | null>(() =>
    findExpandedSection(location.pathname)
  )

  // Auto-expand when route changes (e.g. via top tabs or direct navigation)
  useEffect(() => {
    const match = findExpandedSection(location.pathname)
    if (match) {
      setOpenSection(match)
    }
  }, [location.pathname])

  const toggleSection = useCallback(
    (section: NavSection) => {
      if (!section.children) {
        navigate(section.to)
        return
      }
      setOpenSection((prev) => (prev === section.key ? null : section.key))
    },
    [navigate]
  )

  const isSubItemActive = (to: string) => location.pathname === to

  const isSectionActive = (section: NavSection) => {
    if (!section.children) {
      return location.pathname === section.to || location.pathname.startsWith(section.to + '/')
    }
    return location.pathname === section.to || location.pathname.startsWith(section.to + '/')
  }

  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="fixed left-0 top-0 h-full z-40 flex flex-col w-56 bg-surface">
        <div className="p-5 pb-2">
          <div className="text-sky-100 font-bold tracking-widest uppercase text-sm">{t('nav.brand_line1')}</div>
          <div className="text-sky-100 font-bold tracking-widest uppercase text-sm">{t('nav.brand_line2')}</div>
          <div className="font-headline tracking-tight font-bold uppercase text-[0.6rem] text-sky-400 mt-0.5">
            {t('nav.subtitle')}
          </div>
        </div>
        <nav className="flex-1 mt-4 flex flex-col gap-0.5 overflow-y-auto">
          {navSections.map((section) => {
            const hasChildren = !!section.children
            const isOpen = openSection === section.key
            const isActive = isSectionActive(section)

            return (
              <div key={section.key}>
                {/* Main nav item */}
                {hasChildren ? (
                  <button
                    onClick={() => toggleSection(section)}
                    className={`w-full flex items-center gap-3 px-4 py-2.5 transition-colors duration-200 text-sm ${
                      isActive
                        ? 'bg-surface-container text-sky-400 border-l-2 border-sky-500'
                        : 'text-on-surface-variant hover:bg-surface-container-low border-l-2 border-transparent'
                    }`}
                  >
                    <Icon name={section.icon} className="text-[20px]" />
                    <span className="font-headline tracking-tight font-bold uppercase text-[0.6875rem] flex-1 text-left">
                      {t(section.key)}
                    </span>
                    <Icon
                      name={isOpen ? 'expand_less' : 'expand_more'}
                      className="text-[18px] opacity-60"
                    />
                  </button>
                ) : (
                  <NavLink
                    to={section.to}
                    className={({ isActive: linkActive }) =>
                      `flex items-center gap-3 px-4 py-2.5 transition-colors duration-200 text-sm ${
                        linkActive
                          ? 'bg-surface-container text-sky-400 border-l-2 border-sky-500'
                          : 'text-on-surface-variant hover:bg-surface-container-low border-l-2 border-transparent'
                      }`
                    }
                  >
                    <Icon name={section.icon} className="text-[20px]" />
                    <span className="font-headline tracking-tight font-bold uppercase text-[0.6875rem]">
                      {t(section.key)}
                    </span>
                  </NavLink>
                )}

                {/* Sub-items */}
                {hasChildren && isOpen && (
                  <div className="flex flex-col gap-0.5 py-1">
                    {section.children!.map((child) => {
                      const active = isSubItemActive(child.to)
                      return (
                        <NavLink
                          key={child.to}
                          to={child.to}
                          className={`relative flex items-center pl-11 pr-4 py-1.5 text-xs transition-colors duration-150 ${
                            active
                              ? 'text-sky-400'
                              : 'text-on-surface-variant hover:bg-surface-container-low'
                          }`}
                        >
                          {active && (
                            <span className="absolute left-7 top-1/2 -translate-y-1/2 w-1.5 h-1.5 rounded-full bg-sky-400" />
                          )}
                          {t(child.labelKey)}
                        </NavLink>
                      )
                    })}
                  </div>
                )}
              </div>
            )
          })}
        </nav>
        <div className="p-4 border-t border-outline-variant/15">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-full bg-surface-container-high flex items-center justify-center">
              <Icon name="person" className="text-[16px] text-on-surface-variant" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-xs font-semibold text-on-surface">{t('nav.admin_user')}</div>
              <div className="text-[0.6rem] text-on-surface-variant">{t('nav.system_administrator')}</div>
            </div>
            <button
              onClick={() => { useAuthStore.getState().logout(); navigate('/login'); }}
              title={t('nav.logout', 'Logout')}
              className="p-1.5 rounded hover:bg-error-container/30 text-on-surface-variant hover:text-error transition-colors"
            >
              <Icon name="logout" className="text-[18px]" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <div className="flex-1 ml-56 flex flex-col min-h-screen">
        {/* Top Bar */}
        <header className="sticky top-0 z-30 bg-surface-container-low/80 backdrop-blur-xl">
          <div className="flex items-center justify-between px-6 h-12">
            <div className="flex items-center gap-1">
              <span className="font-headline font-bold text-sm text-on-surface tracking-wide uppercase">IronGrid</span>
              <nav className="flex items-center gap-0 ml-6">
                {topTabs.map((tab) => (
                  <NavLink
                    key={tab.to}
                    to={tab.to}
                    className={({ isActive }) =>
                      `px-3 py-3 text-xs font-medium transition-colors ${
                        isActive
                          ? 'text-on-surface border-b-2 border-on-primary-container'
                          : 'text-on-surface-variant hover:text-on-surface'
                      }`
                    }
                  >
                    {t(tab.key)}
                  </NavLink>
                ))}
              </nav>
            </div>
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-2 bg-surface-container rounded px-3 py-1.5">
                <Icon name="search" className="text-[16px] text-on-surface-variant" />
                <span className="text-xs text-on-surface-variant">{t('nav.search_placeholder')}</span>
                <kbd className="text-[0.6rem] bg-surface-container-high px-1.5 py-0.5 rounded text-on-surface-variant">
                  CMD+K
                </kbd>
              </div>
              <LanguageSwitcher />
              <button className="p-1.5 hover:bg-surface-container rounded">
                <Icon name="notifications" className="text-[18px] text-on-surface-variant" />
              </button>
              <button className="p-1.5 hover:bg-surface-container rounded">
                <Icon name="settings" className="text-[18px] text-on-surface-variant" />
              </button>
              <div className="w-7 h-7 rounded-full machined-gradient flex items-center justify-center">
                <Icon name="person" className="text-[14px] text-on-primary" />
              </div>
            </div>
          </div>
        </header>

        {/* Page Content */}
        <main className="flex-1 overflow-y-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
