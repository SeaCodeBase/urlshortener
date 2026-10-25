import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ApiError } from "@/lib/api";
import type { Domain, Group } from "@/types";
import { DomainRow } from "./DomainRow";

const updateGroup = vi.fn();
const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: (m: string) => toastError(m) } }));
vi.mock("@/lib/hooks", () => ({
  useVerifyDomain: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteDomain: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateGroup: () => ({ mutateAsync: updateGroup, isPending: false }),
}));
vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: (s: unknown) => unknown) => sel({ activeGroupId: 3 }),
}));
vi.mock("./DomainSettingsDialog", () => ({ DomainSettingsDialog: () => null }));

const base: Domain = {
  id: 7,
  user_id: 1,
  group_id: null,
  domain: "links.brand.test",
  verification_token: "abc123",
  verification_status: "ok",
  verification_error: null,
  verified_at: "2026-10-24T00:00:00Z",
  last_checked_at: null,
  root_redirect: null,
  not_found_redirect: null,
  created_at: "2026-10-24T00:00:00Z",
} as Domain;

const groups = [{ id: 3, name: "Marketing", default_domain_id: null }] as unknown as Group[];

function renderRow(domain: Domain, gs: Group[] = groups) {
  return render(
    <TooltipProvider>
      <ul>
        <DomainRow domain={domain} groups={gs} onShowInstructions={() => {}} />
      </ul>
    </TooltipProvider>,
  );
}

async function openMenu() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "More actions for links.brand.test" }));
  return user;
}

describe("DomainRow — Set as default for group", () => {
  beforeEach(() => {
    updateGroup.mockReset().mockResolvedValue({});
    toastSuccess.mockReset();
    toastError.mockReset();
  });

  it("is disabled for a pending domain", async () => {
    renderRow({ ...base, verification_status: "pending" });
    await openMenu();
    const item = await screen.findByRole("menuitem", { name: "Set as default for group" });
    expect(item).toHaveAttribute("aria-disabled", "true");
  });

  it("sets the verified domain as the active group's default", async () => {
    renderRow(base);
    const user = await openMenu();
    const item = await screen.findByRole("menuitem", { name: "Set as default for group" });
    expect(item).not.toHaveAttribute("aria-disabled");
    await user.click(item);
    expect(updateGroup).toHaveBeenCalledWith({ id: 3, data: { default_domain_id: 7 } });
    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Default domain updated"));
  });

  it("toasts an ApiError", async () => {
    updateGroup.mockRejectedValue(new ApiError("domain not available to this group", 400));
    renderRow(base);
    const user = await openMenu();
    await user.click(await screen.findByRole("menuitem", { name: "Set as default for group" }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("domain not available to this group"));
  });

  it("is hidden when the domain is already the group's default", async () => {
    renderRow(base, [{ id: 3, name: "Marketing", default_domain_id: 7 }] as unknown as Group[]);
    await openMenu();
    await screen.findByRole("menuitem", { name: "Remove" });
    expect(screen.queryByRole("menuitem", { name: "Set as default for group" })).not.toBeInTheDocument();
  });
});
