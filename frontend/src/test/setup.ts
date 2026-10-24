// Date formatting uses the local time zone; pin it so tests are deterministic.
process.env.TZ = "UTC";

import "@testing-library/jest-dom/vitest";

// Vitest runs without globals, so Testing Library can't register its own cleanup.
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";
afterEach(() => cleanup());

// jsdom lacks these browser APIs used by cmdk / Radix popovers.
if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}
if (typeof Element !== "undefined") {
  Element.prototype.scrollIntoView ??= function () {};
  Element.prototype.hasPointerCapture ??= () => false;
  Element.prototype.releasePointerCapture ??= function () {};
}
