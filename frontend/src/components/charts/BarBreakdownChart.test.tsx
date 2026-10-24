import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { CategoryTick, truncateLabel } from "./BarBreakdownChart";

describe("truncateLabel", () => {
  it("keeps labels of up to 24 characters", () => {
    expect(truncateLabel("Direct")).toBe("Direct");
    expect(truncateLabel("a".repeat(24))).toBe("a".repeat(24));
  });

  it("cuts longer labels to 24 characters with an ellipsis", () => {
    const out = truncateLabel("https://news.ycombinator.com/item?id=123");
    expect(out).toBe("https://news.ycombinato…");
    expect(out).toHaveLength(24);
  });
});

describe("CategoryTick", () => {
  it("draws the truncated label with the full label as its title", () => {
    const full = "https://news.ycombinator.com/item?id=123";
    const { container } = render(
      <svg>
        <CategoryTick x={10} y={20} payload={{ value: full }} />
      </svg>,
    );
    const text = container.querySelector("text")!;
    expect(text.querySelector("title")?.textContent).toBe(full);
    expect(text.textContent).toContain("https://news.ycombinato…");
  });
});
