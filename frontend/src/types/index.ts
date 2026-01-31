export interface User {
  id: number;
  email: string;
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
  created_at: string;
  updated_at: string;
  short_url: string;
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
