import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { DoughnutChart } from "./DoughnutChart";

describe("DoughnutChart", () => {
  it("renders a legend row per slice with value and percentage", () => {
    render(
      <DoughnutChart
        rows={[
          { label: "Mobile", value: 70 },
          { label: "Desktop", value: 30 },
        ]}
      />,
    );
    const legend = screen.getByRole("list", { name: "Legend" });
    const items = within(legend).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByText("Mobile")).toBeInTheDocument();
    expect(within(items[0]).getByText("70")).toBeInTheDocument();
    expect(within(items[0]).getByText("70%")).toBeInTheDocument();
    expect(within(items[1]).getByText("Desktop")).toBeInTheDocument();
    expect(within(items[1]).getByText("30%")).toBeInTheDocument();
  });

  it("shows NoData for no rows or an all-zero total", () => {
    const { rerender } = render(<DoughnutChart rows={[]} />);
    expect(screen.getByText("No data for this period.")).toBeInTheDocument();
    rerender(<DoughnutChart rows={[{ label: "Mobile", value: 0 }]} />);
    expect(screen.getByText("No data for this period.")).toBeInTheDocument();
  });
});
