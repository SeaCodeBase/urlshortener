import { describe, expect, it } from "vitest";
import { DATE_RANGE_OPTIONS, DEFAULT_LAYOUT, MODULES, rangeFor } from "./modules";

describe("rangeFor", () => {
  const now = new Date("2026-10-24T15:30:00Z");

  it("last_7_days ends today and spans 7 days", () => {
    expect(rangeFor("last_7_days", now)).toEqual({ from: "2026-10-18", to: "2026-10-24" });
  });

  it("today is a single day", () => {
    expect(rangeFor("today", now)).toEqual({ from: "2026-10-24", to: "2026-10-24" });
  });

  it("last_30_days, last_60_days and last_90_days", () => {
    expect(rangeFor("last_30_days", now)).toEqual({ from: "2026-09-25", to: "2026-10-24" });
    expect(rangeFor("last_60_days", now)).toEqual({ from: "2026-08-26", to: "2026-10-24" });
    expect(rangeFor("last_90_days", now)).toEqual({ from: "2026-07-27", to: "2026-10-24" });
  });
});

describe("module registry", () => {
  it("DEFAULT_LAYOUT has the seven keys once each, in the default order", () => {
    const keys = DEFAULT_LAYOUT.modules.map((m) => m.key);
    expect(new Set(keys).size).toBe(7);
    expect(keys).toEqual(["top_day", "top_location", "over_time", "devices", "referrers", "locations", "items"]);
    for (const m of DEFAULT_LAYOUT.modules) expect(MODULES[m.key].allowed).toContain(m.viz);
  });

  it("allowed visualizations follow the contract", () => {
    expect(MODULES.devices.allowed).toEqual(["doughnut", "bar", "table"]);
    expect(MODULES.over_time.allowed).toEqual(["line", "bar", "table"]);
    expect(MODULES.referrers.allowed).toEqual(["bar", "doughnut", "table"]);
    expect(MODULES.top_day.allowed).toEqual(["scorecard"]);
  });

  it("module titles match the spec copy", () => {
    expect(MODULES.over_time.title).toBe("Engagements over time");
    expect(MODULES.items.title).toBe("Links & QR Codes by engagements");
  });

  it("date range options", () => {
    expect(DATE_RANGE_OPTIONS.map((o) => o.label)).toEqual([
      "Today",
      "Last 7 days",
      "Last 30 days",
      "Last 60 days",
      "Last 90 days",
    ]);
  });
});
