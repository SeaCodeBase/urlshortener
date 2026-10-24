import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Link, Page } from "@/types";
import { AddToPageDialog } from "./AddToPageDialog";

const addBlock = vi.fn();
const toastSuccess = vi.fn();
const toastError = vi.fn();
const state: { pages: Page[]; filters: unknown } = { pages: [], filters: undefined };

vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: (m: string) => toastError(m) } }));
vi.mock("@/lib/hooks", () => ({
  usePages: (filters: unknown) => {
    state.filters = filters;
    return { data: { pages: state.pages, total: state.pages.length }, isLoading: false };
  },
  usePageMutations: () => ({ addBlock: { mutateAsync: addBlock, isPending: false } }),
}));

function page(id: number, title: string, slug: string): Page {
  return {
    id,
    group_id: 1,
    slug,
    title,
    description: "",
    avatar_url: "",
    design: {} as Page["design"],
    status: "draft",
    published_at: null,
    has_unpublished_changes: true,
    archived: false,
    url: `http://localhost:8080/m/${slug}`,
    views: 0,
    created_at: "",
    updated_at: "",
  };
}

const link = {
  id: 9,
  title: "",
  original_url: "https://www.example.com/menu?x=1",
  short_url: "http://localhost:8080/abc",
} as Link;

describe("AddToPageDialog", () => {
  beforeEach(() => {
    addBlock.mockReset().mockResolvedValue({ id: 1 });
    toastSuccess.mockReset();
    toastError.mockReset();
    state.pages = [page(1, "Jose", "jose"), page(2, "", "shop")];
  });

  it("submits addBlock with the chosen page and title, toasts and closes", async () => {
    const onOpenChange = vi.fn();
    const onAdded = vi.fn();
    const user = userEvent.setup();
    render(<AddToPageDialog open onOpenChange={onOpenChange} link={link} onAdded={onAdded} />);
    expect(state.filters).toEqual({ archived: "off" });
    expect(screen.getByRole("heading", { name: "Add to a page" })).toBeInTheDocument();
    expect(screen.getByText("Make sure you publish your page's updates after adding this link")).toBeInTheDocument();
    const title = screen.getByLabelText("Button title");
    expect(title).toHaveValue("www.example.com");

    await user.click(screen.getByRole("combobox", { name: "Page" }));
    await user.click(await screen.findByRole("option", { name: "shop" }));
    await user.clear(title);
    await user.type(title, "Our menu");
    await user.click(screen.getByRole("button", { name: "Add to page" }));

    expect(addBlock).toHaveBeenCalledWith({
      pageId: 2,
      input: { type: "link", link_id: 9, content: { title: "Our menu", description: "", thumbnail_url: "" } },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Added to shop");
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(onAdded).toHaveBeenCalledWith(expect.objectContaining({ id: 2 }));
  });

  it("prefills the button title with the link title", () => {
    render(<AddToPageDialog open onOpenChange={vi.fn()} link={{ ...link, title: "Menu" }} />);
    expect(screen.getByLabelText("Button title")).toHaveValue("Menu");
  });

  it("requires a title", async () => {
    const user = userEvent.setup();
    render(<AddToPageDialog open onOpenChange={vi.fn()} link={link} />);
    await user.clear(screen.getByLabelText("Button title"));
    expect(screen.getByRole("button", { name: "Add to page" })).toBeDisabled();
  });

  it("stays open and toasts the API error", async () => {
    addBlock.mockRejectedValue(new Error("Link not found"));
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<AddToPageDialog open onOpenChange={onOpenChange} link={link} />);
    await user.click(screen.getByRole("button", { name: "Add to page" }));
    expect(addBlock).toHaveBeenCalledWith({
      pageId: 1,
      input: { type: "link", link_id: 9, content: { title: "www.example.com", description: "", thumbnail_url: "" } },
    });
    expect(toastError).toHaveBeenCalledWith("Link not found");
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("shows the empty state without a submit button", () => {
    state.pages = [];
    render(<AddToPageDialog open onOpenChange={vi.fn()} link={link} />);
    expect(screen.getByText("You don't have any pages yet")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Create a page" })).toHaveAttribute("href", "/dashboard/pages/new");
    expect(screen.queryByRole("button", { name: "Add to page" })).not.toBeInTheDocument();
  });
});
