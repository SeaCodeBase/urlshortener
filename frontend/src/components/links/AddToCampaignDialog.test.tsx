import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiError } from "@/lib/api";
import type { CampaignDetail, CampaignSummary, Link } from "@/types";
import { AddToCampaignDialog } from "./AddToCampaignDialog";

const addLinks = vi.fn();
const toastSuccess = vi.fn();
const state: { campaigns: CampaignSummary[]; details: Record<number, CampaignDetail>; requested: unknown[] } = {
  campaigns: [],
  details: {},
  requested: [],
};

vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: vi.fn() } }));
vi.mock("@/lib/hooks", () => ({
  useCampaigns: () => ({ data: state.campaigns, isLoading: false }),
  useCampaign: (id: number | null | undefined) => {
    state.requested.push(id);
    return { data: id ? state.details[id] : undefined, isLoading: false };
  },
  useCampaignMutations: () => ({ addLinks: { mutateAsync: addLinks, isPending: false } }),
}));

function summary(id: number, name: string): CampaignSummary {
  return {
    id,
    user_id: 1,
    group_id: 1,
    name,
    description: "",
    auto_utm: false,
    created_at: "",
    updated_at: "",
    channel_count: 0,
    link_count: 0,
    engagements: 0,
  };
}

function detail(id: number, name: string, channels: { id: number; name: string }[]): CampaignDetail {
  return {
    id,
    user_id: 1,
    group_id: 1,
    name,
    description: "",
    auto_utm: false,
    created_at: "",
    updated_at: "",
    channels: channels.map((ch) => ({ ...ch, group_id: 1, created_at: "", links: [] })) };
}

const link = { id: 9, title: "Menu", original_url: "https://example.com" } as Link;

describe("AddToCampaignDialog", () => {
  beforeEach(() => {
    addLinks.mockReset().mockResolvedValue(undefined);
    toastSuccess.mockReset();
    state.campaigns = [summary(1, "Spring sale"), summary(2, "Empty one")];
    state.details = {
      1: detail(1, "Spring sale", [
        { id: 11, name: "Email" },
        { id: 12, name: "Social" },
      ]),
      2: detail(2, "Empty one", []),
    };
    state.requested = [];
  });

  it("cascades campaign → channel and adds the link", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<AddToCampaignDialog open onOpenChange={onOpenChange} link={link} />);
    expect(screen.getByRole("heading", { name: "Add to campaign" })).toBeInTheDocument();
    const channel = screen.getByRole("combobox", { name: "Channel" });
    expect(channel).toBeDisabled();
    expect(screen.getByRole("button", { name: "Add" })).toBeDisabled();

    await user.click(screen.getByRole("combobox", { name: "Campaign" }));
    await user.click(await screen.findByRole("option", { name: "Spring sale" }));
    expect(state.requested).toContain(1);
    expect(screen.getByRole("combobox", { name: "Channel" })).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "Add" })).toBeDisabled();

    await user.click(screen.getByRole("combobox", { name: "Channel" }));
    await user.click(await screen.findByRole("option", { name: "Social" }));
    await user.click(screen.getByRole("button", { name: "Add" }));

    expect(addLinks).toHaveBeenCalledWith({ campaignId: 1, channelId: 12, linkIds: [9] });
    expect(toastSuccess).toHaveBeenCalledWith("Added to Spring sale");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("says when the campaign has no channels and keeps Add disabled", async () => {
    const user = userEvent.setup();
    render(<AddToCampaignDialog open onOpenChange={vi.fn()} link={link} />);
    await user.click(screen.getByRole("combobox", { name: "Campaign" }));
    await user.click(await screen.findByRole("option", { name: "Empty one" }));
    expect(screen.getByText("This campaign has no channels")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add" })).toBeDisabled();
  });

  it("shows API errors inline and stays open", async () => {
    addLinks.mockRejectedValue(new ApiError("links must be regular links", 400));
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<AddToCampaignDialog open onOpenChange={onOpenChange} link={link} />);
    await user.click(screen.getByRole("combobox", { name: "Campaign" }));
    await user.click(await screen.findByRole("option", { name: "Spring sale" }));
    await user.click(screen.getByRole("combobox", { name: "Channel" }));
    await user.click(await screen.findByRole("option", { name: "Email" }));
    await user.click(screen.getByRole("button", { name: "Add" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("links must be regular links");
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("links to campaigns when there are none", () => {
    state.campaigns = [];
    render(<AddToCampaignDialog open onOpenChange={vi.fn()} link={link} />);
    expect(screen.getByRole("link", { name: "Create campaign" })).toHaveAttribute("href", "/dashboard/campaigns");
    expect(screen.queryByRole("button", { name: "Add" })).not.toBeInTheDocument();
  });
});
