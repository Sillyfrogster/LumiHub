import { expect, test } from "bun:test";
import type { BrowseAsset, BrowsePage } from "@/lib/api/query";
import { buildSitemap } from "./sitemap";

const FIRST_ID = "11111111-1111-4111-8111-111111111111";
const SECOND_ID = "22222222-2222-4222-8222-222222222222";

function asset(id: string, name: string): BrowseAsset {
  return {
    id,
    name,
    creator: "garden.keeper",
    kind: "theme",
    isNsfw: false,
    cover: null,
  };
}

test("sitemap follows the whole browse listing", async () => {
  const requests: Array<{ before?: string; beforeId?: string }> = [];
  const pages: Array<Pick<BrowsePage, "items" | "nextCursor">> = [
    {
      items: [asset(FIRST_ID, "First garden")],
      nextCursor: {
        before: "2026-08-13T12:00:00Z",
        beforeId: FIRST_ID,
      },
    },
    {
      items: [asset(SECOND_ID, "Second garden")],
      nextCursor: null,
    },
  ];

  const entries = await buildSitemap(async (params) => {
    requests.push({ before: params.before, beforeId: params.beforeId });
    const page = pages.shift();
    if (!page) throw new Error("sitemap requested an extra page");
    return page;
  });

  expect(entries.map((entry) => entry.url)).toEqual([
    "http://localhost:8000/",
    "http://localhost:8000/browse",
    `http://localhost:8000/a/${FIRST_ID}/first-garden`,
    `http://localhost:8000/a/${SECOND_ID}/second-garden`,
  ]);
  expect(requests).toEqual([
    { before: undefined, beforeId: undefined },
    { before: "2026-08-13T12:00:00Z", beforeId: FIRST_ID },
  ]);
});
