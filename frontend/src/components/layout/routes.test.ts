import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { SETTINGS_NAV } from "@/components/settings/SettingsNav";
import { NAV_ITEMS } from "./Sidebar";

const hrefs = [...NAV_ITEMS.map((i) => i.href), ...SETTINGS_NAV.flatMap((g) => g.items.map((i) => i.href))];

describe("navigation route coverage", () => {
  it.each(hrefs)("%s has a page.tsx", (href) => {
    expect(fs.existsSync(path.join(process.cwd(), "src/app", href, "page.tsx"))).toBe(true);
  });
});
