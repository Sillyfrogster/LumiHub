import { describe, expect, test } from "bun:test";
import { pickerColor, themeAccent, themeColorName } from "./theme-colors";

describe("theme colours", () => {
  test("names SillyTavern palette keys for a reader", () => {
    expect(themeColorName("main_text_color")).toBe("Main text");
    expect(themeColorName("future_colour")).toBe("Future colour");
  });

  test("gives the native picker the colour behind hex and rgba values", () => {
    expect(pickerColor("#abc")).toBe("#aabbcc");
    expect(pickerColor("rgba(48, 38, 65, 0.9)")).toBe("#302641");
  });

  test("turns Lumiverse's HSL object into a swatch and readable value", () => {
    expect(themeAccent('{"h":262,"s":64,"l":62}')).toEqual({
      css: "hsl(262 64% 62%)",
      label: "262° · 64% · 62%",
    });
    expect(themeAccent("not JSON")).toBeNull();
  });
});
