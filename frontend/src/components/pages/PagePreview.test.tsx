import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import type { PageBlock, PublishedBlock } from "@/types";
import { DESIGN_PRESETS } from "./designPresets";
import { PagePreview } from "./PagePreview";

const linkBlock: PageBlock = {
  id: 1,
  type: "link",
  position: 0,
  content: { title: "Menu", description: "Our menu", thumbnail_url: "" },
  is_active: true,
  is_pinned: false,
  link_id: 9,
  link: { id: 9, title: "Menu", short_url: "http://localhost:8080/abc", original_url: "https://example.com/menu" },
};
const hiddenText: PageBlock = {
  id: 2,
  type: "text",
  position: 1,
  content: { heading: "Hidden heading", body: "" },
  is_active: false,
  is_pinned: false,
  link_id: null,
};
const social: PageBlock = {
  id: 3,
  type: "social",
  position: 2,
  content: {
    items: [
      { platform: "instagram", url: "https://instagram.com/jose" },
      { platform: "x", url: "https://x.com/jose" },
      { platform: "tiktok", url: "https://tiktok.com/@jose" },
      { platform: "email", url: "mailto:jose@example.com" },
    ],
  },
  is_active: true,
  is_pinned: false,
  link_id: null,
};

const page = {
  title: "Jose",
  description: "Hello there",
  avatar_url: "",
  design: DESIGN_PRESETS.bold,
};

describe("PagePreview", () => {
  it("renders the title and description", () => {
    render(<PagePreview mode="editor" page={{ ...page, blocks: [] }} />);
    expect(screen.getByRole("heading", { name: "Jose" })).toBeInTheDocument();
    expect(screen.getByText("Hello there")).toBeInTheDocument();
  });

  it("applies the design as CSS variables", () => {
    const { container } = render(<PagePreview mode="editor" page={{ ...page, blocks: [] }} />);
    const root = container.firstElementChild as HTMLElement;
    expect(root.style.getPropertyValue("--btn-bg")).toBe("#F59E0B");
    expect(root.style.getPropertyValue("--btn-text")).toBe("#111827");
    expect(root.style.getPropertyValue("--page-text")).toBe("#F9FAFB");
    expect(root.style.getPropertyValue("--btn-radius")).toBe("999px");
  });

  it("renders a gradient background", () => {
    const { container } = render(<PagePreview mode="editor" page={{ ...page, design: DESIGN_PRESETS.dark, blocks: [] }} />);
    const root = container.firstElementChild as HTMLElement;
    expect(root.style.getPropertyValue("--page-bg")).toBe("linear-gradient(180deg, #0F172A, #1E293B)");
  });

  it("links to short_url in public mode", () => {
    const blocks: PublishedBlock[] = [
      {
        id: 1,
        type: "link",
        content: { title: "Menu", description: "", thumbnail_url: "" },
        is_pinned: false,
        short_url: "http://localhost:8080/abc?r=page",
      },
    ];
    render(<PagePreview mode="public" page={{ ...page, blocks }} />);
    expect(screen.getByRole("link", { name: /Menu/ })).toHaveAttribute("href", "http://localhost:8080/abc?r=page");
  });

  it("uses # links and dims inactive blocks in editor mode", () => {
    render(<PagePreview mode="editor" page={{ ...page, blocks: [linkBlock, hiddenText] }} />);
    expect(screen.getByRole("link", { name: /Menu/ })).toHaveAttribute("href", "#");
    const hidden = screen.getByText("Hidden heading").closest("[data-block-id]") as HTMLElement;
    expect(hidden.style.opacity).toBe("0.5");
    const shown = screen.getByRole("link", { name: /Menu/ }).closest("[data-block-id]") as HTMLElement;
    expect(shown.style.opacity).toBe("");
  });

  it("renders one icon link per social platform", () => {
    render(<PagePreview mode="public" page={{ ...page, blocks: [social] }} />);
    expect(screen.getByRole("link", { name: "Instagram" })).toHaveAttribute("href", "https://instagram.com/jose");
    expect(screen.getByRole("link", { name: "X" })).toHaveAttribute("href", "https://x.com/jose");
    expect(screen.getByRole("link", { name: "TikTok" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Email" })).toHaveAttribute("href", "mailto:jose@example.com");
    expect(document.querySelectorAll("svg").length).toBeGreaterThanOrEqual(4);
  });

  it("renders text and image blocks", () => {
    const blocks: PublishedBlock[] = [
      { id: 4, type: "text", content: { heading: "Welcome", body: "Glad you're here" }, is_pinned: false },
      { id: 5, type: "image", content: { url: "/uploads/a.png", alt: "Cover" }, is_pinned: false },
    ];
    render(<PagePreview mode="public" page={{ ...page, blocks }} />);
    expect(screen.getByText("Welcome")).toBeInTheDocument();
    expect(screen.getByText("Glad you're here")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Cover" })).toHaveAttribute("src", "/uploads/a.png");
  });
});
