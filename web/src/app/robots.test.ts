import { expect, test } from "bun:test";
import { buildRobots } from "./robots";

const robots = buildRobots("https://illarin.xyz");
const rules = Array.isArray(robots.rules) ? robots.rules[0] : robots.rules;
const disallowed = [rules.disallow ?? []].flat();

test("points crawlers at the sitemap", () => {
  expect(robots.sitemap).toBe("https://illarin.xyz/sitemap.xml");
});

test("keeps crawlers off the API, downloads and the link handover", () => {
  expect(disallowed).toContain("/api/");
  expect(disallowed).toContain("/download/");
  expect(disallowed).toContain("/link");
});

test("keeps crawlers off the pages that need an account or carry a token", () => {
  for (const path of [
    "/settings",
    "/upload",
    "/sign-in",
    "/sign-up",
    "/verify-email",
    "/forgot-password",
    "/reset-password",
  ]) {
    expect(disallowed).toContain(path);
  }
});

test("leaves asset pages crawlable, because an unlisted one answers with noindex", () => {
  expect(rules.allow).toBe("/");
  for (const path of disallowed) {
    expect("/a/".startsWith(path)).toBe(false);
  }
});

test("leaves media crawlable, so a link preview can fetch its image", () => {
  for (const path of disallowed) {
    expect("/media/".startsWith(path)).toBe(false);
  }
});
