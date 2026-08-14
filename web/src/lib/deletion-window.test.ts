import { describe, expect, test } from "bun:test";
import { remainingDeletionWindow } from "./deletion-window";

describe("remainingDeletionWindow", () => {
  test("shows whole days while at least a day remains", () => {
    expect(
      remainingDeletionWindow(
        "2026-09-13T12:00:00Z",
        new Date("2026-08-14T13:00:00Z"),
      ),
    ).toBe("30 days remaining");
  });

  test("shows hours on the final day", () => {
    expect(
      remainingDeletionWindow(
        "2026-08-14T17:30:00Z",
        new Date("2026-08-14T13:00:00Z"),
      ),
    ).toBe("5 hours remaining");
  });

  test("does not promise recovery after the deadline", () => {
    expect(
      remainingDeletionWindow(
        "2026-08-14T12:00:00Z",
        new Date("2026-08-14T13:00:00Z"),
      ),
    ).toBe("Recovery window ended");
  });
});
