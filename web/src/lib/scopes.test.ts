import { expect, test } from "bun:test";
import { describeScope, type Scope } from "./scopes";

test("describes each scope a creator can be asked to approve", () => {
  for (const scope of ["asset:receive", "library:sync"] as Scope[]) {
    const copy = describeScope(scope);
    expect(copy.title).not.toBe(scope);
    expect(copy.detail.length).toBeGreaterThan(0);
  }
});

test("shows an unrecognised scope as it was named", () => {
  const copy = describeScope("asset:destroy" as Scope);

  expect(copy.title).toBe("asset:destroy");
  expect(copy.detail).toContain("does not recognise");
});
