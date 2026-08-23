import { describe, expect, test } from "bun:test";
import { buildBrowseHref, readBrowseFilters } from "./browse-url";

describe("browse URL state", () => {
  test("round-trips the complete search expression through one q parameter", () => {
    const expression = '  moon tag:"original character" mood:gentle  ';
    const href = buildBrowseHref({
      kind: "character",
      platform: "raw",
      q: expression,
      facet: ["tone=gentle"],
    });
    const url = new URL(href, "https://illarin.test");

    expect(url.searchParams.getAll("q")).toEqual([expression]);
    expect(readBrowseFilters(Object.fromEntries(url.searchParams))).toEqual({
      kind: "character",
      platform: "raw",
      q: expression,
      facet: ["tone=gentle"],
    });
  });

  test("keeps creator profile filters on the creator profile", () => {
    expect(
      buildBrowseHref({ kind: "lorebook", q: "moonlit" }, "/@verified.creator"),
    ).toBe("/@verified.creator?kind=lorebook&q=moonlit");
  });

  test("keeps Pack as a catalog kind", () => {
    expect(readBrowseFilters({ kind: "pack" }).kind).toBe("pack");
    expect(buildBrowseHref({ kind: "pack" })).toBe("/browse?kind=pack");
  });
});
