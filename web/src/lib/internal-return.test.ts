import { describe, expect, test } from "bun:test";
import { safeInternalReturnPath } from "./internal-return";

describe("safeInternalReturnPath", () => {
  test("keeps an internal path with a query", () => {
    expect(safeInternalReturnPath("/link?code=ABCD-1234")).toBe(
      "/link?code=ABCD-1234",
    );
  });

  test("rejects protocol-relative and absolute destinations", () => {
    expect(safeInternalReturnPath("//outside.example/path")).toBeUndefined();
    expect(
      safeInternalReturnPath("https://outside.example/path"),
    ).toBeUndefined();
    expect(safeInternalReturnPath("/\\outside.example/path")).toBeUndefined();
    expect(
      safeInternalReturnPath("/link\nLocation: //outside.example"),
    ).toBeUndefined();
  });

  test("uses the first value supplied by a route query", () => {
    expect(safeInternalReturnPath(["/browse", "/settings"])).toBe("/browse");
  });
});
