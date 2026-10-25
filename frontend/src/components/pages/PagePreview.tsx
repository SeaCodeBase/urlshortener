import type { CSSProperties } from 'react'
import { Globe } from 'lucide-react'
import type {
  BlockContentMap,
  BlockType,
  PageBlock,
  PageButtonRadius,
  PageDesign,
  PageFont,
  PublishedBlock,
  SocialItem,
} from '@/types'
import { SOCIAL_PLATFORMS } from './designPresets'

// Pure presentational component: no hooks and no 'use client', so the public
// /m/[slug] server route can render it as well as the editor.

/** Editor blocks (`PageBlock`) or published blocks (`PublishedBlock`). */
export type PreviewBlock = PageBlock | PublishedBlock

export interface PagePreviewPage {
  title: string
  description: string
  avatar_url: string
  design: PageDesign
  blocks: PreviewBlock[]
}

export interface PagePreviewProps {
  page: PagePreviewPage
  /** editor: every block (inactive at 50% opacity), links go nowhere; public: already-filtered blocks linking to short_url. */
  mode: 'editor' | 'public'
  /** Prefix for relative asset URLs (`/uploads/...`); default "" keeps them relative. */
  baseHref?: string
}

export const RADIUS: Record<PageButtonRadius, string> = { square: '4px', rounded: '12px', pill: '999px' }

export const FONT: Record<PageFont, string> = {
  inter: 'var(--font-inter), system-ui, sans-serif',
  serif: 'Georgia, "Times New Roman", serif',
  mono: 'ui-monospace, SFMono-Regular, Menlo, monospace',
}

/** The design as the CSS variables the preview reads. */
export function designVars(design: PageDesign): Record<string, string> {
  const { background: bg, button } = design
  const fill = button.style === 'fill'
  return {
    '--page-bg': bg.type === 'gradient' ? `linear-gradient(180deg, ${bg.color}, ${bg.color2})` : bg.color,
    '--page-text': design.text_color,
    '--btn-bg': fill ? button.color : 'transparent',
    '--btn-text': button.text_color,
    '--btn-border': button.color,
    '--btn-radius': RADIUS[button.radius],
    '--page-font': FONT[design.font],
  }
}

/**
 * Colours of the initial-letter avatar: a disc in the button colour with the button text colour
 * for fill buttons; outline buttons have no fill colour pair, so the letter takes the page
 * background colour (the text colour, if that matches the disc).
 */
export function avatarColours(design: PageDesign): { background: string; color: string } {
  const { background: bg, button } = design
  if (button.style === 'fill') return { background: button.color, color: button.text_color }
  const color = bg.color.toLowerCase() !== button.color.toLowerCase() ? bg.color : design.text_color
  return { background: button.color, color }
}

function resolve(url: string, baseHref: string): string {
  return url.startsWith('/') && !url.startsWith('//') ? `${baseHref}${url}` : url
}

function blockHref(block: PreviewBlock, mode: PagePreviewProps['mode']): string {
  if (mode === 'editor') return '#'
  if ('short_url' in block && block.short_url) return block.short_url
  if ('link' in block && block.link) return block.link.short_url
  return '#'
}

function isActive(block: PreviewBlock): boolean {
  return 'is_active' in block ? block.is_active : true
}

const buttonStyle: CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 12,
  width: '100%',
  minHeight: 56,
  padding: '10px 16px',
  background: 'var(--btn-bg)',
  color: 'var(--btn-text)',
  border: '2px solid var(--btn-border)',
  borderRadius: 'var(--btn-radius)',
  textDecoration: 'none',
  fontWeight: 600,
  textAlign: 'center',
}

function LinkBlockView({ content, href, baseHref }: { content: BlockContentMap['link']; href: string; baseHref: string }) {
  return (
    <a href={href} style={buttonStyle}>
      {content.thumbnail_url ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={resolve(content.thumbnail_url, baseHref)}
          alt=""
          width={40}
          height={40}
          style={{ width: 40, height: 40, objectFit: 'cover', borderRadius: 8, flexShrink: 0 }}
        />
      ) : null}
      <span style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <span>{content.title}</span>
        {content.description ? (
          <span style={{ fontSize: 13, fontWeight: 400, opacity: 0.85 }}>{content.description}</span>
        ) : null}
      </span>
      {content.thumbnail_url ? <span aria-hidden style={{ width: 40, flexShrink: 0 }} /> : null}
    </a>
  )
}

