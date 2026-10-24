import { describe, expect, it } from "vitest";
import type { DashboardData } from "@/types";
import { moduleCsv, slugify } from "./dashboardCsv";

function data(over: Partial<DashboardData> = {}): DashboardData {
  return {
    from: "2026-10-18",
    to: "2026-10-24",
    unit: "day",
    compare: true,
    total: 120,
    prev_total: 100,
    change_pct: 20,
    top_day: { date: "2026-10-20", count: 40, prev_count: 12, change_pct: 233.3 },
    top_location: { code: "US", name: "United States", count: 55 },
    series: [
      { date: "2026-10-18", clicks: 3 },
      { date: "2026-10-19", clicks: 5 },
    ],
    prev_series: [
      { date: "2026-10-11", clicks: 1 },
      { date: "2026-10-12", clicks: 2 },
    ],
    devices: [
      { key: "mobile", clicks: 70, percentage: 58.33, prev_clicks: 50 },
      { key: "desktop", clicks: 50, percentage: 41.67, prev_clicks: 50 },
    ],
    referrers: [{ referrer: "", clicks: 90, percentage: 75, prev_clicks: 0 }],
    countries: [{ code: "US", name: "United States", clicks: 55, percentage: 45.8, prev_clicks: 40 }],
    cities: [{ name: "Austin", country: "US", clicks: 20, percentage: 16.7, prev_clicks: 2 }],
    items: [
      { type: "qr", id: 5, link_id: 9, title: "Menu", short_url: "http://localhost:8080/abc", clicks: 60, percentage: 50, prev_clicks: 30 },
      { type: "link", id: 9, link_id: 9, title: "=cmd", short_url: "http://localhost:8080/xyz", clicks: 60, percentage: 50, prev_clicks: 0 },
    ],
    ...over,
  } as DashboardData;
}

const lines = (csv: string) => csv.trimEnd().split("\n");

describe("moduleCsv", () => {
  it("devices: Device,Engagements,% and one row per device", () => {
    expect(lines(moduleCsv("devices", data(), false))).toEqual(["Device,Engagements,%", "Mobile,70,58.3", "Desktop,50,41.7"]);
  });

  it("devices with comparison adds a Previous column", () => {
    expect(lines(moduleCsv("devices", data(), true))).toEqual([
      "Device,Engagements,%,Previous",
      "Mobile,70,58.3,50",
      "Desktop,50,41.7,50",
    ]);
  });

  it("items: Type and Link columns, every row, formula-safe cells", () => {
    expect(lines(moduleCsv("items", data(), false))).toEqual([
      "Type,Title,Link,Engagements,%",
      "QR Code,Menu,http://localhost:8080/abc,60,50",
      "Link,'=cmd,http://localhost:8080/xyz,60,50",
    ]);
  });

  it("over_time pairs the previous series by index when comparing", () => {
    expect(lines(moduleCsv("over_time", data(), true))).toEqual([
      "Date,Engagements,Previous",
      "2026-10-18,3,1",
      "2026-10-19,5,2",
    ]);
  });

  it("referrers name the empty referrer Direct", () => {
    expect(lines(moduleCsv("referrers", data(), false))).toEqual(["Referrer,Engagements,%", "Direct,90,75"]);
  });

  it("scorecards: header only when there is no data", () => {
    expect(lines(moduleCsv("top_day", data({ top_day: null }), false))).toEqual(["Date,Engagements"]);
    expect(lines(moduleCsv("top_location", data(), false))).toEqual(["Country,Code,Engagements", "United States,US,55"]);
  });
});

describe("slugify", () => {
  it("lowercases and dashes the name", () => {
    expect(slugify("  Q4 Launch: Links & QR! ")).toBe("q4-launch-links-qr");
    expect(slugify("???")).toBe("dashboard");
  });
});
