import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { SETTINGS_NAV } from "@/components/settings/SettingsNav";
import { GLOBAL_SHORTCUTS } from "@/lib/hooks/useGlobalShortcuts";
import { CREATE_ITEMS } from "./CreateNewMenu";
import { NAV_ITEMS } from "./Sidebar";

const hrefs = [
  ...NAV_ITEMS.map((i) => i.href),
  ...SETTINGS_NAV.flatMap((g) => g.items.map((i) => i.href)),
  ...CREATE_ITEMS.map((i) => i.href),
  ...Object.values(GLOBAL_SHORTCUTS),
];

describe("navigation route coverage", () => {
  it.each([...new Set(hrefs)])("%s has a page.tsx", (href) => {
    expect(fs.existsSync(path.join(process.cwd(), "src/app", href, "page.tsx"))).toBe(true);
  });

  it("the ComingSoon placeholder is gone", () => {
    expect(fs.existsSync(path.join(process.cwd(), "src/components/common/ComingSoon.tsx"))).toBe(false);
  });
});
