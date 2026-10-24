import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import type { PublishedPage } from "@/types";
import { DESIGN_PRESETS } from "./designPresets";
import { PublicPageView } from "./PublicPageView";

const page: PublishedPage = {
  slug: "jose",
  title: "Jose's links",
  description: "Hello there",
  avatar_url: "",
  design: { ...DESIGN_PRESETS.classic, background: { type: "solid", color: "#112233", color2: "#FFFFFF" } },
  published_at: "2026-10-25T10:00:00Z",
  blocks: [
    { id: 1, type: "link", content: { title: "Menu", description: "", thumbnail_url: "" }, is_pinned: true, short_url: "http://localhost:8080/abc?r=page" },
    { id: 2, type: "text", content: { heading: "About", body: "I cook" }, is_pinned: false },
    { id: 3, type: "link", content: { title: "Book a table", description: "", thumbnail_url: "" }, is_pinned: false, short_url: "http://localhost:8080/xyz?r=page" },
  ],
};

describe("PublicPageView", () => {
  it("renders the title, description and blocks", () => {
    render(<PublicPageView page={page} />);
    expect(screen.getByRole("heading", { name: "Jose's links" })).toBeInTheDocument();
    expect(screen.getByText("Hello there")).toBeInTheDocument();
    expect(screen.getByText("About")).toBeInTheDocument();
  });

  it("renders link buttons to short_url in the same tab", () => {
    render(<PublicPageView page={page} />);
    const menu = screen.getByRole("link", { name: /Menu/ });
    expect(menu).toHaveAttribute("href", "http://localhost:8080/abc?r=page");
    expect(menu).not.toHaveAttribute("target");
    expect(screen.getByRole("link", { name: /Book a table/ })).toHaveAttribute("href", "http://localhost:8080/xyz?r=page");
  });

  it("paints the full-viewport canvas with the design background", () => {
    const { container } = render(<PublicPageView page={page} />);
    const canvas = container.firstElementChild as HTMLElement;
    expect(canvas).toHaveClass("min-h-screen");
    expect(canvas.style.background).toContain("rgb(17, 34, 51)");
  });

  it("uses a gradient background when the design asks for one", () => {
    const { container } = render(
      <PublicPageView page={{ ...page, design: { ...page.design, background: { type: "gradient", color: "#112233", color2: "#445566" } } }} />,
    );
    expect((container.firstElementChild as HTMLElement).style.background).toContain("linear-gradient");
  });

  it("shows the footer linking home", () => {
    render(<PublicPageView page={page} />);
    expect(screen.getByRole("link", { name: "Made with URL Shortener" })).toHaveAttribute("href", "/");
  });
});
