import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Domain } from "@/types";
import { ShortLinkField, domainIdForCreate, generateBackHalf, usableDomains } from "./ShortLinkField";

vi.mock("@/lib/hooks", () => ({
  useMeta: () => ({ data: { base_url: "http://localhost:8080" } }),
  useActiveGroupId: () => 7,
  useGroups: () => ({ data: [{ id: 7, name: "g", default_domain_id: 2, created_at: "", updated_at: "" }] }),
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

const DOMAINS = [
  domain(1, "shared.test", null),
  domain(2, "mine.test", 7),
  domain(3, "other-group.test", 8),
  domain(4, "pending.test", null, "pending"),
];

function Harness({
  onBackHalf,
  domains = [],
  domainId = null,
}: {
  onBackHalf?: (v: string) => void;
  domains?: Domain[];
  domainId?: number | null;
}) {
  const [backHalf, setBackHalf] = useState("");
  return (
    <ShortLinkField
      domains={domains}
      domainId={domainId}
      onDomainChange={() => {}}
      backHalf={backHalf}
      onBackHalfChange={(v) => {
        setBackHalf(v);
        onBackHalf?.(v);
      }}
    />
  );
}

function renderField(onBackHalf?: (v: string) => void, domains: Domain[] = [], domainId: number | null = null) {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <Harness onBackHalf={onBackHalf} domains={domains} domainId={domainId} />
    </QueryClientProvider>,
  );
}

describe("ShortLinkField", () => {
  it("Generate fills the back-half with 7 alphanumerics", () => {
    const onBackHalf = vi.fn();
    renderField(onBackHalf);
    fireEvent.click(screen.getByRole("button", { name: "Generate" }));
    const value = onBackHalf.mock.calls.at(-1)?.[0] as string;
    expect(value).toMatch(/^[a-zA-Z0-9]{7}$/);
    expect(screen.getByLabelText("Back-half (optional)")).toHaveValue(value);
  });

  it("generateBackHalf always yields 7 alphanumerics", () => {
    for (let i = 0; i < 50; i++) expect(generateBackHalf()).toMatch(/^[a-zA-Z0-9]{7}$/);
  });

  it("rejects an invalid back-half on blur", () => {
    renderField();
    const input = screen.getByLabelText("Back-half (optional)");
    fireEvent.change(input, { target: { value: "bad code!" } });
    fireEvent.blur(input);
    expect(screen.getByText("Only letters, numbers, - and _ are allowed")).toBeInTheDocument();
    expect(input).toHaveAttribute("aria-invalid", "true");
  });

  it("shows the default domain host", () => {
    renderField();
    expect(screen.getByText("localhost:8080")).toBeInTheDocument();
  });

  it("shows a selected domain of the active group", () => {
    renderField(undefined, DOMAINS, 2);
    expect(screen.getByText("mine.test")).toBeInTheDocument();
  });

  it("does not offer a domain bound to another group", () => {
    renderField(undefined, DOMAINS, 3);
    expect(screen.queryByText("other-group.test")).not.toBeInTheDocument();
  });
});

describe("usableDomains", () => {
  it("keeps verified domains that are shared or bound to the active group", () => {
    expect(usableDomains(DOMAINS, 7).map((d) => d.id)).toEqual([1, 2]);
    expect(usableDomains(DOMAINS, 8).map((d) => d.id)).toEqual([1, 3]);
    expect(usableDomains(undefined, 7)).toEqual([]);
  });
});

describe("domainIdForCreate", () => {
  it("sends the explicit pick, 0 for the default domain, else the group default", () => {
    expect(domainIdForCreate(5, 2)).toBe(5);
    expect(domainIdForCreate(null, 2)).toBe(0);
    expect(domainIdForCreate(undefined, 2)).toBe(2);
    expect(domainIdForCreate(undefined, null)).toBeUndefined();
  });
});
