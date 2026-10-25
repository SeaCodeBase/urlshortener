import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { StatusChip } from "./StatusChip";

describe("StatusChip", () => {
  it.each([
    ["success", "bg-success/15 text-success"],
    ["warning", "bg-amber-500/15 text-amber-700 dark:text-amber-400"],
    ["info", "bg-primary/10 text-primary"],
    ["muted", "bg-muted text-muted-foreground"],
    ["destructive", "bg-destructive/10 text-destructive"],
  ] as const)("renders the %s tone classes", (tone, classes) => {
    render(<StatusChip tone={tone}>Label</StatusChip>);
    const chip = screen.getByText("Label");
    for (const c of classes.split(" ")) expect(chip).toHaveClass(c);
  });

  it("merges className and forwards title", () => {
    render(
      <StatusChip tone="success" className="w-12 font-mono" title="why">
        200
      </StatusChip>,
    );
    const chip = screen.getByText("200");
    expect(chip).toHaveClass("w-12", "font-mono", "bg-success/15");
    expect(chip).toHaveAttribute("title", "why");
  });
});
