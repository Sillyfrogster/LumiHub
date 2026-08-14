import { expect, test } from "bun:test";
import { assetHref, assetRedirect, assetSlug } from "./asset-url";

const ID = "0f6b7a4c-3d21-4a5e-9c8b-1f2e3d4c5b6a";
const CHRISSY = { id: ID, name: "Christine Novak" };

test("lowercases and joins words with single hyphens", () => {
  expect(assetSlug("The Quiet Archivist")).toBe("the-quiet-archivist");
});

test("drops combining marks left by decomposition", () => {
  expect(assetSlug("Chrissy Nóvak")).toBe("chrissy-novak");
  expect(assetSlug("Ångström")).toBe("angstrom");
});

test("collapses runs of punctuation and trims the ends", () => {
  expect(assetSlug("  ***Rain,  and No Umbrella!!!  ")).toBe(
    "rain-and-no-umbrella",
  );
});

test("has no slug for a name that normalizes to nothing", () => {
  expect(assetSlug("日本語")).toBe("");
  expect(assetSlug("!!!")).toBe("");
  expect(assetSlug("")).toBe("");
});

test("caps at sixty characters, preferring the last word boundary", () => {
  const slug = assetSlug(
    "A brilliant unassuming girl who spent years being overlooked entirely",
  );

  expect(slug).toBe("a-brilliant-unassuming-girl-who-spent-years-being");
  expect(slug.length).toBeLessThanOrEqual(60);
  expect(slug.endsWith("-")).toBe(false);
});

test("cuts mid-word only when the first sixty characters hold no boundary", () => {
  expect(assetSlug("a".repeat(80))).toBe("a".repeat(60));
});

test("transliterates nothing", () => {
  expect(assetSlug("Ярославль")).toBe("");
  expect(assetSlug("Straße")).toBe("stra-e");
});

test("addresses an asset by id, with the slug only when there is one", () => {
  expect(assetHref(ID, "The Quiet Archivist")).toBe(
    `/a/${ID}/the-quiet-archivist`,
  );
  expect(assetHref(ID, "日本語")).toBe(`/a/${ID}`);
});

test("sends a bare address to the slugged one", () => {
  expect(assetRedirect({ id: ID }, CHRISSY)).toBe(`/a/${ID}/christine-novak`);
});

test("sends a stale or wrong slug to the current one", () => {
  expect(assetRedirect({ id: ID, slug: ["chrissy-old-name"] }, CHRISSY)).toBe(
    `/a/${ID}/christine-novak`,
  );
  expect(assetRedirect({ id: ID, slug: ["christine", "novak"] }, CHRISSY)).toBe(
    `/a/${ID}/christine-novak`,
  );
});

test("leaves the canonical address alone", () => {
  expect(assetRedirect({ id: ID, slug: ["christine-novak"] }, CHRISSY)).toBe(
    null,
  );
});

test("makes the bare address canonical when there is no slug", () => {
  const unnamed = { id: ID, name: "日本語" };

  expect(assetRedirect({ id: ID }, unnamed)).toBe(null);
  expect(assetRedirect({ id: ID, slug: ["something"] }, unnamed)).toBe(
    `/a/${ID}`,
  );
});