function TextBlockView({ content }: { content: BlockContentMap['text'] }) {
  return (
    <div style={{ textAlign: 'center', color: 'var(--page-text)' }}>
      {content.heading ? <h2 style={{ fontSize: 18, fontWeight: 700, margin: 0 }}>{content.heading}</h2> : null}
      {content.body ? (
        <p style={{ fontSize: 15, margin: content.heading ? '6px 0 0' : 0, whiteSpace: 'pre-line' }}>{content.body}</p>
      ) : null}
    </div>
  )
}

function ImageBlockView({ content, baseHref }: { content: BlockContentMap['image']; baseHref: string }) {
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={resolve(content.url, baseHref)}
      alt={content.alt}
      style={{ display: 'block', width: '100%', height: 'auto', borderRadius: 'var(--btn-radius)' }}
    />
  )
}

function SocialIcon({ item, mode }: { item: SocialItem; mode: PagePreviewProps['mode'] }) {
  const info = SOCIAL_PLATFORMS[item.platform] ?? { label: item.platform, icon: Globe }
  const Icon = info.icon
  return (
    <a
      href={mode === 'editor' ? '#' : item.url}
      aria-label={info.label}
      title={info.label}
      data-platform={item.platform}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: 40,
        height: 40,
        color: 'var(--page-text)',
      }}
      {...(mode === 'public' && item.platform !== 'email' ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
    >
      <Icon aria-hidden size={24} />
    </a>
  )
}

function SocialBlockView({ content, mode }: { content: BlockContentMap['social']; mode: PagePreviewProps['mode'] }) {
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: 8 }}>
      {content.items.map((item, i) => (
        <SocialIcon key={`${item.platform}-${i}`} item={item} mode={mode} />
      ))}
    </div>
  )
}

function BlockView({ block, mode, baseHref }: { block: PreviewBlock; mode: PagePreviewProps['mode']; baseHref: string }) {
  switch (block.type) {
    case 'link':
      return <LinkBlockView content={block.content} href={blockHref(block, mode)} baseHref={baseHref} />
    case 'text':
      return <TextBlockView content={block.content} />
    case 'image':
      return <ImageBlockView content={block.content} baseHref={baseHref} />
    case 'social':
      return <SocialBlockView content={block.content} mode={mode} />
    default:
      return null
  }
}

function initials(title: string): string {
  return title.trim().charAt(0).toUpperCase() || '?'
}

/** The link-in-bio page: avatar, title, description and blocks in the given order, styled from `design`. */
export function PagePreview({ page, mode, baseHref = '' }: PagePreviewProps) {
  const rootStyle = {
    ...designVars(page.design),
    background: 'var(--page-bg)',
    color: 'var(--page-text)',
    fontFamily: 'var(--page-font)',
    minHeight: '100%',
    width: '100%',
    padding: '40px 20px',
  } as CSSProperties

  return (
    <div style={rootStyle} data-theme={page.design.theme}>
      <div style={{ maxWidth: 560, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: 16 }}>
        <header style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8, textAlign: 'center' }}>
          {page.avatar_url ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={resolve(page.avatar_url, baseHref)}
              alt=""
              width={96}
              height={96}
              style={{ width: 96, height: 96, borderRadius: '50%', objectFit: 'cover' }}
            />
          ) : (
            <div
              aria-hidden
              style={{
                width: 96,
                height: 96,
                borderRadius: '50%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: 36,
                fontWeight: 700,
                ...avatarColours(page.design),
              }}
            >
              {initials(page.title)}
            </div>
          )}
          {page.title ? <h1 style={{ fontSize: 22, fontWeight: 700, margin: 0 }}>{page.title}</h1> : null}
          {page.description ? (
            <p style={{ fontSize: 15, margin: 0, opacity: 0.9, whiteSpace: 'pre-line' }}>{page.description}</p>
          ) : null}
        </header>
        {page.blocks.map((block) => (
          <div
            key={block.id}
            data-block-id={block.id}
            data-block-type={block.type satisfies BlockType}
            style={mode === 'editor' && !isActive(block) ? { opacity: 0.5 } : undefined}
          >
            <BlockView block={block} mode={mode} baseHref={baseHref} />
          </div>
        ))}
      </div>
    </div>
  )
}
