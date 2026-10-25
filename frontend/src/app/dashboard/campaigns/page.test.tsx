import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { CampaignSummary } from "@/types";
import CampaignsPage from "./page";

const remove = vi.fn();
const toastSuccess = vi.fn();
const state: { data: CampaignSummary[] | undefined; isPending: boolean; isError: boolean } = {
  data: [],
  isPending: false,
  isError: false,
};

vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: vi.fn() } }));
vi.mock("@/lib/hooks", () => ({
  useCampaigns: () => state,
  useChannels: () => ({ data: [], isPending: false }),
  useCampaign: () => ({ data: undefined, isPending: false }),
  useCampaignMutations: () => ({
    create: { mutateAsync: vi.fn(), isPending: false },
    update: { mutateAsync: vi.fn(), isPending: false },
    remove: { mutateAsync: remove, isPending: false },
    createChannel: { mutateAsync: vi.fn(), isPending: false },
  }),
}));

function summary(id: number, name: string, extra: Partial<CampaignSummary> = {}): CampaignSummary {
  return {
    id,
    user_id: 1,
    group_id: 1,
    name,
    description: "",
    auto_utm: false,
    created_at: "2026-10-24T12:00:00Z",
    updated_at: "2026-10-24T12:00:00Z",
    channel_count: 0,
    link_count: 0,
    engagements: 0,
    ...extra,
  };
}

describe("Campaigns page", () => {
  beforeEach(() => {
    remove.mockReset().mockResolvedValue(undefined);
    toastSuccess.mockReset();
    state.data = [];
    state.isPending = false;
    state.isError = false;
  });

  it("renders a card per campaign with meta and created date", () => {
    state.data = [
      summary(3, "Fall launch", { description: "Q4 push", channel_count: 2, link_count: 1, engagements: 1240 }),
      summary(4, "Spring", { channel_count: 1, link_count: 3, engagements: 1 }),
    ];
    render(<CampaignsPage />);
    expect(screen.getByRole("heading", { name: "Campaigns" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Fall launch" })).toHaveAttribute("href", "/dashboard/campaigns/3");
    expect(screen.getByText("Q4 push")).toBeInTheDocument();
    expect(screen.getByText("2 channels · 1 link · 1,240 engagements")).toBeInTheDocument();
    expect(screen.getByText("1 channel · 3 links · 1 engagement")).toBeInTheDocument();
    expect(screen.getAllByText("Oct 24, 2026")).toHaveLength(2);
  });

  it("shows the empty state", () => {
    render(<CampaignsPage />);
    expect(screen.getByRole("button", { name: "Create your first campaign" })).toBeInTheDocument();
  });

  it("shows the error state", () => {
    state.data = undefined;
    state.isError = true;
    render(<CampaignsPage />);
    expect(screen.getByRole("alert")).toHaveTextContent("Failed to load campaigns");
  });

  it("opens the create dialog from the header", async () => {
    const user = userEvent.setup();
    render(<CampaignsPage />);
    await user.click(screen.getByRole("button", { name: "Create campaign" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });

  it("deletes a campaign after confirmation", async () => {
    state.data = [summary(3, "Fall launch")];
    const user = userEvent.setup();
    render(<CampaignsPage />);
    await user.click(screen.getByRole("button", { name: "More" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText("Delete Fall launch?")).toBeInTheDocument();
    expect(
      within(dialog).getByText("Links stay in your account; only the campaign and its channel assignments are removed."),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));
    expect(remove).toHaveBeenCalledWith(3);
    expect(toastSuccess).toHaveBeenCalledWith("Campaign deleted");
  });
});
