import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { AnalyticsOverview } from "@/types";
import { PerformanceOverview } from "./PerformanceOverview";
import { TopPerforming } from "./TopPerforming";

vi.mock("@/components/charts/TimeSeriesChart", () => ({
  TimeSeriesChart: (p: { seriesName: string; prevSeriesName: string }) => (
    <div data-testid="chart">
      {p.seriesName}/{p.prevSeriesName}
    </div>
  ),
}));
vi.mock("@/components/common/Favicon", () => ({ Favicon: ({ url }: { url: string }) => <span data-testid="favicon" data-url={url} /> }));

const refetch = vi.fn();
const state: { data: AnalyticsOverview; isError: boolean } = { data: undefined as unknown as AnalyticsOverview, isError: false };
vi.mock("@/lib/hooks", () => ({
  useAnalyticsOverview: () => ({ data: state.data, isLoading: false, isError: state.isError, refetch }),
}));

function overview(over: Partial<AnalyticsOverview> = {}): AnalyticsOverview {
  const week = { from: "2026-10-19", to: "2026-10-25", total: 1240, series: [{ date: "2026-10-19", clicks: 3 }] };
  return { this_week: week, last_week: { ...week, total: 1000 }, change_pct: 24, top_performing: [], ...over };
}

beforeEach(() => {
  state.data = overview();
  state.isError = false;
  refetch.mockReset();
});

describe("PerformanceOverview", () => {

  it("shows this week's total, the change badge and both weeks charted", () => {
    render(<PerformanceOverview />);
    expect(screen.getByText("TOTAL ENGAGEMENTS · THIS WEEK")).toBeInTheDocument();
    expect(screen.getByText("1,240")).toBeInTheDocument();
    expect(screen.getByText("+24.0%")).toBeInTheDocument();
    expect(screen.getByText("vs last week")).toBeInTheDocument();
    expect(screen.getByTestId("chart")).toHaveTextContent("This week/Last week");
  });

  it("hides the badge when change_pct is null", () => {
    state.data = overview({ change_pct: null });
    render(<PerformanceOverview />);
    expect(screen.queryByText("vs last week")).not.toBeInTheDocument();
  });
});

describe("TopPerforming", () => {
  it("links QR items to the QR Code and link items to the link", () => {
    state.data = overview({
      top_performing: [
        { type: "qr", id: 5, link_id: 9, title: "Menu", short_url: "http://localhost:8080/abc", original_url: "https://menu.example/", clicks: 1500, percentage: 60, prev_clicks: 0 },
        { type: "link", id: 3, link_id: 3, title: "", short_url: "http://localhost:8080/xyz", original_url: "https://xyz.example/", clicks: 7, percentage: 40, prev_clicks: 0 },
      ],
    });
    render(<TopPerforming />);
    expect(screen.getByRole("link", { name: "Menu" })).toHaveAttribute("href", "/dashboard/qr-codes/5");
    expect(screen.getAllByRole("link", { name: "localhost:8080/xyz" })[0]).toHaveAttribute("href", "/dashboard/links/3");
    expect(screen.getByText("1,500 engagements")).toBeInTheDocument();
    expect(screen.getByText("7 engagements")).toBeInTheDocument();
    expect(screen.getAllByTestId("favicon").map((f) => f.getAttribute("data-url"))).toEqual([
      "https://menu.example/",
      "https://xyz.example/",
    ]);
  });

  it("shows the empty state without engagements", () => {
    state.data = overview();
    render(<TopPerforming />);
    expect(screen.getByText("No engagements in the last 30 days")).toBeInTheDocument();
    expect(screen.getByText("Share your links and QR Codes to see the best performers here.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Create a link" })).toHaveAttribute("href", "/dashboard/links/create");
  });
});

describe("overview error states", () => {
  it("show an error with Retry instead of a skeleton or a false empty state", async () => {
    state.data = undefined as unknown as AnalyticsOverview;
    state.isError = true;
    const user = userEvent.setup();
    render(
      <>
        <PerformanceOverview />
        <TopPerforming />
      </>,
    );
    expect(screen.queryByText("No engagements in the last 30 days")).not.toBeInTheDocument();
    expect(screen.queryByTestId("chart")).not.toBeInTheDocument();
    expect(screen.getByText("Couldn't load the performance overview.")).toBeInTheDocument();
    expect(screen.getByText("Couldn't load top performing links and QR Codes.")).toBeInTheDocument();
    const retries = screen.getAllByRole("button", { name: "Retry" });
    expect(retries).toHaveLength(2);
    await user.click(retries[0]);
    await user.click(retries[1]);
    expect(refetch).toHaveBeenCalledTimes(2);
  });
});
