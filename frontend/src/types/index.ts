export interface User {
  id: number;
  email: string;
  display_name?: string;
  created_at: string;
  updated_at: string;
}

export interface Link {
  id: number;
  user_id: number;
  short_code: string;
  original_url: string;
  title?: string;
  expires_at?: string;
  is_active: boolean;
  domain_id?: number;
  created_at: string;
  updated_at: string;
  short_url: string;
}

export interface Domain {
  id: number;
  user_id: number;
  domain: string;
  created_at: string;
}

export interface DomainsListResponse {
  domains: Domain[];
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface LinksListResponse {
  links: Link[];
  total: number;
  page: number;
  total_pages: number;
}

export interface DailyStats {
  date: string;
  clicks: number;
}

export interface ReferrerStats {
  referrer: string;
  count: number;
}

export interface DeviceStats {
  device_type: string;
  count: number;
}

export interface BrowserStats {
  browser: string;
  count: number;
}

export interface CountryStats {
  code: string;
  name: string;
  clicks: number;
  percentage: number;
}

export interface CityStats {
  name: string;
  country: string;
  clicks: number;
  percentage: number;
}

export interface LocationStats {
  countries: CountryStats[];
  cities: CityStats[];
}

export interface LinkStats {
  total_clicks: number;
  unique_visitors: number;
  daily_stats: DailyStats[];
  top_referrers: ReferrerStats[];
  device_stats: DeviceStats[];
  browser_stats: BrowserStats[];
  locations: LocationStats;
}

export interface Passkey {
  id: number;
  user_id: number;
  name: string;
  created_at: string;
  last_used_at?: string;
}
