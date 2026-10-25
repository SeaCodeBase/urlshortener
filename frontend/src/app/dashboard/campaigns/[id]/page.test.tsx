import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { CampaignDetail, CampaignLinkItem, CampaignStats, Link } from "@/types";
import CampaignDetailPage from "./page";

vi.mock("next/navigation", () => ({ useParams: () => ({ id: "5" }) }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/components/common/Favicon", () => ({ Favicon: () => <span data-testid="favicon" /> }));
vi.mock("@/components/charts/TimeSeriesChart", () => ({
  TimeSeriesChart: (p: { multiSeries?: { name: string }[] }) => (
    <div data-testid="chart">{p.multiSeries?.map((s) => s.name).join(",")}</div>
  ),
}));

const removeLink = vi.fn();
const state: {
  detail: CampaignDetail | undefined;
  isError: boolean;
  stats: CampaignStats | undefined;
  campaignId: unknown;
} = { detail: undefined, isError: false, stats: undefined, campaignId: undefined };

vi.mock("@/lib/hooks", () => ({
  useCampaign: (id: unknown) => {
    if (id != null) state.campaignId = id;
    return { data: state.detail, isPending: false, isLoading: false, isError: state.isError };
  },
  useCampaignStats: () => ({ data: state.stats, isLoading: false, isError: false, refetch: vi.fn() }),
  useChannels: () => ({ data: [], isPending: false }),
  useDebouncedValue: <T,>(v: T) => v,
  useLinks: () => ({ data: { pages: [] }, isLoading: false }),
  useCampaignMutations: () => ({
    create: { mutateAsync: vi.fn(), isPending: false },
    update: { mutateAsync: vi.fn(), isPending: false },
    createChannel: { mutateAsync: vi.fn(), isPending: false },
    addLinks: { mutateAsync: vi.fn(), isPending: false },
    createLinkInChannel: { mutateAsync: vi.fn(), isPending: false },
    removeLink: { mutateAsync: removeLink, isPending: false },
  }),
}));

function item(id: number, title: string, channelId: number, channelName: string, clicks: number): CampaignLinkItem {
  const l: Link = {
    id,
    user_id: 1,
    group_id: 1,
    short_code: `c${id}`,
    original_url: `https://example.com/${id}`,
    title,
    expires_at: null,
    is_active: true,
    archived: false,
    has_custom_back_half: false,
    redirect_count: 0,
    source: "link",
    created_at: "2026-10-20T12:00:00Z",
    updated_at: "2026-10-20T12:00:00Z",
    short_url: `http://localhost:8080/c${id}`,
    tags: [],
    total_clicks: clicks,
    favicon_url: "",
  };
  return { ...l, channel_id: channelId, channel_name: channelName };
}

function detail(over: Partial<CampaignDetail> = {}): CampaignDetail {
  return {
    id: 5,
    user_id: 1,
    group_id: 1,
    name: "Fall launch",
    description: "Autumn promo",
    auto_utm: true,
    created_at: "2026-10-01T00:00:00Z",
    updated_at: "2026-10-01T00:00:00Z",
    channels: [
      { id: 10, group_id: 1, name: "Email", created_at: "", links: [item(1, "Newsletter", 10, "Email", 42), item(2, "", 10, "Email", 3)] },
      { id: 11, group_id: 1, name: "Social", created_at: "", links: [item(1, "Newsletter", 11, "Social", 42)] },
    ],
    ...over,
  };
}

beforeEach(() => {
  removeLink.mockReset().mockResolvedValue(undefined);
  state.campaignId = undefined;
  state.detail = detail();
  state.isError = false;
  state.stats = {
    totals: { links: 2, engagements: 50 },
    channels: [
      { channel_id: 10, name: "Email", links: 2, engagements: 45, series: [] },
      { channel_id: 11, name: "Social", links: 1, engagements: 5, series: [] },
    ],
  };
});

describe("CampaignDetailPage", () => {
  it("renders the header and one card per channel", () => {
    render(<CampaignDetailPage />);
    expect(state.campaignId).toBe(5);
    expect(screen.getByRole("link", { name: /Campaigns/ })).toHaveAttribute("href", "/dashboard/campaigns");
    expect(screen.getByRole("heading", { level: 1, name: "Fall launch" })).toBeInTheDocument();
    expect(screen.getByText("Autumn promo")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument();

    const email = screen.getByRole("region", { name: "Email" });
    expect(within(email).getByText("2 links · 45 engagements in range")).toBeInTheDocument();
    expect(within(email).getByText("Newsletter")).toBeInTheDocument();
    expect(within(email).getByText("example.com/2")).toBeInTheDocument();
    expect(within(email).getByText("localhost:8080/c1")).toBeInTheDocument();
    expect(within(email).getByRole("button", { name: "Add link to channel" })).toBeInTheDocument();
    const social = screen.getByRole("region", { name: "Social" });
    expect(within(social).getByText("1 link · 5 engagements in range")).toBeInTheDocument();

    expect(screen.getByRole("heading", { name: "Performance" })).toBeInTheDocument();
    expect(screen.getByTestId("chart")).toHaveTextContent("Email,Social");
  });

  it("removes a link from a channel after confirming", async () => {
    const user = userEvent.setup();
    render(<CampaignDetailPage />);
    const social = screen.getByRole("region", { name: "Social" });
    await user.click(within(social).getByRole("button", { name: "More actions for Newsletter" }));
    await user.click(await screen.findByRole("menuitem", { name: "Remove from channel" }));
    expect(screen.getByText("Remove from channel?")).toBeInTheDocument();
    expect(screen.getByText("The link itself is not deleted.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Remove" }));
    expect(removeLink).toHaveBeenCalledWith({ campaignId: 5, channelId: 11, linkId: 1 });
  });

  it("lists every membership in the Links tab", async () => {
    const user = userEvent.setup();
    render(<CampaignDetailPage />);
    await user.click(screen.getByRole("tab", { name: "Links" }));
    const rows = within(screen.getAllByRole("table")[0]).getAllByRole("row");
    expect(rows).toHaveLength(4);
    expect(within(rows[0]).getAllByRole("columnheader").map((h) => h.textContent)).toEqual([
      "Link",
      "Channel",
      "Engagements",
      "Created",
    ]);
    expect(within(rows[3]).getByText("Social")).toBeInTheDocument();
    expect(within(rows[3]).getByText("42")).toBeInTheDocument();
  });

  it("shows an empty state without channels", () => {
    state.detail = detail({ channels: [] });
    render(<CampaignDetailPage />);
    expect(screen.getByText("No channels yet")).toBeInTheDocument();
    expect(screen.getByText("Edit the campaign to add channels.")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Edit" })).toHaveLength(2);
  });

  it("shows an alert when the campaign fails to load", () => {
    state.detail = undefined;
    state.isError = true;
    render(<CampaignDetailPage />);
    expect(screen.getByRole("alert")).toHaveTextContent("Failed to load campaign");
  });
});
