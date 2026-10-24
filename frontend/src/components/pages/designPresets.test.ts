import { describe, expect, it } from "vitest";
import {
  DESIGN_PRESETS,
  PRESET_THEMES,
  RESERVED_SLUGS,
  SLUG_RE,
  SOCIAL_PLATFORMS,
  isValidSlug,
  pageUrl,
} from "./designPresets";

const HEX = /^#[0-9A-F]{6}$/;

describe("DESIGN_PRESETS", () => {
  it("defines the five presets", () => {
    expect(Object.keys(DESIGN_PRESETS).sort()).toEqual(["bold", "classic", "dark", "minimal", "sunset"]);
    expect([...PRESET_THEMES].sort()).toEqual(Object.keys(DESIGN_PRESETS).sort());
  });

  it.each(Object.entries(DESIGN_PRESETS))("%s matches the design shape", (name, d) => {
    expect(d.theme).toBe(name);
    expect(["solid", "gradient"]).toContain(d.background.type);
    expect(["fill", "outline"]).toContain(d.button.style);
    expect(["square", "rounded", "pill"]).toContain(d.button.radius);
    expect(["inter", "serif", "mono"]).toContain(d.font);
    for (const c of [d.background.color, d.background.color2, d.text_color, d.button.color, d.button.text_color]) {
      expect(c).toMatch(HEX);
    }
  });

  it("uses the spec values", () => {
    expect(DESIGN_PRESETS.classic).toEqual({
      theme: "classic",
      background: { type: "solid", color: "#F4F6FA", color2: "#FFFFFF" },
      text_color: "#273144",
      button: { style: "fill", radius: "rounded", color: "#2A5BD7", text_color: "#FFFFFF" },
      font: "inter",
    });
    expect(DESIGN_PRESETS.bold.button).toEqual({ style: "fill", radius: "pill", color: "#F59E0B", text_color: "#111827" });
    expect(DESIGN_PRESETS.minimal.button.style).toBe("outline");
    expect(DESIGN_PRESETS.minimal.font).toBe("serif");
    expect(DESIGN_PRESETS.dark.background).toEqual({ type: "gradient", color: "#0F172A", color2: "#1E293B" });
    expect(DESIGN_PRESETS.dark.font).toBe("mono");
    expect(DESIGN_PRESETS.sunset.background).toEqual({ type: "gradient", color: "#F97316", color2: "#EC4899" });
    expect(DESIGN_PRESETS.sunset.button).toEqual({ style: "fill", radius: "pill", color: "#FFFFFF", text_color: "#9A3412" });
  });
});

describe("SLUG_RE / isValidSlug", () => {
  it.each(["abc", "jose", "my-page", "a1b", "a".repeat(40), "0-0"])("accepts %s", (s) => {
    expect(SLUG_RE.test(s)).toBe(true);
    expect(isValidSlug(s)).toBe(true);
  });

  it.each(["-abc", "abc-", "ABC", "a_b", "a b", "héllo", ""])("rejects %s by the regex", (s) => {
    expect(SLUG_RE.test(s)).toBe(false);
    expect(isValidSlug(s)).toBe(false);
  });

  it("enforces 3–40 chars", () => {
    expect(isValidSlug("ab")).toBe(false);
    expect(isValidSlug("a".repeat(41))).toBe(false);
  });

  it("lists the reserved slugs", () => {
    expect([...RESERVED_SLUGS].sort()).toEqual(
      ["admin", "api", "assets", "dashboard", "health", "login", "m", "register", "static"].sort(),
    );
  });
});

describe("SOCIAL_PLATFORMS / pageUrl", () => {
  it("has a label and icon per platform", () => {
    expect(Object.keys(SOCIAL_PLATFORMS).sort()).toEqual(
      ["email", "facebook", "instagram", "linkedin", "tiktok", "website", "x", "youtube"].sort(),
    );
    for (const p of Object.values(SOCIAL_PLATFORMS)) {
      expect(p.label).toBeTruthy();
      expect(p.icon).toBeTruthy();
    }
  });

  it("builds the shareable page URL", () => {
    expect(pageUrl("http://localhost:8080", "jose")).toBe("http://localhost:8080/m/jose");
    expect(pageUrl("https://s.example/", "jose")).toBe("https://s.example/m/jose");
  });
});
