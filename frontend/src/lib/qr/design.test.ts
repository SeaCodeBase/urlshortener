import { describe, expect, it } from "vitest";
import { DEFAULT_QR_DESIGN } from "@/types";
import { frameInsets, normalizeQrDesign, toStylingOptions } from "./design";

describe("normalizeQrDesign", () => {
  it("returns the defaults for an empty object", () => {
    expect(normalizeQrDesign({})).toEqual(DEFAULT_QR_DESIGN);
  });

  it("returns the defaults for non-objects", () => {
    expect(normalizeQrDesign(null)).toEqual(DEFAULT_QR_DESIGN);
    expect(normalizeQrDesign("x")).toEqual(DEFAULT_QR_DESIGN);
  });

  it("drops invalid hex colors, keeps valid enums and clamps logo_size", () => {
    const d = normalizeQrDesign({ color: "red", pattern: "circle", logo_size: 2 });
    expect(d.color).toBe(DEFAULT_QR_DESIGN.color);
    expect(d.pattern).toBe("circle");
    expect(d.logo_size).toBe(0.5);
  });

  it("clamps a small logo_size and ignores unknown enum values", () => {
    const d = normalizeQrDesign({ logo_size: 0.05, corners: "star", background: "#abc", logo: "/uploads/a.png" });
    expect(d.logo_size).toBe(0.2);
    expect(d.corners).toBe("square");
    expect(d.background).toBe("#abc");
    expect(d.logo).toBe("/uploads/a.png");
  });
});

describe("toStylingOptions", () => {
  it("maps the standard pattern to square dots", () => {
    expect(toStylingOptions(DEFAULT_QR_DESIGN, "x", 300).dotsOptions?.type).toBe("square");
  });

  it("maps circle to dots and passes logo, colors and error correction", () => {
    const o = toStylingOptions(
      { ...DEFAULT_QR_DESIGN, pattern: "circle", corners: "dot", logo: "/l.png", logo_size: 0.4, error_correction: "Q" },
      "https://s.io/a",
      300,
    );
    expect(o.dotsOptions?.type).toBe("dots");
    expect(o.cornersSquareOptions?.type).toBe("dot");
    expect(o.cornersDotOptions?.type).toBe("dot");
    expect(o.image).toBe("/l.png");
    expect(o.imageOptions).toMatchObject({ imageSize: 0.4, margin: 4, crossOrigin: "anonymous" });
    expect(o.qrOptions?.errorCorrectionLevel).toBe("Q");
    expect(o.data).toBe("https://s.io/a");
    expect(o.width).toBe(300);
  });
});

describe("frameInsets", () => {
  it("is zero without a frame", () => {
    expect(frameInsets(DEFAULT_QR_DESIGN, 100)).toEqual({ border: 0, textHeight: 0, top: 0, bottom: 0 });
  });

  it("scales the border with the size", () => {
    expect(frameInsets({ ...DEFAULT_QR_DESIGN, frame: "border" }, 200)).toEqual({ border: 8, textHeight: 0, top: 0, bottom: 0 });
    expect(frameInsets({ ...DEFAULT_QR_DESIGN, frame: "border" }, 1024)).toEqual({ border: 41, textHeight: 0, top: 0, bottom: 0 });
  });

  it("adds a text band at the bottom for text-bottom", () => {
    expect(frameInsets({ ...DEFAULT_QR_DESIGN, frame: "text-bottom" }, 100)).toEqual({ border: 4, textHeight: 18, top: 0, bottom: 18 });
  });

  it("adds a text band at the top for text-top", () => {
    expect(frameInsets({ ...DEFAULT_QR_DESIGN, frame: "text-top" }, 200)).toEqual({ border: 8, textHeight: 36, top: 36, bottom: 0 });
  });

  it("reserves no text band when the frame text is blank", () => {
    expect(frameInsets({ ...DEFAULT_QR_DESIGN, frame: "text-bottom", frame_text: "  " }, 100)).toEqual({ border: 4, textHeight: 0, top: 0, bottom: 0 });
  });
});
