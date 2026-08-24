import { describe, expect, test } from "bun:test";
import {
  BROWSER_MUTATION_HEADER,
  markBrowserMutation,
} from "./browser-mutation";

describe("browser mutation marker", () => {
  test("marks unsafe methods without replacing their headers", () => {
    const headers = new Headers({ "Content-Type": "application/json" });
    markBrowserMutation("patch", headers);

    expect(headers.get(BROWSER_MUTATION_HEADER)).toBe("1");
    expect(headers.get("Content-Type")).toBe("application/json");
  });

  test("does not mark safe methods", () => {
    const headers = new Headers();
    markBrowserMutation("GET", headers);

    expect(headers.has(BROWSER_MUTATION_HEADER)).toBe(false);
  });
});
