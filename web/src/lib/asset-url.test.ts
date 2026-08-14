import { expect, test } from "bun:test";
import { assetHref, assetSlug } from "./asset-url";

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
  const id = "0f6b7a4c-3d21-4a5e-9c8b-1f2e3d4c5b6a";

  expect(assetHref(id, "The Quiet Archivist")).toBe(
    `/a/${id}/the-quiet-archivist`,
  );
  expect(assetHref(id, "日本語")).toBe(`/a/${id}`);
});
