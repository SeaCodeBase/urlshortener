import { describe, expect, it } from "vitest";
import type { LinkStatsV2 } from "@/types";
import { buildStatsCsv, csvCell } from "./csv";

const stats: LinkStatsV2 = {
  from: "2026-10-01",
  to: "2026-10-24",
  unit: "day",
  total_all_time: 10,
  total_range: 5,
  prev_total_range: 0,
  change_pct: null,
  series: [{ date: "2026-10-24", clicks: 5 }],
  prev_series: [],
  countries: [],
  cities: [{ name: "Washington, D.C.", country: "US", clicks: 1, percentage: 20, prev_clicks: 0 }],
  referrers: [{ referrer: "=1+1", clicks: 5, percentage: 100, prev_clicks: 0 }],
  devices: [],
  browsers: [{ key: "@SUM(A1)", clicks: 5, percentage: 100, prev_clicks: 0 }],
  os: [],
};

describe("csvCell", () => {
  it("neutralises formula-like strings", () => {
    for (const s of ["=1+1", "+1", "-1", "@x", "\tx", "\rx"]) expect(csvCell(s).replace(/^"/, "")).toMatch(/^'/);
    expect(csvCell("=1+1")).toBe("'=1+1");
  });
  it("quotes cells with commas or quotes", () => {
    expect(csvCell("a,b")).toBe('"a,b"');
    expect(csvCell('say "hi"')).toBe('"say ""hi"""');
  });
  it("keeps numbers numeric, negatives included", () => {
    expect(csvCell(-5)).toBe("-5");
    expect(csvCell(12.5)).toBe("12.5");
  });
});

describe("buildStatsCsv", () => {
  it("writes sections with escaped attacker-controlled values", () => {
    const csv = buildStatsCsv(stats, { rangeLabel: "Last 30 days" });
    const lines = csv.split("\n");
    expect(lines[0]).toBe("Summary");
    expect(lines).toContain("Engagements (Last 30 days),5");
    expect(lines).toContain("'=1+1,5,100");
    expect(lines).toContain("'@SUM(A1),5,100");
    expect(lines).toContain('"Washington, D.C.",US,1,20');
    expect(lines).toContain("Referrers");
  });
});
