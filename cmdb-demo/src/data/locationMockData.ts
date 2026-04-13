// ---------------------------------------------------------------------------
// IronGrid CMDB — Location hierarchy mock data
// ---------------------------------------------------------------------------

export interface IdcNode {
  id: string
  slug: string
  name: string
  modules: number
  racks: number
  assets: number
  pue: number
  power: number
  occupancy: number
  criticalAlerts: number
}

export interface CampusNode {
  id: string
  slug: string
  name: string
  nameEn: string
  address: string
  idcs: IdcNode[]
}

export interface CityNode {
  id: string
  slug: string
  name: string
  nameEn: string
  campuses: CampusNode[]
}

export interface RegionNode {
  id: string
  slug: string
  name: string
  nameEn: string
  cities: CityNode[]
}

export interface CountryNode {
  id: string
  slug: string
  name: string
  nameEn: string
  regions: RegionNode[]
}

export interface LocationTree {
  countries: CountryNode[]
}

// ---------------------------------------------------------------------------
// Aggregate KPI shape
// ---------------------------------------------------------------------------

export interface AggregatedKpis {
  totalIdcs: number
  totalRacks: number
  totalAssets: number
  avgPue: number
  totalPower: number
  avgOccupancy: number
  totalCriticalAlerts: number
}

// ---------------------------------------------------------------------------
// Data
// ---------------------------------------------------------------------------

