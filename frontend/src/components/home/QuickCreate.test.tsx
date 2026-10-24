import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DEFAULT_QR_DESIGN, type Domain, type QrCode } from "@/types";
import { QuickCreate } from "./QuickCreate";

const mutateAsync = vi.fn().mockResolvedValue({ id: 1, short_url: "http://localhost:8080/abc" });
const push = vi.fn();
const createQr = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
}));

const hookState: { domains: Domain[]; defaultDomainId: number | null } = { domains: [], defaultDomainId: null };

vi.mock("@/components/qr/QrPreview", () => ({ QrPreview: () => <div data-testid="qr-preview" /> }));
vi.mock("@/components/qr/QrDownloadMenu", () => ({ QrDownloadMenu: () => <button type="button">Download</button> }));

const qr: QrCode = {
  id: 5,
  user_id: 1,
  group_id: 7,
  link_id: 11,
  title: null,
  render_customizations: {},
  archived: false,
  created_at: "",
  updated_at: "",
  tags: [],
  total_scans: 0,
  short_url: "http://localhost:8080/q",
  qr_url: "http://localhost:8080/q",
  original_url: "https://example.com/q",
  favicon_url: "",
  expires_at: null,
  link_source: "qr",
  image_url: "",
};

vi.mock("@/lib/hooks", () => ({
  useCreateLink: () => ({ mutateAsync, isPending: false }),
  useCreateQrCode: () => ({ mutateAsync: createQr, isPending: false }),
  useDomains: () => ({ data: hookState.domains }),
  useMeta: () => ({ data: { base_url: "http://localhost:8080" } }),
  useActiveGroupId: () => 7,
  useGroups: () => ({
    data: [{ id: 7, name: "g", default_domain_id: hookState.defaultDomainId, created_at: "", updated_at: "" }],
  }),
}));

function domain(id: number, name: string, groupId: number | null, status: Domain["verification_status"] = "ok"): Domain {
  return {
    id,
    user_id: 1,
    group_id: groupId,
    domain: name,
    verification_token: "t",
    verification_status: status,
    verification_error: null,
    verified_at: null,
    last_checked_at: null,
    root_redirect: null,
    not_found_redirect: null,
    created_at: "",
  };
}

describe("QuickCreate", () => {
  beforeEach(() => {
    hookState.domains = [];
    hookState.defaultDomainId = null;
    mutateAsync.mockClear();
    createQr.mockReset().mockResolvedValue(qr);
  });

  it("submits on Enter and shows the link-ready dialog", async () => {
    const user = userEvent.setup();
    render(<QuickCreate />);
    await user.type(screen.getByPlaceholderText("Paste a long URL to shorten it..."), "https://example.com/long{Enter}");
    expect(await screen.findByText("Your link is ready!")).toBeInTheDocument();
    expect(screen.getByText("localhost:8080/abc")).toBeInTheDocument();
    expect(mutateAsync).toHaveBeenCalledWith(expect.objectContaining({ original_url: "https://example.com/long" }));
    expect(screen.getByRole("button", { name: /Copy link/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /View link details/ })).toHaveAttribute("href", "/dashboard/links/1");
  });

  it("QR Code mode creates the code with the default design and shows the ready modal", async () => {
    const user = userEvent.setup();
    render(<QuickCreate />);
    await user.click(screen.getByRole("radio", { name: "QR Code" }));
    expect(screen.getByRole("combobox", { name: "Domain" })).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText("Paste a long URL to shorten it..."), "https://example.com/q");
    await user.click(screen.getByRole("button", { name: /Create your code/ }));
    expect(createQr).toHaveBeenCalledWith({
      long_url: "https://example.com/q",
      domain_id: undefined,
      share_as_link: false,
      render_customizations: DEFAULT_QR_DESIGN,
    });
    expect(await screen.findByText("Your code is ready!")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /View code details/ })).toHaveAttribute("href", "/dashboard/qr-codes/5");
    expect(mutateAsync).not.toHaveBeenCalled();
    expect(push).not.toHaveBeenCalled();
  });

  it("QR Code mode sends share_as_link when Also create a link is checked, Enter submits", async () => {
    const user = userEvent.setup();
    render(<QuickCreate />);
    await user.click(screen.getByRole("radio", { name: "QR Code" }));
    await user.click(screen.getByRole("checkbox", { name: "Also create a link for this QR Code" }));
    await user.type(screen.getByPlaceholderText("Paste a long URL to shorten it..."), "https://example.com/s{Enter}");
    expect(createQr).toHaveBeenCalledWith(expect.objectContaining({ long_url: "https://example.com/s", share_as_link: true }));
    expect(await screen.findByText("Your code is ready!")).toBeInTheDocument();
  });

  it("Also create a QR Code attaches a code to the new link and offers View QR Code", async () => {
    const user = userEvent.setup();
    render(<QuickCreate />);
    await user.click(screen.getByRole("checkbox", { name: "Also create a QR Code for this link" }));
    await user.type(screen.getByPlaceholderText("Paste a long URL to shorten it..."), "https://example.com/l{Enter}");
    expect(await screen.findByText("Your link is ready!")).toBeInTheDocument();
    expect(createQr).toHaveBeenCalledWith({ link_id: 1, render_customizations: DEFAULT_QR_DESIGN });
    expect(screen.getByRole("link", { name: /View QR Code/ })).toHaveAttribute("href", "/dashboard/qr-codes/5");
    expect(screen.queryByRole("link", { name: /Create QR Code/ })).not.toBeInTheDocument();
  });

  it("preselects the active group's default domain and sends it", async () => {
    hookState.domains = [domain(2, "go.brand.test", 7), domain(3, "other-group.test", 8)];
    hookState.defaultDomainId = 2;
    const user = userEvent.setup();
    render(<QuickCreate />);
    expect(screen.getByRole("combobox", { name: "Domain" })).toHaveTextContent("go.brand.test");
    expect(screen.queryByText("other-group.test")).not.toBeInTheDocument();
    await user.type(screen.getByPlaceholderText("Paste a long URL to shorten it..."), "https://example.com/d{Enter}");
    expect(mutateAsync).toHaveBeenCalledWith(expect.objectContaining({ domain_id: 2 }));
  });

  it("ignores a group default that is not usable", async () => {
    hookState.domains = [domain(2, "go.brand.test", 7, "error")];
    hookState.defaultDomainId = 2;
    const user = userEvent.setup();
    render(<QuickCreate />);
    expect(screen.getByRole("combobox", { name: "Domain" })).toHaveTextContent("localhost:8080");
    await user.type(screen.getByPlaceholderText("Paste a long URL to shorten it..."), "https://example.com/e{Enter}");
    expect(mutateAsync).toHaveBeenCalledWith(expect.objectContaining({ domain_id: undefined }));
  });
});
