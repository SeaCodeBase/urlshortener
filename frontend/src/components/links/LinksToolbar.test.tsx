import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LinksToolbar, type LinksToolbarProps } from "./LinksToolbar";

function setup(overrides: Partial<LinksToolbarProps> = {}) {
  const props: LinksToolbarProps = {
    selectedCount: 0,
    allSelected: false,
    onToggleAll: vi.fn(),
    onExport: vi.fn(),
    onHide: vi.fn(),
    onTag: vi.fn(),
    show: "active",
    onShowChange: vi.fn(),
    view: "default",
    onViewChange: vi.fn(),
    ...overrides,
  };
  render(<LinksToolbar {...props} />);
  return props;
}

describe("LinksToolbar", () => {
  it("shows 0 selected and disables Hide/Tag without a selection", () => {
    setup();
    expect(screen.getByText("0 selected")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Export" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Hide" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Tag" })).toBeDisabled();
  });

  it("enables Hide/Tag with a selection and says Unhide in the Hidden view", () => {
    setup({ selectedCount: 2, show: "hidden" });
    expect(screen.getByText("2 selected")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Unhide" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Tag" })).toBeEnabled();
  });

  it("renders the Show select with Active and Hidden", async () => {
    const props = setup();
    const trigger = screen.getByRole("combobox", { name: "Show" });
    expect(trigger).toHaveTextContent("Show: Active");
    await userEvent.setup().click(trigger);
    const hidden = await screen.findByRole("option", { name: "Hidden" });
    await userEvent.setup().click(hidden);
    expect(props.onShowChange).toHaveBeenCalledWith("hidden");
  });

  it("renders the three view radios", async () => {
    const props = setup();
    const radios = screen.getAllByRole("radio").map((r) => r.getAttribute("aria-label"));
    expect(radios).toEqual(["Compact view", "Default view", "Grid view"]);
    expect(screen.getByRole("radio", { name: "Default view" })).toBeChecked();
    await userEvent.setup().click(screen.getByRole("radio", { name: "Grid view" }));
    expect(props.onViewChange).toHaveBeenCalledWith("grid");
  });
});

describe("LinksToolbar for QR Codes", () => {
  it("uses the codes noun and can hide the view toggle", () => {
    render(
      <LinksToolbar
        noun="codes"
        showViews={false}
        selectedCount={0}
        allSelected={false}
        onToggleAll={vi.fn()}
        onExport={vi.fn()}
        onHide={vi.fn()}
        onTag={vi.fn()}
        show="active"
        onShowChange={vi.fn()}
      />,
    );
    expect(screen.getByRole("checkbox", { name: "Select all codes" })).toBeInTheDocument();
    expect(screen.queryByRole("radiogroup", { name: "View" })).not.toBeInTheDocument();
  });
});
