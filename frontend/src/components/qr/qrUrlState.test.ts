import { describe, expect, it } from "vitest";
import {
  hasActiveQrQuery,
  parseQrUrlState,
  qrUrlStateToSearch,
  toQrApiParams,
  type QrUrlState,
} from "./qrUrlState";

const state: QrUrlState = {
  q: "a",
  show: "hidden",
  filters: { tags: ["x"], expiration: "expiring", hasLink: "with" },
  from: null,
  to: null,
};

describe("qrUrlState", () => {
  it("round-trips through the URL", () => {
    const qs = qrUrlStateToSearch(state);
    expect(parseQrUrlState(new URLSearchParams(qs))).toEqual(state);
  });

  it("maps to API params", () => {
    expect(toQrApiParams(state)).toEqual({
      archived: "on",
      has_link: "on",
      expiration: "expiring",
      tags: ["x"],
      q: "a",
    });
  });

  it("uses defaults for an empty URL and omits them when serialising", () => {
    const empty = parseQrUrlState(new URLSearchParams());
    expect(empty).toEqual({
      q: "",
      show: "active",
      filters: { tags: [], expiration: null, hasLink: "any" },
      from: null,
      to: null,
    });
    expect(qrUrlStateToSearch(empty)).toBe("");
    expect(toQrApiParams(empty)).toEqual({ archived: "off", has_link: "both" });
    expect(hasActiveQrQuery(empty)).toBe(false);
  });

  it("maps without → off and keeps created dates", () => {
    const s = parseQrUrlState(new URLSearchParams("link=without&from=2026-10-01&to=2026-10-20"));
    expect(toQrApiParams(s)).toEqual({
      archived: "off",
      has_link: "off",
      created_after: "2026-10-01",
      created_before: "2026-10-20",
    });
    expect(hasActiveQrQuery(s)).toBe(true);
  });
});
