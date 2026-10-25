import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render } from '@testing-library/react'
import { shouldIgnoreShortcut, useGlobalShortcuts } from './useGlobalShortcuts'

const push = vi.fn()
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/dashboard',
}))

function Harness() {
  useGlobalShortcuts()
  return <p>page</p>
}

function append<K extends keyof HTMLElementTagNameMap>(tag: K, attrs: Record<string, string> = {}) {
  const el = document.createElement(tag)
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v)
  document.body.appendChild(el)
  return el
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('useGlobalShortcuts', () => {
  beforeEach(() => push.mockReset())

  it('l opens create link, q opens create QR Code', () => {
    render(<Harness />)
    fireEvent.keyDown(window, { key: 'l' })
    expect(push).toHaveBeenLastCalledWith('/dashboard/links/create')
    fireEvent.keyDown(window, { key: 'q' })
    expect(push).toHaveBeenLastCalledWith('/dashboard/qr-codes/create')
    expect(push).toHaveBeenCalledTimes(2)
  })

  it('accepts shifted keys', () => {
    render(<Harness />)
    fireEvent.keyDown(window, { key: 'L', shiftKey: true })
    expect(push).toHaveBeenCalledWith('/dashboard/links/create')
    fireEvent.keyDown(window, { key: 'Q', shiftKey: true })
    expect(push).toHaveBeenLastCalledWith('/dashboard/qr-codes/create')
  })

  it('prevents the default action on a match', () => {
    render(<Harness />)
    const ev = new KeyboardEvent('keydown', { key: 'l', cancelable: true, bubbles: true })
    window.dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(true)
  })

  it('ignores other keys', () => {
    render(<Harness />)
    fireEvent.keyDown(window, { key: 'k' })
    fireEvent.keyDown(window, { key: 'Enter' })
    expect(push).not.toHaveBeenCalled()
  })

  it('ignores keys typed into input, textarea or contenteditable', () => {
    render(<Harness />)
    fireEvent.keyDown(append('input'), { key: 'l' })
    fireEvent.keyDown(append('textarea'), { key: 'q' })
    const editable = append('div', { contenteditable: 'true' })
    const child = document.createElement('span')
    editable.appendChild(child)
    fireEvent.keyDown(child, { key: 'l' })
    expect(push).not.toHaveBeenCalled()
  })

  it('ignores keys while a dialog is open anywhere in the document', () => {
    render(<Harness />)
    append('div', { role: 'dialog' })
    fireEvent.keyDown(window, { key: 'l' })
    fireEvent.keyDown(document.body, { key: 'q' })
    expect(push).not.toHaveBeenCalled()
  })

  it('ignores ctrl, meta and alt combinations', () => {
    render(<Harness />)
    fireEvent.keyDown(window, { key: 'l', ctrlKey: true })
    fireEvent.keyDown(window, { key: 'l', metaKey: true })
    fireEvent.keyDown(window, { key: 'q', altKey: true })
    expect(push).not.toHaveBeenCalled()
  })

  it('ignores auto-repeat', () => {
    render(<Harness />)
    fireEvent.keyDown(window, { key: 'l', repeat: true })
    expect(push).not.toHaveBeenCalled()
  })

  it('removes its listener on unmount', () => {
    const { unmount } = render(<Harness />)
    unmount()
    fireEvent.keyDown(window, { key: 'l' })
    expect(push).not.toHaveBeenCalled()
  })
})

describe('shouldIgnoreShortcut', () => {
  function keyOn(target: EventTarget, init: KeyboardEventInit = {}) {
    let captured: KeyboardEvent | undefined
    const grab = (e: Event) => {
      captured = e as KeyboardEvent
    }
    target.addEventListener('keydown', grab)
    target.dispatchEvent(new KeyboardEvent('keydown', { key: 'l', bubbles: true, cancelable: true, ...init }))
    target.removeEventListener('keydown', grab)
    return captured!
  }

  it.each<[string, KeyboardEventInit, boolean]>([
    ['plain key', {}, false],
    ['shift', { shiftKey: true }, false],
    ['meta', { metaKey: true }, true],
    ['ctrl', { ctrlKey: true }, true],
    ['alt', { altKey: true }, true],
    ['repeat', { repeat: true }, true],
    ['composing', { isComposing: true }, true],
  ])('modifiers/flags: %s → %s', (_name, init, expected) => {
    expect(shouldIgnoreShortcut(keyOn(document.body, init), document)).toBe(expected)
  })

  it('ignores an already-handled event', () => {
    const ev = new KeyboardEvent('keydown', { key: 'l', cancelable: true })
    ev.preventDefault()
    expect(shouldIgnoreShortcut(ev, document)).toBe(true)
  })

  it.each<[string, () => HTMLElement, boolean]>([
    ['button', () => append('button'), false],
    ['input', () => append('input'), true],
    ['textarea', () => append('textarea'), true],
    ['select', () => append('select'), true],
    ['contenteditable=""', () => append('div', { contenteditable: '' }), true],
    ['contenteditable="true"', () => append('div', { contenteditable: 'true' }), true],
    ['contenteditable="false"', () => append('div', { contenteditable: 'false' }), false],
    ['role=textbox', () => append('div', { role: 'textbox' }), true],
    ['focused combobox trigger', () => append('button', { role: 'combobox', 'aria-expanded': 'false' }), true],
    [
      'child of contenteditable',
      () => {
        const host = append('div', { contenteditable: 'true' })
        const b = document.createElement('b')
        host.appendChild(b)
        return b
      },
      true,
    ],
  ])('target %s → %s', (_name, make, expected) => {
    expect(shouldIgnoreShortcut(keyOn(make()), document)).toBe(expected)
  })

  it.each<[string, Record<string, string>, boolean]>([
    ['dialog', { role: 'dialog' }, true],
    ['alertdialog', { role: 'alertdialog' }, true],
    ['menu', { role: 'menu' }, true],
    ['listbox', { role: 'listbox' }, true],
    ['expanded combobox', { role: 'combobox', 'aria-expanded': 'true' }, true],
    ['collapsed combobox', { role: 'combobox', 'aria-expanded': 'false' }, false],
    ['region', { role: 'region' }, false],
  ])('open overlay %s → %s', (_name, attrs, expected) => {
    append('div', attrs)
    expect(shouldIgnoreShortcut(keyOn(window), document)).toBe(expected)
  })

  it('handles a window target', () => {
    expect(shouldIgnoreShortcut(keyOn(window), document)).toBe(false)
  })
})
