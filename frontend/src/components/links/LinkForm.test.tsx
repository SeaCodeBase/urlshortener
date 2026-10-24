import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { Page } from "@/types";
import { LinkForm } from "./LinkForm";

const push = vi.fn();
const createLink = vi.fn();
const addBlock = vi.fn();
const toastSuccess = vi.fn();
const toastError = vi.fn();
const state: { pages: Page[] } = { pages: [] };

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
}));
vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: (m: string) => toastError(m) } }));
vi.mock("@/lib/hooks", () => ({
  useActiveGroupId: () => 7,
  useCreateLink: () => ({ mutateAsync: createLink, isPending: false }),
  useCreateQrCode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUploadImage: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDomains: () => ({ data: [] }),
  useTags: () => ({ data: [] }),
  useMeta: () => ({ data: { base_url: "http://localhost:8080" } }),
  useGroups: () => ({ data: [{ id: 7, name: "g", default_domain_id: null, created_at: "", updated_at: "" }] }),
  usePages: () => ({ data: { pages: state.pages, total: state.pages.length } }),
  usePageMutations: () => ({ addBlock: { mutateAsync: addBlock, isPending: false } }),
}));

function page(id: number, title: string, slug: string): Page {
  return { id, title, slug } as Page;
}

function wrap(ui: ReactNode) {
  return render(<TooltipProvider>{ui}</TooltipProvider>);
}

describe("LinkForm Add to a page", () => {
  beforeEach(() => {
    push.mockReset();
    createLink.mockReset().mockResolvedValue({ id: 11, short_url: "http://localhost:8080/abc" });
    addBlock.mockReset().mockResolvedValue({ id: 1 });
    toastSuccess.mockReset();
    toastError.mockReset();
    state.pages = [page(1, "Jose", "jose"), page(2, "Shop", "shop")];
  });

  it("the switch is enabled and adds the new link to the chosen page after create", async () => {
    const user = userEvent.setup();
    wrap(<LinkForm />);
    await user.type(screen.getByLabelText("Destination URL"), "https://www.example.com/menu");
    const sw = screen.getByRole("switch", { name: "Add to a page" });
    expect(sw).toBeEnabled();
    await user.click(sw);
    expect(screen.getByLabelText("Button title")).toHaveValue("www.example.com");
    await user.click(screen.getByRole("combobox", { name: "Page" }));
    await user.click(await screen.findByRole("option", { name: "Shop" }));
    await user.click(screen.getByRole("button", { name: "Create your link" }));

    expect(createLink).toHaveBeenCalledWith(expect.objectContaining({ original_url: "https://www.example.com/menu" }));
    expect(addBlock).toHaveBeenCalledWith({
      pageId: 2,
      input: { type: "link", link_id: 11, content: { title: "www.example.com", description: "", thumbnail_url: "" } },
    });
    expect(push).toHaveBeenCalledWith("/dashboard/links/11");
  });

  it("toasts and continues when the follow-up fails", async () => {
    addBlock.mockRejectedValue(new Error("boom"));
    const user = userEvent.setup();
    wrap(<LinkForm />);
    await user.type(screen.getByLabelText("Destination URL"), "https://example.com/x");
    await user.click(screen.getByRole("switch", { name: "Add to a page" }));
    await user.click(screen.getByRole("button", { name: "Create your link" }));
    expect(addBlock).toHaveBeenCalledWith(expect.objectContaining({ pageId: 1 }));
    expect(toastError).toHaveBeenCalledWith("Link created, but it couldn't be added to the page");
    expect(push).toHaveBeenCalledWith("/dashboard/links/11");
  });

  it("does not call addBlock when the switch is off", async () => {
    const user = userEvent.setup();
    wrap(<LinkForm />);
    await user.type(screen.getByLabelText("Destination URL"), "https://example.com/x");
    await user.click(screen.getByRole("button", { name: "Create your link" }));
    expect(createLink).toHaveBeenCalled();
    expect(addBlock).not.toHaveBeenCalled();
  });
});
