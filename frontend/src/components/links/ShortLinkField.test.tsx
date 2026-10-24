import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ShortLinkField, generateBackHalf } from "./ShortLinkField";

vi.mock("@/lib/hooks", () => ({
  useMeta: () => ({ data: { base_url: "http://localhost:8080" } }),
}));

function Harness({ onBackHalf }: { onBackHalf?: (v: string) => void }) {
  const [backHalf, setBackHalf] = useState("");
  return (
    <ShortLinkField
      domains={[]}
      domainId={null}
      onDomainChange={() => {}}
      backHalf={backHalf}
      onBackHalfChange={(v) => {
        setBackHalf(v);
        onBackHalf?.(v);
      }}
    />
  );
}

function renderField(onBackHalf?: (v: string) => void) {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <Harness onBackHalf={onBackHalf} />
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
});
