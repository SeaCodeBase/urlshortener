import { useEffect } from 'react';
import { useRouter } from 'next/navigation';

/** Key (lower-cased) → destination. */
export const GLOBAL_SHORTCUTS: Readonly<Record<string, string>> = {
  l: '/dashboard/links/create',
  q: '/dashboard/qr-codes/create',
};

/** Elements that consume typed letters (a focused combobox does typeahead). */
const TYPING_TARGET =
  'input, textarea, select, [contenteditable=""], [contenteditable="true"], [contenteditable="plaintext-only"], [role="textbox"], [role="combobox"]';

/** Any of these in the document means an overlay is open and owns the keyboard. */
const OPEN_OVERLAY =
  '[role="dialog"], [role="alertdialog"], [role="menu"], [role="listbox"], [role="combobox"][aria-expanded="true"]';

/** True when a keydown must not trigger a global shortcut. */
export function shouldIgnoreShortcut(event: KeyboardEvent, doc: Document): boolean {
  if (event.metaKey || event.ctrlKey || event.altKey) return true;
  if (event.repeat || event.defaultPrevented || event.isComposing) return true;
  const target = event.target;
  if (target instanceof Element && target.closest(TYPING_TARGET)) return true;
  return doc.querySelector(OPEN_OVERLAY) != null;
}

/** Bitly-style `L` → create link, `Q` → create QR Code. Mount once, inside the signed-in shell. */
export function useGlobalShortcuts(): void {
  const router = useRouter();
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const href = GLOBAL_SHORTCUTS[event.key?.toLowerCase() ?? ''];
      if (!href || shouldIgnoreShortcut(event, document)) return;
      event.preventDefault();
      router.push(href);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [router]);
}
