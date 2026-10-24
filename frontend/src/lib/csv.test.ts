import { describe, expect, it } from "vitest";
import type { LinkStatsV2, PageStats } from "@/types";
import { buildPageStatsCsv, buildStatsCsv, csvCell } from "./csv";

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
  compare_link: null,
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

  it("uses the scans noun and leaves out Referrers and Devices for QR stats", () => {
    const lines = buildStatsCsv(stats, { rangeLabel: "Last 30 days", noun: "Scans", scansOnly: true }).split("\n");
    expect(lines).toContain("Scans over time");
    expect(lines).toContain("Date,Scans");
    expect(lines.some((l) => l.startsWith("Total scans,"))).toBe(true);
    expect(lines).not.toContain("Referrers");
    expect(lines).not.toContain("Devices");
    expect(lines).toContain("Browsers");
  });

  it("adds a Compared link section when stats compare with another link", () => {
    const lines = buildStatsCsv(
      {
        ...stats,
        compare_link: {
          id: 2,
          title: "=Spring",
          short_url: "https://sho.rt/s",
          total_range: 7,
          series: [{ date: "2026-10-24", clicks: 7 }],
        },
      },
      { rangeLabel: "Last 30 days" },
    ).split("\n");
    const i = lines.indexOf("Compared link");
    expect(i).toBeGreaterThan(0);
    expect(lines[i + 1]).toBe("Date,Engagements");
    expect(lines[i + 2]).toBe("2026-10-24,7");
    expect(buildStatsCsv(stats, { rangeLabel: "Last 30 days" })).not.toContain("Compared link");
  });
});


describe("buildPageStatsCsv", () => {
  const page: PageStats = {
    from: "2026-10-01",
    to: "2026-10-24",
    unit: "day",
    total_views_all_time: 40,
    total_views: 9,
    prev_total_views: 0,
    change_pct: null,
    series: [{ date: "2026-10-24", clicks: 9 }],
    prev_series: [],
    referrers: [{ referrer: "", clicks: 9, percentage: 100, prev_clicks: 0 }],
    devices: [{ key: "mobile", clicks: 9, percentage: 100, prev_clicks: 0 }],
    countries: [{ code: "US", name: "United States", clicks: 9, percentage: 100, prev_clicks: 0 }],
    cities: [{ name: "Austin", country: "US", clicks: 9, percentage: 100, prev_clicks: 0 }],
    blocks: [{ block_id: 5, title: "=Menu", link_id: 9, short_url: "http://sho.rt/abc", clicks: 4 }],
  };

  it("writes the page sections in order with escaped values", () => {
    const lines = buildPageStatsCsv(page, { rangeLabel: "Last 30 days" }).split("\n");
    const titles = ["Summary", "Views over time", "Link clicks", "Referrers", "Devices", "Countries", "Cities"];
    const idx = titles.map((t) => lines.indexOf(t));
    expect(idx.every((i) => i >= 0)).toBe(true);
    expect([...idx].sort((a, b) => a - b)).toEqual(idx);
    expect(lines[0]).toBe("Summary");
    expect(lines).toContain("Range,2026-10-01 to 2026-10-24");
    expect(lines).toContain("Total views,40");
    expect(lines).toContain("Views (Last 30 days),9");
    expect(lines).toContain("Date,Views");
    expect(lines).toContain("2026-10-24,9");
    expect(lines).toContain("Block,Short URL,Clicks");
    expect(lines).toContain("'=Menu,http://sho.rt/abc,4");
    expect(lines).toContain("Direct,9,100");
    expect(lines).toContain("Mobile,9,100");
    expect(lines).toContain("United States,US,9,100");
    expect(lines).toContain("Austin,US,9,100");
  });
});
