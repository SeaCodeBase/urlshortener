import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { countActiveFilters, DEFAULT_LINK_FILTERS, FiltersDialog, type LinkFilters } from "./FiltersDialog";

vi.mock("@/lib/hooks", () => ({
  useCampaigns: () => ({
    data: [
      { id: 3, name: "Spring sale" },
      { id: 4, name: "Launch" },
    ],
    isLoading: false,
  }),
}));

const active: LinkFilters = { tags: ["promo"], linkType: "custom", expiration: "expired", hasQr: "with", campaignId: "" };

describe("FiltersDialog", () => {
  it("'Clear all filters' resets and Apply passes the defaults", () => {
    const onApply = vi.fn();
    render(<FiltersDialog open onOpenChange={vi.fn()} value={active} tags={["promo", "x"]} onApply={onApply} />);
    expect(screen.getByRole("heading", { name: "Filters" })).toBeInTheDocument();
    expect(screen.getByText("promo")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Clear all filters" }));
    expect(screen.queryByText("promo")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(onApply).toHaveBeenCalledWith({ tags: [], linkType: "all", expiration: null, hasQr: "any", campaignId: "" });
  });

  it("Apply without changes passes the current value", () => {
    const onApply = vi.fn();
    render(<FiltersDialog open onOpenChange={vi.fn()} value={active} tags={[]} onApply={onApply} />);
    fireEvent.click(screen.getByRole("radio", { name: "No expiration" }));
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(onApply).toHaveBeenCalledWith({ ...active, expiration: "none" });
  });

  it("applies the chosen campaign", async () => {
    const onApply = vi.fn();
    const user = userEvent.setup();
    render(<FiltersDialog open onOpenChange={vi.fn()} value={DEFAULT_LINK_FILTERS} tags={[]} onApply={onApply} />);
    const select = screen.getByRole("combobox", { name: "Campaign" });
    expect(select).toHaveTextContent("All campaigns");
    await user.click(select);
    await user.click(await screen.findByRole("option", { name: "Launch" }));
    await user.click(screen.getByRole("button", { name: "Apply" }));
    expect(onApply).toHaveBeenCalledWith({ ...DEFAULT_LINK_FILTERS, campaignId: "4" });
  });

  it("'All campaigns' clears the campaign", async () => {
    const onApply = vi.fn();
    const user = userEvent.setup();
    render(
      <FiltersDialog open onOpenChange={vi.fn()} value={{ ...DEFAULT_LINK_FILTERS, campaignId: "3" }} tags={[]} onApply={onApply} />,
    );
    const select = screen.getByRole("combobox", { name: "Campaign" });
    expect(select).toHaveTextContent("Spring sale");
    await user.click(select);
    await user.click(await screen.findByRole("option", { name: "All campaigns" }));
    await user.click(screen.getByRole("button", { name: "Apply" }));
    expect(onApply).toHaveBeenCalledWith({ ...DEFAULT_LINK_FILTERS, campaignId: "" });
  });
});

describe("countActiveFilters", () => {
  it("counts a chosen campaign", () => {
    expect(countActiveFilters(DEFAULT_LINK_FILTERS)).toBe(0);
    expect(countActiveFilters({ ...DEFAULT_LINK_FILTERS, campaignId: "all" })).toBe(0);
    expect(countActiveFilters({ ...DEFAULT_LINK_FILTERS, campaignId: "4" })).toBe(1);
    expect(countActiveFilters({ ...active, campaignId: "4" })).toBe(5);
  });
});

describe("FiltersDialog — unknown campaign", () => {
  it("shows a 'Campaign #id' fallback when the selected campaign isn't loaded", () => {
    render(
      <FiltersDialog open onOpenChange={vi.fn()} value={{ ...DEFAULT_LINK_FILTERS, campaignId: "99" }} tags={[]} onApply={vi.fn()} />,
    );
    expect(screen.getByRole("combobox", { name: "Campaign" })).toHaveTextContent("Campaign #99");
  });

  it("keeps the unknown campaign on Apply", () => {
    const onApply = vi.fn();
    render(
      <FiltersDialog open onOpenChange={vi.fn()} value={{ ...DEFAULT_LINK_FILTERS, campaignId: "99" }} tags={[]} onApply={onApply} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(onApply).toHaveBeenCalledWith({ ...DEFAULT_LINK_FILTERS, campaignId: "99" });
  });
});
