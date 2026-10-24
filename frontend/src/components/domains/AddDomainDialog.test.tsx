import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AddDomainDialog } from "./AddDomainDialog";

const createDomain = vi.fn();
const verifyDomain = vi.fn();

const domain = {
  id: 7,
  user_id: 1,
  group_id: null,
  domain: "links.brand.test",
  verification_token: "abc123",
  verification_status: "pending",
  verification_error: null,
  verified_at: null,
  last_checked_at: null,
  root_redirect: null,
  not_found_redirect: null,
  created_at: "2026-10-24T00:00:00Z",
};

vi.mock("@/lib/hooks", () => ({
  useMeta: () => ({ data: { redirect_host: "go.example.test", redirect_ip: "" } }),
  useGroups: () => ({ data: [] }),
  useCreateDomain: () => ({ mutateAsync: createDomain, isPending: false }),
  useVerifyDomain: () => ({ mutateAsync: verifyDomain, isPending: false }),
}));

describe("AddDomainDialog", () => {
  beforeEach(() => {
    createDomain.mockReset().mockResolvedValue(domain);
    verifyDomain.mockReset().mockResolvedValue({ ...domain, verification_status: "ok" });
  });

  it("shows CNAME and TXT instructions after step 1 and verifies the domain", async () => {
    const user = userEvent.setup();
    render(<AddDomainDialog open onOpenChange={() => {}} />);

    await user.type(screen.getByLabelText("Domain"), "links.brand.test");
    await user.click(screen.getByRole("button", { name: "Next" }));

    expect(createDomain).toHaveBeenCalledWith({ domain: "links.brand.test", group_id: null });

    const cname = await screen.findByTestId("dns-record-CNAME");
    expect(within(cname).getByText("CNAME")).toBeInTheDocument();
    expect(within(cname).getByText("go.example.test")).toBeInTheDocument();

    const txt = screen.getByTestId("dns-record-TXT");
    expect(within(txt).getByText("_urlshortener.links.brand.test")).toBeInTheDocument();
    expect(within(txt).getByText("verify=abc123")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Verify" }));
    expect(verifyDomain).toHaveBeenCalledWith(7);
    expect(await screen.findByText("Your domain is verified")).toBeInTheDocument();
  });

  it("shows the returned error with Try again when verification fails", async () => {
    verifyDomain.mockResolvedValue({ ...domain, verification_status: "error", verification_error: "no TXT record found" });
    const user = userEvent.setup();
    render(<AddDomainDialog open onOpenChange={() => {}} initialDomain={domain as never} />);
    await user.click(screen.getByRole("button", { name: "Verify" }));
    expect(await screen.findByText("no TXT record found")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "I'll do this later" })).toBeInTheDocument();
  });
});
