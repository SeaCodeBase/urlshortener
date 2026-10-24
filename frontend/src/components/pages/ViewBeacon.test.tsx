import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import { StrictMode } from "react";

const recordPageView = vi.fn();
vi.mock("@/lib/api", () => ({ api: { recordPageView: (...a: unknown[]) => recordPageView(...a) } }));

import { ViewBeacon } from "./ViewBeacon";

const nav = navigator as Navigator & { sendBeacon?: Navigator["sendBeacon"] };
const original = nav.sendBeacon;

beforeEach(() => {
  recordPageView.mockReset();
  recordPageView.mockResolvedValue(undefined);
});

afterEach(() => {
  if (original) {
    Object.defineProperty(nav, "sendBeacon", { value: original, configurable: true, writable: true });
  } else {
    delete (nav as { sendBeacon?: unknown }).sendBeacon;
  }
});

function setBeacon(fn: unknown) {
  Object.defineProperty(nav, "sendBeacon", { value: fn, configurable: true, writable: true });
}

describe("ViewBeacon", () => {
  it("sends one beacon per mount when sendBeacon is available", () => {
    const beacon = vi.fn(() => true);
    setBeacon(beacon);
    const { container, rerender } = render(
      <StrictMode>
        <ViewBeacon slug="jose" />
      </StrictMode>,
    );
    rerender(
      <StrictMode>
        <ViewBeacon slug="jose" />
      </StrictMode>,
    );
    expect(beacon).toHaveBeenCalledTimes(1);
    expect(beacon).toHaveBeenCalledWith("/api/public/pages/jose/view", JSON.stringify({ referrer: "" }));
    expect(recordPageView).not.toHaveBeenCalled();
    expect(container).toBeEmptyDOMElement();
  });

  it("falls back to api.recordPageView once when sendBeacon is missing", () => {
    setBeacon(undefined);
    render(<ViewBeacon slug="jose" />);
    expect(recordPageView).toHaveBeenCalledTimes(1);
    expect(recordPageView).toHaveBeenCalledWith("jose", "");
  });

  it("sends document.referrer in the beacon body and the fallback", () => {
    const spy = vi.spyOn(document, "referrer", "get").mockReturnValue("https://instagram.com/");
    const beacon = vi.fn(() => false);
    setBeacon(beacon);
    render(<ViewBeacon slug="jose" />);
    expect(beacon).toHaveBeenCalledWith("/api/public/pages/jose/view", '{"referrer":"https://instagram.com/"}');
    expect(recordPageView).toHaveBeenCalledWith("jose", "https://instagram.com/");
    spy.mockRestore();
  });

  it("falls back to api.recordPageView when sendBeacon refuses the request", () => {
    setBeacon(vi.fn(() => false));
    render(<ViewBeacon slug="jose" />);
    expect(recordPageView).toHaveBeenCalledTimes(1);
  });

  it("encodes the slug in the beacon URL", () => {
    const beacon = vi.fn(() => true);
    setBeacon(beacon);
    render(<ViewBeacon slug="a b" />);
    expect(beacon).toHaveBeenCalledWith("/api/public/pages/a%20b/view", JSON.stringify({ referrer: "" }));
  });

  it("swallows errors from sendBeacon and the API", async () => {
    setBeacon(
      vi.fn(() => {
        throw new Error("blocked");
      }),
    );
    recordPageView.mockRejectedValue(new Error("network"));
    const errors: unknown[] = [];
    const onErr = (e: PromiseRejectionEvent) => errors.push(e.reason);
    window.addEventListener("unhandledrejection", onErr);
    expect(() => render(<ViewBeacon slug="jose" />)).not.toThrow();
    await new Promise((r) => setTimeout(r, 0));
    window.removeEventListener("unhandledrejection", onErr);
    expect(recordPageView).toHaveBeenCalledTimes(1);
    expect(errors).toEqual([]);
  });

  it("swallows a synchronous throw from api.recordPageView", () => {
    setBeacon(undefined);
    recordPageView.mockImplementation(() => {
      throw new Error("boom");
    });
    expect(() => render(<ViewBeacon slug="jose" />)).not.toThrow();
  });
});
