import { cloneElement, type ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { TimeSeriesChart } from "./TimeSeriesChart";

// jsdom has no layout: give the chart a fixed size so recharts renders its legend.
vi.mock("recharts", async (importOriginal) => {
  const actual = await importOriginal<typeof import("recharts")>();
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: ReactElement<{ width?: number; height?: number }> }) =>
      cloneElement(children, { width: 600, height: 280 }),
  };
});

const pts = (clicks: number[]) => clicks.map((c, i) => ({ date: `2026-10-2${i}`, clicks: c }));

describe("TimeSeriesChart", () => {
  it("shows NoData for an empty series", () => {
    render(<TimeSeriesChart series={[]} unit="day" />);
    expect(screen.getByText("No data for this period.")).toBeInTheDocument();
  });

  it("renders one legend entry per multiSeries entry and ignores series", () => {
    const { container } = render(
      <TimeSeriesChart
        series={[]}
        unit="day"
        multiSeries={[
          { name: "Email", points: pts([1, 2, 3]) },
          { name: "Social", points: pts([0, 4, 1]) },
          { name: "Ads", points: pts([2, 2, 2]) },
        ]}
      />,
    );
    const legend = Array.from(container.querySelectorAll(".recharts-legend-item-text")).map((e) => e.textContent);
    expect(legend).toEqual(["Email", "Social", "Ads"]);
    expect(screen.queryByText("No data for this period.")).not.toBeInTheDocument();
  });

  it("shows NoData when every multiSeries entry is empty", () => {
    render(
      <TimeSeriesChart
        series={pts([1])}
        unit="day"
        multiSeries={[
          { name: "Email", points: [] },
          { name: "Social", points: [] },
        ]}
      />,
    );
    expect(screen.getByText("No data for this period.")).toBeInTheDocument();
  });
});
