import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { FiltersDialog, type LinkFilters } from "./FiltersDialog";

const active: LinkFilters = { tags: ["promo"], linkType: "custom", expiration: "expired", hasQr: "with" };

describe("FiltersDialog", () => {
  it("'Clear all filters' resets and Apply passes the defaults", () => {
    const onApply = vi.fn();
    render(<FiltersDialog open onOpenChange={vi.fn()} value={active} tags={["promo", "x"]} onApply={onApply} />);
    expect(screen.getByRole("heading", { name: "Filters" })).toBeInTheDocument();
    expect(screen.getByText("promo")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Clear all filters" }));
    expect(screen.queryByText("promo")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(onApply).toHaveBeenCalledWith({ tags: [], linkType: "all", expiration: null, hasQr: "any" });
  });

  it("Apply without changes passes the current value", () => {
    const onApply = vi.fn();
    render(<FiltersDialog open onOpenChange={vi.fn()} value={active} tags={[]} onApply={onApply} />);
    fireEvent.click(screen.getByRole("radio", { name: "No expiration" }));
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(onApply).toHaveBeenCalledWith({ ...active, expiration: "none" });
  });
});
