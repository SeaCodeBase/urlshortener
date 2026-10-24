// Browser only: qr-code-styling touches window/document at import time, so it
// is imported dynamically inside each function, never at module top level.
import type QRCodeStyling from 'qr-code-styling';
import { downloadBlob } from '@/lib/download';
import type { QrCode, QrDesign } from '@/types';
import { frameInsets, normalizeQrDesign, toStylingOptions, type FrameInsets } from './design';

export type QrImageFormat = 'png' | 'svg' | 'jpeg';

const FILE_EXTENSIONS: Record<QrImageFormat, string> = { png: 'png', svg: 'svg', jpeg: 'jpg' };
const MIME_TYPES: Record<QrImageFormat, string> = { png: 'image/png', svg: 'image/svg+xml', jpeg: 'image/jpeg' };
const FONT_FAMILY = 'system-ui, -apple-system, "Segoe UI", Roboto, sans-serif';

/** A qr-code-styling instance for the bare code (no frame). */
export async function createQrInstance(design: QrDesign, data: string, size: number): Promise<QRCodeStyling> {
  const { default: Styling } = await import('qr-code-styling');
  return new Styling(toStylingOptions(design, data, size));
}

/** Where the frame text goes: centered in the text band, relative to the full image. */
function frameTextLayout(insets: FrameInsets, size: number) {
  const bandTop = insets.top > 0 ? insets.border : insets.border + size;
  return {
    x: insets.border + size / 2,
    y: bandTop + insets.textHeight / 2,
    fontSize: Math.round(insets.textHeight * 0.5),
  };
}

function loadImage(blob: Blob): Promise<HTMLImageElement> {
  const url = URL.createObjectURL(blob);
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(url);
      resolve(img);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error('Could not load the QR image'));
    };
    img.src = url;
  });
}

async function composeRaster(code: Blob, design: QrDesign, size: number, format: 'png' | 'jpeg'): Promise<Blob> {
  const insets = frameInsets(design, size);
  const canvas = document.createElement('canvas');
  canvas.width = size + 2 * insets.border;
  canvas.height = size + 2 * insets.border + insets.top + insets.bottom;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('Canvas is not supported');

  // The frame fill covers the whole canvas, so the output is opaque (JPEG has no alpha).
  ctx.fillStyle = design.frame_color;
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.drawImage(await loadImage(code), insets.border, insets.border + insets.top, size, size);

  if (insets.textHeight > 0 && design.frame_text) {
    const t = frameTextLayout(insets, size);
    ctx.fillStyle = design.background;
    ctx.font = `600 ${t.fontSize}px ${FONT_FAMILY}`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(design.frame_text, t.x, t.y, size);
  }

  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('Could not encode the QR image'))), MIME_TYPES[format]);
  });
}

function escapeXml(text: string): string {
  return text.replace(/[<>&"']/g, (c) => `&#${c.charCodeAt(0)};`);
}

async function composeSvg(code: Blob, design: QrDesign, size: number): Promise<Blob> {
  const insets = frameInsets(design, size);
  const width = size + 2 * insets.border;
  const height = size + 2 * insets.border + insets.top + insets.bottom;
  // Inline the code's <svg> (minus its XML declaration), positioned inside the frame.
  const inner = (await code.text())
    .replace(/^<\?xml[^>]*\?>\s*/, '')
    .replace(/^<svg\b/, `<svg x="${insets.border}" y="${insets.border + insets.top}"`);
  let text = '';
  if (insets.textHeight > 0 && design.frame_text) {
    const t = frameTextLayout(insets, size);
    text =
      `<text x="${t.x}" y="${t.y}" fill="${design.background}" font-family='${FONT_FAMILY}' font-size="${t.fontSize}" ` +
      `font-weight="600" text-anchor="middle" dominant-baseline="central">${escapeXml(design.frame_text)}</text>`;
  }
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">` +
    `<rect width="${width}" height="${height}" fill="${design.frame_color}"/>${text}${inner}</svg>`;
  return new Blob([svg], { type: MIME_TYPES.svg });
}

/** The QR image (with its frame, if any) as a Blob. */
export async function renderQrBlob({
  design,
  data,
  format,
  size = 1024,
}: {
  design: QrDesign;
  data: string;
  format: QrImageFormat;
  size?: number;
}): Promise<Blob> {
  const instance = await createQrInstance(design, data, size);
  const raw = await instance.getRawData(format);
  if (!(raw instanceof Blob)) throw new Error('Could not render the QR code');
  if (design.frame === 'none') return raw;
  return format === 'svg' ? composeSvg(raw, design, size) : composeRaster(raw, design, size, format);
}

/** `{title or back-half}.{ext}`, with path separators and reserved characters replaced. */
function qrFileName(qr: Pick<QrCode, 'title' | 'short_url'>, format: QrImageFormat): string {
  const base = qr.title?.trim() || qr.short_url.replace(/\/+$/, '').split('/').pop() || 'qr-code';
  return `${base.replace(/[\\/:*?"<>|]/g, '-')}.${FILE_EXTENSIONS[format]}`;
}

/** Renders the QR code's stored design and saves it as a file. */
export async function downloadQr(
  qr: Pick<QrCode, 'title' | 'short_url' | 'render_customizations' | 'qr_url'>,
  format: QrImageFormat,
): Promise<void> {
  const blob = await renderQrBlob({ design: normalizeQrDesign(qr.render_customizations), data: qr.qr_url, format });
  downloadBlob(qrFileName(qr, format), blob);
}
