import type { Options } from 'qr-code-styling';
import {
  DEFAULT_QR_DESIGN,
  type QrCorners,
  type QrDesign,
  type QrErrorCorrection,
  type QrFrame,
  type QrPattern,
} from '@/types';

// Pure design model: no DOM, safe to import anywhere (the qr-code-styling
// import above is type-only and erased at build time).

const PATTERNS: readonly QrPattern[] = ['standard', 'rounded', 'circle', 'diamond', 'classy'];
const CORNERS: readonly QrCorners[] = ['square', 'rounded', 'extra-rounded', 'dot'];
const FRAMES: readonly QrFrame[] = ['none', 'border', 'text-bottom', 'text-top'];
const ERROR_CORRECTIONS: readonly QrErrorCorrection[] = ['L', 'M', 'Q', 'H'];

export const MIN_LOGO_SIZE = 0.2;
export const MAX_LOGO_SIZE = 0.5;
/** Frame border width in px (preview and export). */
export const FRAME_BORDER = 8;
/** Frame text band height as a fraction of the code size. */
export const FRAME_TEXT_RATIO = 0.18;

const HEX_COLOR = /^#(?:[0-9a-f]{3}|[0-9a-f]{6})$/i;

type DotType = NonNullable<NonNullable<Options['dotsOptions']>['type']>;
type CornerSquareType = NonNullable<NonNullable<Options['cornersSquareOptions']>['type']>;
type CornerDotType = NonNullable<NonNullable<Options['cornersDotOptions']>['type']>;

const DOT_TYPES: Record<QrPattern, DotType> = {
  standard: 'square',
  rounded: 'rounded',
  circle: 'dots',
  diamond: 'classy-rounded',
  classy: 'classy',
};

const CORNER_SQUARE_TYPES: Record<QrCorners, CornerSquareType> = {
  square: 'square',
  rounded: 'rounded',
  'extra-rounded': 'extra-rounded',
  dot: 'dot',
};

function oneOf<T extends string>(value: unknown, allowed: readonly T[], fallback: T): T {
  return typeof value === 'string' && (allowed as readonly string[]).includes(value) ? (value as T) : fallback;
}

function hex(value: unknown, fallback: string): string {
  return typeof value === 'string' && HEX_COLOR.test(value) ? value : fallback;
}

/** A complete, valid design from stored `render_customizations` (or anything). */
export function normalizeQrDesign(raw: unknown): QrDesign {
  const r = raw !== null && typeof raw === 'object' ? (raw as Record<string, unknown>) : {};
  const d = DEFAULT_QR_DESIGN;
  const logoSize =
    typeof r.logo_size === 'number' && Number.isFinite(r.logo_size)
      ? Math.min(MAX_LOGO_SIZE, Math.max(MIN_LOGO_SIZE, r.logo_size))
      : d.logo_size;
  return {
    pattern: oneOf(r.pattern, PATTERNS, d.pattern),
    corners: oneOf(r.corners, CORNERS, d.corners),
    color: hex(r.color, d.color),
    background: hex(r.background, d.background),
    logo: typeof r.logo === 'string' && r.logo !== '' ? r.logo : null,
    logo_size: logoSize,
    frame: oneOf(r.frame, FRAMES, d.frame),
    frame_text: typeof r.frame_text === 'string' ? r.frame_text : d.frame_text,
    frame_color: hex(r.frame_color, d.frame_color),
    error_correction: oneOf(r.error_correction, ERROR_CORRECTIONS, d.error_correction),
  };
}

/** qr-code-styling options for the bare code (without the frame). */
export function toStylingOptions(design: QrDesign, data: string, size: number): Options {
  return {
    width: size,
    height: size,
    data,
    margin: 0,
    qrOptions: { errorCorrectionLevel: design.error_correction },
    dotsOptions: { type: DOT_TYPES[design.pattern], color: design.color },
    cornersSquareOptions: { type: CORNER_SQUARE_TYPES[design.corners], color: design.color },
    cornersDotOptions: { type: (design.corners === 'dot' ? 'dot' : 'square') satisfies CornerDotType, color: design.color },
    backgroundOptions: { color: design.background },
    image: design.logo ?? undefined,
    imageOptions: { imageSize: design.logo_size, margin: 4, crossOrigin: 'anonymous' },
  };
}

export interface FrameInsets {
  /** Border width on every side (0 without a frame). */
  border: number;
  /** Height of the text band (0 unless the frame has text). */
  textHeight: number;
  /** Extra space above the code inside the border (the text band for text-top). */
  top: number;
  /** Extra space below the code inside the border (the text band for text-bottom). */
  bottom: number;
}

/**
 * Space the frame adds around a `size` px code, so the preview and the export
 * lay it out identically: total width = size + 2·border,
 * total height = size + 2·border + top + bottom.
 */
export function frameInsets(design: QrDesign, size: number): FrameInsets {
  if (design.frame === 'none') return { border: 0, textHeight: 0, top: 0, bottom: 0 };
  const textHeight = design.frame === 'border' ? 0 : Math.round(size * FRAME_TEXT_RATIO);
  return {
    border: FRAME_BORDER,
    textHeight,
    top: design.frame === 'text-top' ? textHeight : 0,
    bottom: design.frame === 'text-bottom' ? textHeight : 0,
  };
}
