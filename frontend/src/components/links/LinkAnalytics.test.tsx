import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { api, ApiError } from "@/lib/api";
import type { LinkStatsQuery, LinkStatsV2 } from "@/types";
import { LinkAnalytics } from "./LinkAnalytics";

let search = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace, prefetch: vi.fn() }),
  usePathname: () => "/dashboard/links/7",
  useSearchParams: () => search,
}));

const fixture: LinkStatsV2 = {
  from: "2026-09-25",
  to: "2026-10-24",
  unit: "day",
  total_all_time: 1240,
  total_range: 120,
  prev_total_range: 99,
  change_pct: 21.2,
  series: [
    { date: "2026-10-23", clicks: 50 },
    { date: "2026-10-24", clicks: 70 },
  ],
  prev_series: [
    { date: "2026-08-26", clicks: 40 },
    { date: "2026-08-27", clicks: 59 },
  ],
  countries: [{ code: "US", name: "United States", clicks: 80, percentage: 66.7, prev_clicks: 60 }],
  cities: [{ name: "Austin", country: "US", clicks: 80, percentage: 66.7, prev_clicks: 60 }],
  referrers: [{ referrer: "google.com", clicks: 120, percentage: 100, prev_clicks: 100 }],
  devices: [{ key: "mobile", clicks: 70, percentage: 58.3, prev_clicks: 50 }],
  browsers: [{ key: "Chrome", clicks: 120, percentage: 100, prev_clicks: 100 }],
  os: [{ key: "iOS", clicks: 120, percentage: 100, prev_clicks: 100 }],
  compare_link: null,
};

const useLinkStats = vi.fn();
const replace = vi.fn();
vi.mock("@/lib/hooks", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/hooks")>()),
  useLinkStats: (id: number, query: LinkStatsQuery) => useLinkStats(id, query),
}));

