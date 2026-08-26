import { expect, test } from "bun:test";
import type { AssetDetail } from "./api/query";
import { assetMetadata } from "./asset-metadata";
import { SITE_CARD } from "./site-metadata";

const ID = "0f6b7a4c-3d21-4a5e-9c8b-1f2e3d4c5b6a";

function asset(over: Partial<AssetDetail> = {}): AssetDetail {
  return {
    id: ID,
    kind: "character",
    name: "Christine Novak",
    blurb: "She closes the book on a ribbon.",
    tags: [],
    creator: "nimhloth",
    isNsfw: false,
    discovery: "listed",
    lifecycle: "published",
    isOwner: false,
    linkedInstallOnly: false,
    allowedApps: [],
    eligibleApps: [],
    downloads: [],
    original: null,
    createdAt: "2026-08-13T00:00:00Z",
    blocks: [],
    media: [],
    preview: "/media/aaaa/og/1",
    visibility: "blurred",
    ...over,
  };
}

test("calls an asset with no name Untitled", () => {
  const meta = assetMetadata(asset({ name: "", blurb: "" }));

  expect(meta.title).toBe("Untitled · Character");
});

test("names the asset and its kind, and pitches it with the blurb", () => {
  const meta = assetMetadata(asset());

  expect(meta.title).toBe("Christine Novak · Character");
  expect(meta.description).toBe("She closes the book on a ribbon.");
  expect(meta.openGraph?.title).toBe("Christine Novak · Character");
  expect(meta.twitter && "card" in meta.twitter && meta.twitter.card).toBe(
    "summary_large_image",
  );
});

test("points the preview at the canonical address", () => {
  const meta = assetMetadata(asset());
  const canonical = `/a/${ID}/christine-novak`;

  expect(meta.alternates?.canonical).toBe(canonical);
  expect(meta.openGraph && "url" in meta.openGraph && meta.openGraph.url).toBe(
    canonical,
  );
});

test("uses the composed social preview, and the site card without it", () => {
  expect(assetMetadata(asset()).openGraph?.images).toEqual([
    {
      url: "/media/aaaa/og/1",
      alt: "Christine Novak",
      width: 1200,
      height: 630,
    },
  ]);
  expect(assetMetadata(asset({ preview: null })).openGraph?.images).toEqual([
    SITE_CARD,
  ]);
});

test("stands in a description when the creator wrote no blurb", () => {
  expect(assetMetadata(asset({ blurb: "" })).description).toBe(
    "A character by nimhloth.",
  );
});

test("asks not to be indexed while unlisted, and still invites following", () => {
  expect(assetMetadata(asset({ discovery: "unlisted" })).robots).toEqual({
    index: false,
  });
  expect(assetMetadata(asset()).robots).toBeUndefined();
});

test("does not copy protected prompt text into page or social metadata", () => {
  const privateText = "metadata-disclosure-canary-1c7ed05b";
  const metadata = assetMetadata(
    asset({
      kind: "preset",
      linkedInstallOnly: true,
      allowedApps: ["lumiverse"],
      blocks: [
        {
          id: ID,
          definition: "preset_core",
          title: "Prompt",
          titleIsDefault: true,
          position: 0,
          hidden: false,
          layout: "single",
          width: "full",
          allowedLayouts: ["single"],
          required: true,
          hideable: false,
          isEmpty: false,
          elements: [
            {
              id: ID,
              type: "prompt_list",
              role: "prompt_fragments",
              slot: "main",
              label: "Prompt fragments",
              pinned: true,
              isEmpty: false,
              facts: ["1 fragment"],
              content: {
                groups: [],
                fragments: [
                  {
                    id: ID,
                    name: "Private instructions",
                    role: "system",
                    text: privateText,
                    protected: true,
                    enabled: true,
                  },
                ],
              },
            },
          ],
        },
      ],
    }),
  );

  expect(JSON.stringify(metadata)).not.toContain(privateText);
  expect(metadata.openGraph?.description).toBe(
    "She closes the book on a ribbon.",
  );
});
