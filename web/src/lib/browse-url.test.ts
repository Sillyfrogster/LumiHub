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
    const url = new URL(href, "https://lumihub.test");

    expect(url.searchParams.getAll("q")).toEqual([expression]);
    expect(readBrowseFilters(Object.fromEntries(url.searchParams))).toEqual({
      kind: "character",
      platform: "raw",
      q: expression,
      facet: ["tone=gentle"],
    });
  });
});