function wrap(ui: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("LinkAnalytics", () => {
  beforeEach(() => {
    search = new URLSearchParams();
    useLinkStats.mockReset();
    replace.mockReset();
  });

  it("renders the controls and every module with defaults", () => {
    useLinkStats.mockReturnValue({ data: { ...fixture, change_pct: null }, isLoading: false });
    wrap(<LinkAnalytics linkId={7} destination="https://example.com/a/b" />);

    expect(screen.getByRole("heading", { name: "Link analytics - example.com" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Last 30 days/ })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Comparison" })).toHaveTextContent("No comparison");
    expect(screen.getByRole("combobox", { name: "Source" })).toHaveTextContent("All engagements");
    expect(screen.getByRole("button", { name: "Download CSV" })).toBeInTheDocument();

    for (const title of [
      "Engagements over time",
      "Locations",
      "Referrers",
      "Devices",
      "Browsers",
      "Operating systems",
    ]) {
      expect(screen.getByRole("heading", { name: title })).toBeInTheDocument();
    }
    expect(screen.getByText("Total engagements")).toBeInTheDocument();
    expect(screen.getByText("Engagements (Last 30 days)")).toBeInTheDocument();
    expect(screen.getByText("1,240")).toBeInTheDocument();
    expect(screen.queryByText("+21.2%")).not.toBeInTheDocument();

    const [id, query] = useLinkStats.mock.calls[0];
    expect(id).toBe(7);
    expect(query).toMatchObject({ compare: "none", source: "all", unit: "day" });
  });

  it("shows the ▲ change badge when comparing to the previous period", () => {
    search = new URLSearchParams("compare=previous");
    useLinkStats.mockReturnValue({ data: fixture, isLoading: false });
    wrap(<LinkAnalytics linkId={7} destination="https://example.com" />);

    expect(screen.getByRole("combobox", { name: "Comparison" })).toHaveTextContent("Previous period");
    const badge = screen.getByText("+21.2%");
    // The badge is "▲" (aria-hidden) followed by the signed percentage.
    expect(badge).toHaveTextContent("▲+21.2%");
    expect(useLinkStats.mock.calls[0][1]).toMatchObject({ compare: "previous" });
  });

  it("uses hourly buckets for a range of two days or less", () => {
    search = new URLSearchParams("from=2026-10-23&to=2026-10-24");
    useLinkStats.mockReturnValue({ data: fixture, isLoading: false });
    wrap(<LinkAnalytics linkId={7} destination="https://example.com" />);
    expect(useLinkStats.mock.calls[0][1]).toMatchObject({ from: "2026-10-23", to: "2026-10-24", unit: "hour" });
    expect(screen.queryByRole("button", { name: /Last 30 days/ })).not.toBeInTheDocument();
  });

  it("shows the empty message in each module when there is no data", () => {
    useLinkStats.mockReturnValue({
      data: { ...fixture, series: [], countries: [], cities: [], referrers: [], devices: [], browsers: [], os: [] },
      isLoading: false,
    });
    wrap(<LinkAnalytics linkId={7} destination="https://example.com" />);
    expect(screen.getAllByText("No data for this period.")).toHaveLength(6);
  });

  it("shows an error card with Retry when the stats fail to load", () => {
    const refetch = vi.fn();
    useLinkStats.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new ApiError("range too long", 400),
      refetch,
    });
    wrap(<LinkAnalytics linkId={7} destination="https://example.com" />);
    expect(screen.getByText("Couldn't load analytics")).toBeInTheDocument();
    expect(screen.getByText("range too long")).toBeInTheDocument();
    expect(screen.queryByText("No data for this period.")).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Locations" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("renders scan analytics for a QR code from the QR stats endpoint", async () => {
    const getQrStats = vi.spyOn(api, "getQrStats").mockResolvedValue(fixture);
    useLinkStats.mockReturnValue({ data: undefined, isLoading: false });
    wrap(<LinkAnalytics subject={{ kind: "qr", id: 5 }} destination="https://example.com/menu" />);

    expect(await screen.findByText("Total scans")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Scan data" })).toBeInTheDocument();
    expect(screen.getByText("Scans (Last 30 days)")).toBeInTheDocument();
    for (const title of ["Scans over time", "Scans by location", "Scans by browser", "Scans by operating system"]) {
      expect(screen.getByRole("heading", { name: title })).toBeInTheDocument();
    }
    expect(screen.queryByRole("heading", { name: "Referrers" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Devices" })).not.toBeInTheDocument();
    expect(screen.queryByText("Referrers")).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "Source" })).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Countries" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Cities" })).toBeInTheDocument();

    expect(getQrStats).toHaveBeenCalledTimes(1);
    const [id, query] = getQrStats.mock.calls[0];
    expect(id).toBe(5);
    expect(query).toMatchObject({ compare: "none", unit: "day" });
    expect(query).not.toHaveProperty("source");
    // The link stats query stays disabled (no id).
    for (const [linkId] of useLinkStats.mock.calls) expect(linkId).toBeNull();
    getQrStats.mockRestore();
  });

  it("still accepts the deprecated linkId prop and a link subject", () => {
    useLinkStats.mockReturnValue({ data: fixture, isLoading: false });
    wrap(<LinkAnalytics subject={{ kind: "link", id: 9 }} destination="https://example.com" />);
    expect(screen.getByRole("heading", { name: "Link analytics - example.com" })).toBeInTheDocument();
    expect(useLinkStats.mock.calls[0][0]).toBe(9);
  });

  it("compares with another link from ?compare_link and clears it from the chip", () => {
    search = new URLSearchParams("compare_link=12&source=qr");
    useLinkStats.mockReturnValue({
      data: {
        ...fixture,
        compare_link: {
          id: 12,
          title: "Spring sale",
          short_url: "https://sho.rt/spring",
          total_range: 4321,
          series: [
            { date: "2026-10-23", clicks: 3 },
            { date: "2026-10-24", clicks: 4 },
          ],
        },
      },
      isLoading: false,
    });
    wrap(<LinkAnalytics linkId={7} destination="https://example.com" />);

    expect(useLinkStats.mock.calls[0][1]).toMatchObject({ compare_link: 12 });
    expect(screen.queryByRole("button", { name: "Compare to a link" })).not.toBeInTheDocument();
    expect(screen.getByText("4,321")).toBeInTheDocument();
    expect(screen.getAllByText("Spring sale").length).toBeGreaterThanOrEqual(2);

    fireEvent.click(screen.getByRole("button", { name: "Remove comparison" }));
    expect(replace).toHaveBeenCalledWith("/dashboard/links/7?source=qr", { scroll: false });
  });

  it("offers Compare to a link and ignores an invalid compare_link", () => {
    search = new URLSearchParams("compare_link=abc");
    useLinkStats.mockReturnValue({ data: fixture, isLoading: false });
    wrap(<LinkAnalytics linkId={7} destination="https://example.com" />);
    expect(useLinkStats.mock.calls[0][1]).not.toHaveProperty("compare_link");
    expect(screen.getByRole("button", { name: "Compare to a link" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Remove comparison" })).not.toBeInTheDocument();
  });

  it("never offers Compare to a link on QR scan data", async () => {
    const getQrStats = vi.spyOn(api, "getQrStats").mockResolvedValue(fixture);
    search = new URLSearchParams("compare_link=12");
    useLinkStats.mockReturnValue({ data: undefined, isLoading: false });
    wrap(<LinkAnalytics subject={{ kind: "qr", id: 5 }} destination="https://example.com/menu" />);
    expect(await screen.findByText("Total scans")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Compare to a link" })).not.toBeInTheDocument();
    expect(getQrStats.mock.calls[0][1]).not.toHaveProperty("compare_link");
    getQrStats.mockRestore();
  });
});
