import type { AuthResponse, User, Link, LinksListResponse } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

class ApiClient {
  private token: string | null = null;

  setToken(token: string | null) {
    this.token = token;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    // Merge any additional headers from options
    if (options.headers) {
      const optionHeaders = options.headers as Record<string, string>;
      Object.assign(headers, optionHeaders);
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.error || 'Request failed');
    }

    if (response.status === 204) {
      return {} as T;
    }

    return response.json();
  }

  // Auth
  async register(email: string, password: string) {
    return this.request<AuthResponse>('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  }

  async login(email: string, password: string) {
    return this.request<AuthResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  }

  async me() {
    return this.request<User>('/api/auth/me');
  }

  async changePassword(oldPassword: string, newPassword: string) {
    return this.request<{ message: string }>('/api/auth/password', {
      method: 'PUT',
      body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
    });
  }

  // Links
  async getLinks(page = 1, limit = 20) {
    return this.request<LinksListResponse>(`/api/links?page=${page}&limit=${limit}`);
  }

  async createLink(data: {
    original_url: string;
    custom_code?: string;
    title?: string;
    expires_at?: string;
  }) {
    return this.request<Link>('/api/links', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getLink(id: number) {
    return this.request<Link>(`/api/links/${id}`);
  }

  async updateLink(id: number, data: Partial<Link>) {
    return this.request<Link>(`/api/links/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteLink(id: number) {
    return this.request<void>(`/api/links/${id}`, {
      method: 'DELETE',
    });
  }
}

export const api = new ApiClient();