export const LOCATION_TREE: LocationTree = {
  countries: [
    {
      id: 'cn',
      slug: 'china',
      name: '中國',
      nameEn: 'China',
      regions: [
        {
          id: 'cn-east',
          slug: 'east',
          name: '華東',
          nameEn: 'East China',
          cities: [
            {
              id: 'cn-east-sh',
              slug: 'shanghai',
              name: '上海',
              nameEn: 'Shanghai',
              campuses: [
                {
                  id: 'cn-east-sh-pd',
                  slug: 'pudong',
                  name: '浦東園區',
                  nameEn: 'Pudong Campus',
                  address: '上海市浦東新區張江高科技園區',
                  idcs: [
                    { id: 'idc-01', slug: 'idc-01', name: 'IDC-01', modules: 12, racks: 240, assets: 12842, pue: 1.22, power: 1248.5, occupancy: 84.2, criticalAlerts: 4 },
                    { id: 'idc-02', slug: 'idc-02', name: 'IDC-02', modules: 8, racks: 160, assets: 8560, pue: 1.19, power: 890.2, occupancy: 72.1, criticalAlerts: 1 },
                    { id: 'idc-03', slug: 'idc-03', name: 'IDC-03', modules: 6, racks: 80, assets: 3200, pue: 1.28, power: 420.8, occupancy: 45.5, criticalAlerts: 0 },
                  ],
                },
                {
                  id: 'cn-east-sh-jd',
                  slug: 'jiading',
                  name: '嘉定園區',
                  nameEn: 'Jiading Campus',
                  address: '上海市嘉定區工業園區',
                  idcs: [
                    { id: 'idc-04', slug: 'idc-04', name: 'IDC-04', modules: 4, racks: 60, assets: 2100, pue: 1.35, power: 280.4, occupancy: 55.0, criticalAlerts: 0 },
                  ],
                },
              ],
            },
            {
              id: 'cn-east-nj',
              slug: 'nanjing',
              name: '南京',
              nameEn: 'Nanjing',
              campuses: [
                {
                  id: 'cn-east-nj-jn',
                  slug: 'jiangning',
                  name: '江寧園區',
                  nameEn: 'Jiangning Campus',
                  address: '南京市江寧區科技園',
                  idcs: [
                    { id: 'idc-05', slug: 'idc-05', name: 'IDC-05', modules: 6, racks: 120, assets: 5400, pue: 1.31, power: 680.0, occupancy: 68.0, criticalAlerts: 2 },
                  ],
                },
              ],
            },
          ],
        },
        {
          id: 'cn-south',
          slug: 'south',
          name: '華南',
          nameEn: 'South China',
          cities: [
            {
              id: 'cn-south-gz',
              slug: 'guangzhou',
              name: '廣州',
              nameEn: 'Guangzhou',
              campuses: [
                {
                  id: 'cn-south-gz-thp',
                  slug: 'tianhe',
                  name: '天河園區',
                  nameEn: 'Tianhe Campus',
                  address: '廣州市天河區科技園',
                  idcs: [
                    { id: 'idc-06', slug: 'idc-06', name: 'IDC-06', modules: 10, racks: 200, assets: 9800, pue: 1.24, power: 1050.0, occupancy: 78.5, criticalAlerts: 1 },
                    { id: 'idc-07', slug: 'idc-07', name: 'IDC-07', modules: 8, racks: 180, assets: 7200, pue: 1.26, power: 920.0, occupancy: 71.0, criticalAlerts: 0 },
                  ],
                },
              ],
            },
            {
              id: 'cn-south-sz',
              slug: 'shenzhen',
              name: '深圳',
              nameEn: 'Shenzhen',
              campuses: [
                {
                  id: 'cn-south-sz-ns',
                  slug: 'nanshan',
                  name: '南山園區',
                  nameEn: 'Nanshan Campus',
                  address: '深圳市南山區科技園',
                  idcs: [
                    { id: 'idc-08', slug: 'idc-08', name: 'IDC-08', modules: 12, racks: 260, assets: 11200, pue: 1.20, power: 1320.0, occupancy: 82.0, criticalAlerts: 3 },
                    { id: 'idc-09', slug: 'idc-09', name: 'IDC-09', modules: 6, racks: 140, assets: 5600, pue: 1.22, power: 680.0, occupancy: 65.0, criticalAlerts: 0 },
                  ],
                },
              ],
            },
          ],
        },
        {
          id: 'cn-north',
          slug: 'north',
          name: '華北',
          nameEn: 'North China',
          cities: [
            {
              id: 'cn-north-bj',
              slug: 'beijing',
              name: '北京',
              nameEn: 'Beijing',
              campuses: [
                {
                  id: 'cn-north-bj-yq',
                  slug: 'yizhuang',
                  name: '亦莊園區',
                  nameEn: 'Yizhuang Campus',
                  address: '北京市大興區亦莊經濟開發區',
                  idcs: [
                    { id: 'idc-10', slug: 'idc-10', name: 'IDC-10', modules: 8, racks: 180, assets: 8400, pue: 1.30, power: 960.0, occupancy: 75.0, criticalAlerts: 1 },
                    { id: 'idc-11', slug: 'idc-11', name: 'IDC-11', modules: 5, racks: 100, assets: 4200, pue: 1.33, power: 520.0, occupancy: 60.0, criticalAlerts: 0 },
                  ],
                },
              ],
            },
          ],
        },
      ],
    },
    {
      id: 'jp',
      slug: 'japan',
      name: '日本',
      nameEn: 'Japan',
      regions: [
        {
          id: 'jp-kanto',
          slug: 'kanto',
          name: '關東',
          nameEn: 'Kanto',
          cities: [
            {
              id: 'jp-kanto-tk',
              slug: 'tokyo',
              name: '東京',
              nameEn: 'Tokyo',
              campuses: [
                {
                  id: 'jp-kanto-tk-ct',
                  slug: 'chiyoda',
                  name: '千代田園區',
                  nameEn: 'Chiyoda Campus',
                  address: 'Tokyo, Chiyoda-ku',
                  idcs: [
                    { id: 'idc-12', slug: 'idc-12', name: 'IDC-12', modules: 6, racks: 140, assets: 6800, pue: 1.18, power: 780.0, occupancy: 82.0, criticalAlerts: 0 },
                    { id: 'idc-13', slug: 'idc-13', name: 'IDC-13', modules: 4, racks: 80, assets: 3400, pue: 1.21, power: 420.0, occupancy: 58.0, criticalAlerts: 1 },
                  ],
                },
              ],
            },
          ],
        },
      ],
    },
    {
      id: 'sg',
      slug: 'singapore',
      name: '新加坡',
      nameEn: 'Singapore',
      regions: [
        {
          id: 'sg-central',
          slug: 'central',
          name: '中部',
          nameEn: 'Central',
          cities: [
            {
              id: 'sg-central-sg',
              slug: 'singapore-city',
              name: '新加坡市',
              nameEn: 'Singapore City',
              campuses: [
                {
                  id: 'sg-central-sg-jt',
                  slug: 'jurong',
                  name: 'Jurong 園區',
                  nameEn: 'Jurong Campus',
                  address: 'Jurong East, Singapore',
                  idcs: [
                    { id: 'idc-14', slug: 'idc-14', name: 'IDC-14', modules: 10, racks: 220, assets: 9600, pue: 1.15, power: 1100.0, occupancy: 86.0, criticalAlerts: 0 },
                    { id: 'idc-15', slug: 'idc-15', name: 'IDC-15', modules: 8, racks: 180, assets: 7800, pue: 1.17, power: 920.0, occupancy: 80.0, criticalAlerts: 1 },
                  ],
                },
              ],
            },
          ],
        },
      ],
    },
  ],
}

