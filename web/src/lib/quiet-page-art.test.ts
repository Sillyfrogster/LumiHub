import { describe, expect, test } from "bun:test";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";

// The map itself is a Record over every kind, so TypeScript already refuses a
// kind without a piece. What it cannot see is the files on disk, which is what
// `make quiet-page-art` writes and what these tests hold to.

const ART_DIRECTORY = path.join(import.meta.dir, "../assets/art/full");
const KINDS = ["character", "lorebook", "preset", "theme", "pack"];
const THEMES = ["light", "dark"];

function artPath(kind: string, theme: string) {
  return path.join(
    ART_DIRECTORY,
    `illarin-quiet-page-${kind}-${theme}-v1.webp`,
  );
}

describe("the artwork a quiet page shows", () => {
  test("every kind has a piece composed for light and for dark", async () => {
    for (const kind of KINDS) {
      for (const theme of THEMES) {
        const file = await readFile(artPath(kind, theme));
        expect(file.byteLength).toBeGreaterThan(0);
      }
    }
  });

  test("no two pieces are the same image", async () => {
    const digests = await Promise.all(
      KINDS.flatMap((kind) =>
        THEMES.map(async (theme) =>
          createHash("sha256")
            .update(await readFile(artPath(kind, theme)))
            .digest("hex"),
        ),
      ),
    );
    expect(new Set(digests).size).toBe(digests.length);
  });
});
