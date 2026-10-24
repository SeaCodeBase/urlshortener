import { Facebook, Globe, Instagram, Linkedin, Mail, Music2, Twitter, Youtube, type LucideIcon } from 'lucide-react'
import type { PageDesign, PageTheme, SocialPlatform } from '@/types'

/** The five design presets (spec §5.7); the Design tab starts from one of these. */
export const DESIGN_PRESETS: Record<PageTheme, PageDesign> = {
  classic: {
    theme: 'classic',
    background: { type: 'solid', color: '#F4F6FA', color2: '#FFFFFF' },
    text_color: '#273144',
    button: { style: 'fill', radius: 'rounded', color: '#2A5BD7', text_color: '#FFFFFF' },
    font: 'inter',
  },
  bold: {
    theme: 'bold',
    background: { type: 'solid', color: '#111827', color2: '#111827' },
    text_color: '#F9FAFB',
    button: { style: 'fill', radius: 'pill', color: '#F59E0B', text_color: '#111827' },
    font: 'inter',
  },
  minimal: {
    theme: 'minimal',
    background: { type: 'solid', color: '#FFFFFF', color2: '#FFFFFF' },
    text_color: '#111827',
    button: { style: 'outline', radius: 'square', color: '#111827', text_color: '#111827' },
    font: 'serif',
  },
  dark: {
    theme: 'dark',
    background: { type: 'gradient', color: '#0F172A', color2: '#1E293B' },
    text_color: '#E2E8F0',
    button: { style: 'fill', radius: 'rounded', color: '#38BDF8', text_color: '#0F172A' },
    font: 'mono',
  },
  sunset: {
    theme: 'sunset',
    background: { type: 'gradient', color: '#F97316', color2: '#EC4899' },
    text_color: '#FFFFFF',
    button: { style: 'fill', radius: 'pill', color: '#FFFFFF', text_color: '#9A3412' },
    font: 'inter',
  },
}

export const PRESET_THEMES = Object.keys(DESIGN_PRESETS) as PageTheme[]

/** Slug shape (length 3–40 is checked by isValidSlug). */
export const SLUG_RE = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/
export const SLUG_MIN = 3
export const SLUG_MAX = 40

export const RESERVED_SLUGS: ReadonlySet<string> = new Set([
  'api',
  'admin',
  'm',
  'dashboard',
  'login',
  'register',
  'static',
  'assets',
  'health',
])

/** Regex + length; reserved words are a separate check. */
export function isValidSlug(slug: string): boolean {
  return slug.length >= SLUG_MIN && slug.length <= SLUG_MAX && SLUG_RE.test(slug)
}

export function isReservedSlug(slug: string): boolean {
  return RESERVED_SLUGS.has(slug.toLowerCase())
}

export interface SocialPlatformInfo {
  label: string
  icon: LucideIcon
}

/** Label + lucide icon per platform (TikTok has no lucide icon → Music2). */
export const SOCIAL_PLATFORMS: Record<SocialPlatform, SocialPlatformInfo> = {
  instagram: { label: 'Instagram', icon: Instagram },
  x: { label: 'X', icon: Twitter },
  facebook: { label: 'Facebook', icon: Facebook },
  tiktok: { label: 'TikTok', icon: Music2 },
  youtube: { label: 'YouTube', icon: Youtube },
  linkedin: { label: 'LinkedIn', icon: Linkedin },
  email: { label: 'Email', icon: Mail },
  website: { label: 'Website', icon: Globe },
}

export const SOCIAL_PLATFORM_KEYS = Object.keys(SOCIAL_PLATFORMS) as SocialPlatform[]

/** Shareable page URL `{baseUrl}/m/{slug}`. */
export function pageUrl(baseUrl: string, slug: string): string {
  return `${baseUrl.replace(/\/+$/, '')}/m/${slug}`
}