// ---------------------------------------------------------------------------
// Lookup helpers
// ---------------------------------------------------------------------------

export function getCountryBySlug(slug: string): CountryNode | undefined {
  return LOCATION_TREE.countries.find((c) => c.slug === slug)
}

export function getRegionBySlug(
  countrySlug: string,
  regionSlug: string,
): RegionNode | undefined {
  const country = getCountryBySlug(countrySlug)
  return country?.regions.find((r) => r.slug === regionSlug)
}

export function getCityBySlug(
  countrySlug: string,
  regionSlug: string,
  citySlug: string,
): CityNode | undefined {
  const region = getRegionBySlug(countrySlug, regionSlug)
  return region?.cities.find((c) => c.slug === citySlug)
}

export function getCampusBySlug(
  countrySlug: string,
  regionSlug: string,
  citySlug: string,
  campusSlug: string,
): CampusNode | undefined {
  const city = getCityBySlug(countrySlug, regionSlug, citySlug)
  return city?.campuses.find((c) => c.slug === campusSlug)
}

// ---------------------------------------------------------------------------
// Aggregation helpers
// ---------------------------------------------------------------------------

function aggregateIdcs(idcs: IdcNode[]): AggregatedKpis {
  if (idcs.length === 0) {
    return {
      totalIdcs: 0,
      totalRacks: 0,
      totalAssets: 0,
      avgPue: 0,
      totalPower: 0,
      avgOccupancy: 0,
      totalCriticalAlerts: 0,
    }
  }

  let totalRacks = 0
  let totalAssets = 0
  let totalPower = 0
  let totalCriticalAlerts = 0
  let weightedPueSum = 0
  let weightedOccupancySum = 0

  for (const idc of idcs) {
    totalRacks += idc.racks
    totalAssets += idc.assets
    totalPower += idc.power
    totalCriticalAlerts += idc.criticalAlerts
    // Weight PUE and occupancy by power consumption
    weightedPueSum += idc.pue * idc.power
    weightedOccupancySum += idc.occupancy * idc.racks
  }

  return {
    totalIdcs: idcs.length,
    totalRacks,
    totalAssets,
    avgPue: totalPower > 0 ? +(weightedPueSum / totalPower).toFixed(2) : 0,
    totalPower: +totalPower.toFixed(1),
    avgOccupancy:
      totalRacks > 0 ? +(weightedOccupancySum / totalRacks).toFixed(1) : 0,
    totalCriticalAlerts,
  }
}

export function aggregateCampusKpis(campus: CampusNode): AggregatedKpis {
  return aggregateIdcs(campus.idcs)
}

export function aggregateCityKpis(city: CityNode): AggregatedKpis {
  const allIdcs = city.campuses.flatMap((c) => c.idcs)
  return aggregateIdcs(allIdcs)
}

export function aggregateRegionKpis(region: RegionNode): AggregatedKpis {
  const allIdcs = region.cities.flatMap((c) => c.campuses.flatMap((cp) => cp.idcs))
  return aggregateIdcs(allIdcs)
}

export function aggregateCountryKpis(country: CountryNode): AggregatedKpis {
  const allIdcs = country.regions.flatMap((r) =>
    r.cities.flatMap((c) => c.campuses.flatMap((cp) => cp.idcs)),
  )
  return aggregateIdcs(allIdcs)
}
