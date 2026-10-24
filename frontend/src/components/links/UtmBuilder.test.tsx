import { describe, expect, it } from "vitest";
import { buildUtmPreview } from "./UtmBuilder";

describe("buildUtmPreview", () => {
  it("appends UTM params before the fragment, keeping existing params", () => {
    expect(buildUtmPreview("https://a.com/p?x=1#f", { source: "s", medium: "m" })).toBe(
      "https://a.com/p?x=1&utm_source=s&utm_medium=m#f",
    );
  });

  it("skips empty fields", () => {
    expect(
      buildUtmPreview("https://a.com/", { source: "s", medium: "", campaign: "  ", custom: [{ key: "", value: "v" }] }),
    ).toBe("https://a.com/?utm_source=s");
  });

  it("overwrites an existing utm_source", () => {
    expect(buildUtmPreview("https://a.com/?utm_source=old&y=2", { source: "new" })).toBe(
      "https://a.com/?utm_source=new&y=2",
    );
  });

  it("adds custom parameters", () => {
    expect(buildUtmPreview("https://a.com/", { custom: [{ key: "ref", value: "home" }] })).toBe(
      "https://a.com/?ref=home",
    );
  });

  it("returns an empty string for an invalid destination", () => {
    expect(buildUtmPreview("not a url", { source: "s" })).toBe("");
  });
});
