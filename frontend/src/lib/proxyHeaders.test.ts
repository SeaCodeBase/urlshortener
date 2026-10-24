import { describe, expect, it } from "vitest";
import { buildProxyRequestHeaders, buildProxyResponseHeaders } from "./proxyHeaders";

describe("buildProxyRequestHeaders", () => {
  it("forwards the allowlisted headers, including X-Group-ID", () => {
    const out = buildProxyRequestHeaders(
      new Headers({
        Authorization: "Bearer t",
        "Content-Type": "text/plain",
        Accept: "text/csv",
        "X-Group-ID": "7",
      }),
    );
    expect(out.get("Authorization")).toBe("Bearer t");
    expect(out.get("Content-Type")).toBe("text/plain");
    expect(out.get("Accept")).toBe("text/csv");
    expect(out.get("X-Group-ID")).toBe("7");
  });

  it("passes a multipart Content-Type (with its boundary) through unchanged", () => {
    const ct = "multipart/form-data; boundary=----abc123";
    expect(buildProxyRequestHeaders(new Headers({ "Content-Type": ct })).get("Content-Type")).toBe(ct);
  });

  it("drops headers outside the allowlist", () => {
    const out = buildProxyRequestHeaders(new Headers({ Cookie: "a=b", Host: "evil", "X-Real-Port": "1" }));
    expect(out.has("Cookie")).toBe(false);
    expect(out.has("Host")).toBe(false);
    expect(out.has("X-Real-Port")).toBe(false);
  });

  it("forwards User-Agent and Referer for page-view tracking", () => {
    const out = buildProxyRequestHeaders(
      new Headers({ "User-Agent": "Mozilla/5.0 Test", Referer: "https://instagram.com/" }),
    );
    expect(out.get("User-Agent")).toBe("Mozilla/5.0 Test");
    expect(out.get("Referer")).toBe("https://instagram.com/");
  });

  it("forwards the visitor IP as X-Forwarded-For", () => {
    const out = buildProxyRequestHeaders(new Headers({ "X-Forwarded-For": "1.2.3.4, 10.0.0.1" }));
    expect(out.get("X-Forwarded-For")).toBe("1.2.3.4, 10.0.0.1");
  });

  it("falls back to X-Real-IP for X-Forwarded-For", () => {
    const out = buildProxyRequestHeaders(new Headers({ "X-Real-IP": "5.6.7.8" }));
    expect(out.get("X-Forwarded-For")).toBe("5.6.7.8");
  });

  it("prefers X-Forwarded-For over X-Real-IP", () => {
    const out = buildProxyRequestHeaders(new Headers({ "X-Forwarded-For": "1.2.3.4", "X-Real-IP": "5.6.7.8" }));
    expect(out.get("X-Forwarded-For")).toBe("1.2.3.4");
  });

  it("defaults Content-Type to JSON and omits missing optional headers", () => {
    const out = buildProxyRequestHeaders(new Headers());
    expect(out.get("Content-Type")).toBe("application/json");
    expect(out.has("Authorization")).toBe(false);
    expect(out.has("X-Group-ID")).toBe(false);
    expect(out.has("X-Forwarded-For")).toBe(false);
  });
});

describe("buildProxyResponseHeaders", () => {
  it("passes Content-Type and Content-Disposition back (CSV export)", () => {
    const out = buildProxyResponseHeaders(
      new Headers({
        "Content-Type": "text/csv; charset=utf-8",
        "Content-Disposition": 'attachment; filename="links.csv"',
        "Set-Cookie": "x=y",
      }),
    );
    expect(out.get("Content-Type")).toBe("text/csv; charset=utf-8");
    expect(out.get("Content-Disposition")).toBe('attachment; filename="links.csv"');
    expect(out.has("Set-Cookie")).toBe(false);
  });

  it("defaults Content-Type to JSON", () => {
    const out = buildProxyResponseHeaders(new Headers());
    expect(out.get("Content-Type")).toBe("application/json");
    expect(out.has("Content-Disposition")).toBe(false);
  });
});
