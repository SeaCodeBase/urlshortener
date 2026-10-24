import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { TagPicker } from "./TagPicker";

describe("TagPicker", () => {
  it("creates a new tag on Enter and calls onChange with it", () => {
    const onChange = vi.fn();
    render(<TagPicker value={[]} onChange={onChange} suggestions={["marketing", "sales"]} />);
    fireEvent.click(screen.getByRole("button", { name: "Select tags" }));
    const input = screen.getByPlaceholderText("Search or create a tag");
    fireEvent.change(input, { target: { value: "  launch  " } });
    expect(screen.getByText("Create 'launch'")).toBeInTheDocument();
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith(["launch"]);
  });

  it("does not offer to create a tag that is already selected", () => {
    render(<TagPicker value={["launch"]} onChange={vi.fn()} suggestions={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "Select tags" }));
    fireEvent.change(screen.getByPlaceholderText("Search or create a tag"), { target: { value: "launch" } });
    expect(screen.queryByText("Create 'launch'")).not.toBeInTheDocument();
  });

  it("removes a chip with its ✕ button", () => {
    const onChange = vi.fn();
    render(<TagPicker value={["a", "b"]} onChange={onChange} suggestions={[]} />);
    fireEvent.click(screen.getByRole("button", { name: "Remove tag a" }));
    expect(onChange).toHaveBeenCalledWith(["b"]);
  });
});
