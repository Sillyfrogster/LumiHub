import { expect, test } from "bun:test";
import { readProfileAddress } from "./profile-address";

test("reads the canonical address", () => {
  expect(readProfileAddress("@garden.keeper")).toEqual({
    form: "canonical",
    handle: "garden.keeper",
  });
});

test("reads v1's bare address as the legacy form", () => {
  expect(readProfileAddress("garden.keeper")).toEqual({
    form: "legacy",
    handle: "garden.keeper",
  });
});

test("lowercases the handle in either form", () => {
  expect(readProfileAddress("@Garden.Keeper")?.handle).toBe("garden.keeper");
  expect(readProfileAddress("Garden.Keeper")?.handle).toBe("garden.keeper");
});

test("reads an all-digit handle, which three live accounts use", () => {
  expect(readProfileAddress("@314159")).toEqual({
    form: "canonical",
    handle: "314159",
  });
});

test("is no address at all when the handle could not exist", () => {
  expect(readProfileAddress("@")).toBe(null);
  expect(readProfileAddress("")).toBe(null);
  expect(readProfileAddress("no")).toBe(null);
  expect(readProfileAddress("a".repeat(33))).toBe(null);
  expect(readProfileAddress("garden keeper")).toBe(null);
  expect(readProfileAddress("...")).toBe(null);
});
