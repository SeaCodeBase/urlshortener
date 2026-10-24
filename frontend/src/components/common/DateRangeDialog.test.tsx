import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { DateRangeDialog } from "./DateRangeDialog";

describe("DateRangeDialog", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 9, 24, 15, 30));
  });
  afterEach(() => vi.useRealTimers());

  it("preset 'Last 7 days' selects today-6 → today and Apply passes both dates", () => {
    const onApply = vi.fn();
    render(<DateRangeDialog open onOpenChange={vi.fn()} value={null} onApply={onApply} />);
    expect(screen.getByText("Filter by created date")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Last 7 days" }));
    expect(screen.getByLabelText("Start date")).toHaveValue("10/18/2026");
    expect(screen.getByLabelText("End date")).toHaveValue("10/24/2026");
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(onApply).toHaveBeenCalledTimes(1);
    const { from, to } = onApply.mock.calls[0][0];
    expect(from).toEqual(new Date(2026, 9, 18, 0, 0, 0, 0));
    expect(to.getFullYear()).toBe(2026);
    expect(to.getMonth()).toBe(9);
    expect(to.getDate()).toBe(24);
  });

  it("Apply is disabled until a range is chosen", () => {
    render(<DateRangeDialog open onOpenChange={vi.fn()} value={null} onApply={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Apply" })).toBeDisabled();
  });

  it("maxDays disables Apply and explains the limit for longer ranges", () => {
    render(<DateRangeDialog open onOpenChange={vi.fn()} value={null} onApply={vi.fn()} maxDays={366} />);
    fireEvent.change(screen.getByLabelText("Start date"), { target: { value: "01/01/2025" } });
    fireEvent.change(screen.getByLabelText("End date"), { target: { value: "10/24/2026" } });
    expect(screen.getByText("Select up to 366 days")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Apply" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Start date"), { target: { value: "10/24/2025" } });
    expect(screen.queryByText("Select up to 366 days")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Apply" })).toBeEnabled();
  });
});
