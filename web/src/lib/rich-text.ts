import type { PhrasingContent, RootContent } from "mdast";
import { fromMarkdown } from "mdast-util-from-markdown";

export type RichInline =
  | { kind: "text"; text: string }
  | { kind: "break" }
  | { kind: "code"; text: string }
  | { kind: "emphasis"; children: RichInline[] }
  | { kind: "strong"; children: RichInline[] }
  | { kind: "link"; href: string; children: RichInline[] };

export type RichBlock =
  | { kind: "paragraph"; children: RichInline[] }
  /** Depth is 1 for the shallowest heading in the text, whatever it was typed
   * as, so a page never skips a heading level. */
  | { kind: "heading"; depth: number; children: RichInline[] }
  | { kind: "quote"; children: RichBlock[] }
  | { kind: "list"; ordered: boolean; start: number; items: RichBlock[][] };

export type RichText = {
  blocks: RichBlock[];
  /** True where the words survived but the way they were dressed did not. */
  formattingRemoved: boolean;
};

/**
 * Reads what a creator wrote as emphasis, headings, lists, links, block quotes
 * and inline code, and nothing else. HTML is reduced to the words inside it, so
 * a greeting wrapped in markup shows what it says. The stored text is never
 * touched and a download still carries every original byte.
 */
export function readRichText(source: string): RichText {
  const stripped = stripHtml(source);
  const removed = { formatting: stripped.removed };
  const tree = fromMarkdown(stripped.text, { extensions: [DISABLED] });
  const blocks = readBlocks(tree.children, removed);
  const shallowest = shallowestHeading(blocks) ?? 1;
  return {
    blocks: shallowest > 1 ? raiseHeadings(blocks, shallowest - 1) : blocks,
    formattingRemoved: removed.formatting,
  };
}

/** The depth of the shallowest heading anywhere in these blocks, or null. */
function shallowestHeading(blocks: RichBlock[]): number | null {
  const depths: number[] = [];
  for (const block of blocks) {
    if (block.kind === "heading") depths.push(block.depth);
    if (block.kind === "quote") {
      const inner = shallowestHeading(block.children);
      if (inner !== null) depths.push(inner);
    }
    if (block.kind === "list") {
      for (const item of block.items) {
        const inner = shallowestHeading(item);
        if (inner !== null) depths.push(inner);
      }
    }
  }
  return depths.length === 0 ? null : Math.min(...depths);
}

function raiseHeadings(blocks: RichBlock[], by: number): RichBlock[] {
  return blocks.map((block) => {
    if (block.kind === "heading") return { ...block, depth: block.depth - by };
    if (block.kind === "quote") {
      return { ...block, children: raiseHeadings(block.children, by) };
    }
    if (block.kind === "list") {
      return {
        ...block,
        items: block.items.map((item) => raiseHeadings(item, by)),
      };
    }
    return block;
  });
}

/** Whether any of these texts loses formatting on the way to the page. */
export function formattingWasRemoved(texts: readonly string[]): boolean {
  return texts.some((text) => readRichText(text).formattingRemoved);
}

/**
 * The text on an element that a person wrote for a person. A body a creator
 * marked verbatim is left out, and so is a prompt fragment, because both are
 * the exact words an app sends to a model rather than prose about them.
 */
export function richTextsOf(element: {
  type: string;
  display?: string;
  content: unknown;
}): string[] {
  if (!element.content || typeof element.content !== "object") return [];
  const content = element.content as Record<string, unknown>;
  const verbatim = element.display === "verbatim";
  switch (element.type) {
    case "prose":
      return verbatim ? [] : texts([content], "text");
    case "text_set":
      return verbatim ? [] : texts(items(content.texts), "text");
    case "dialogue_sample":
      return texts(items(content.turns), "text");
    case "entry_table":
      return texts(items(content.entries), "text");
    case "field_list":
      return texts(items(content.fields), "value");
    case "link_list":
      return texts(items(content.links), "note");
    case "variable_schema":
      return texts(items(content.variables), "description");
    default:
      return [];
  }
}

function items(value: unknown): Record<string, unknown>[] {
  return Array.isArray(value) ? (value as Record<string, unknown>[]) : [];
}

function texts(entries: Record<string, unknown>[], key: string): string[] {
  const found: string[] = [];
  for (const entry of entries) {
    const value = entry?.[key];
    if (typeof value === "string" && value.trim() !== "") found.push(value);
  }
  return found;
}

/**
 * The markdown this page never reads. HTML is off outright, so no HTML node can
 * reach the renderer. Indented lines stay prose, because a creator who indents
 * a paragraph means a paragraph. A rule stays the characters it was typed as,
 * so nobody loses their scene break.
 */
const DISABLED = {
  disable: { null: ["codeIndented", "htmlFlow", "htmlText", "thematicBreak"] },
};

type Removed = { formatting: boolean };

function readBlocks(nodes: RootContent[], removed: Removed): RichBlock[] {
  const blocks: RichBlock[] = [];
  for (const node of nodes) {
    switch (node.type) {
      case "paragraph": {
        const children = readInline(node.children, removed);
        if (!isBlank(children)) blocks.push({ kind: "paragraph", children });
        break;
      }
      case "heading": {
        const children = readInline(node.children, removed);
        if (!isBlank(children)) {
          blocks.push({ kind: "heading", depth: node.depth, children });
        }
        break;
      }
      case "blockquote": {
        const children = readBlocks(node.children, removed);
        if (children.length > 0) blocks.push({ kind: "quote", children });
        break;
      }
      case "list": {
        const listItems = node.children
          .map((item) => readBlocks(item.children, removed))
          .filter((item) => item.length > 0);
        if (listItems.length > 0) {
          blocks.push({
            kind: "list",
            ordered: node.ordered ?? false,
            start: node.start ?? 1,
            items: listItems,
          });
        }
        break;
      }
      case "code": {
        removed.formatting = true;
        blocks.push({ kind: "paragraph", children: readLines(node.value) });
        break;
      }
      case "definition":
        break;
      default: {
        removed.formatting = true;
        const words = flatten(node);
        if (words.trim() !== "") {
          blocks.push({ kind: "paragraph", children: readLines(words) });
        }
      }
    }
  }
  return blocks;
}

function readInline(nodes: PhrasingContent[], removed: Removed): RichInline[] {
  const children: RichInline[] = [];
  for (const node of nodes) {
    switch (node.type) {
      case "text":
        children.push(...readLines(node.value));
        break;
      case "emphasis":
        children.push({
          kind: "emphasis",
          children: readInline(node.children, removed),
        });
        break;
      case "strong":
        children.push({
          kind: "strong",
          children: readInline(node.children, removed),
        });
        break;
      case "inlineCode":
        children.push({ kind: "code", text: node.value });
        break;
      case "break":
        children.push({ kind: "break" });
        break;
      case "link": {
        const href = readHref(node.url);
        const inner = readInline(node.children, removed);
        if (href) {
          children.push({ kind: "link", href, children: inner });
        } else {
          removed.formatting = true;
          children.push(...inner);
        }
        break;
      }
      case "image":
      case "imageReference":
        removed.formatting = true;
        children.push(...readLines(node.alt ?? ""));
        break;
      case "linkReference":
        removed.formatting = true;
        children.push(...readInline(node.children, removed));
        break;
      default:
        removed.formatting = true;
        children.push(...readLines(flatten(node)));
    }
  }
  return children;
}

/** Newlines survive as line breaks, because a creator typing one meant one. */
function readLines(value: string): RichInline[] {
  const children: RichInline[] = [];
  value.split("\n").forEach((part, index) => {
    if (index > 0) children.push({ kind: "break" });
    if (part !== "") children.push({ kind: "text", text: part });
  });
  return children;
}

function isBlank(children: RichInline[]): boolean {
  return children.every((child) => {
    if (child.kind === "break") return true;
    if (child.kind === "text" || child.kind === "code") {
      return child.text.trim() === "";
    }
    if (child.kind === "link") return false;
    return isBlank(child.children);
  });
}

function flatten(node: unknown): string {
  if (!node || typeof node !== "object") return "";
  const branch = node as { value?: unknown; children?: unknown };
  if (typeof branch.value === "string") return branch.value;
  if (!Array.isArray(branch.children)) return "";
  return branch.children.map(flatten).join("");
}

const FOLLOWABLE_SCHEMES = new Set(["http", "https", "mailto"]);

/**
 * The address a link is allowed to carry. A scheme the page cannot follow, such
 * as javascript, is refused, and so is an address starting with two slashes,
 * which leaves the site without naming where it goes.
 */
function readHref(url: string): string | null {
  // Control characters and spaces come out first, so nothing hides a scheme.
  const cleaned = url.replace(/[\s\p{Cc}]/gu, "");
  if (cleaned === "") return null;
  const scheme = /^([a-zA-Z][a-zA-Z0-9+.-]*):/.exec(cleaned);
  if (scheme) {
    return FOLLOWABLE_SCHEMES.has(scheme[1].toLowerCase()) ? cleaned : null;
  }
  return cleaned.startsWith("/") && !cleaned.startsWith("//") ? cleaned : null;
}

/** Elements whose own content is not words anybody came to read. */
const DISCARDED = /<(script|style|svg)\b[^>]*>[\s\S]*?(?:<\/\1\s*>|$)/gi;

const COMMENT_OR_TAG =
  /<!--[\s\S]*?-->|<\/?([a-zA-Z][a-zA-Z0-9-]*)(?:\s(?:"[^"]*"|'[^']*'|[^'">])*)?\/?>/g;

/**
 * Removes the markup a page is dressed in and keeps the words. Only names HTML
 * itself defines count as markup: a creator writing `<personality>` in a
 * prompt, or `a < b` in a sentence, wrote text and gets text back.
 */
function stripHtml(source: string): { text: string; removed: boolean } {
  let removed = false;
  let text = source.replace(DISCARDED, () => {
    removed = true;
    return "";
  });
  text = text.replace(COMMENT_OR_TAG, (match: string, name?: string) => {
    if (name === undefined) {
      removed = true;
      return "";
    }
    const tag = name.toLowerCase();
    if (!HTML_ELEMENTS.has(tag)) return match;
    removed = true;
    return BLOCK_ELEMENTS.has(tag) ? "\n" : "";
  });
  if (!removed) return { text: source, removed: false };
  // The indentation belonged to the markup, not to the words inside it.
  const flat = text
    .split("\n")
    .map((line) => line.trim())
    .join("\n");
  return { text: flat.replace(/\n{3,}/g, "\n\n").trim(), removed };
}

const HTML_ELEMENTS = new Set(
  `a abbr acronym address applet area article aside audio b base basefont bdi
   bdo bgsound big blink blockquote body br button canvas caption center cite
   code col colgroup data datalist dd del details dfn dialog dir div dl dt em
   embed fieldset figcaption figure font footer form frame frameset h1 h2 h3 h4
   h5 h6 head header hgroup hr html i iframe image img input ins kbd keygen
   label legend li link listing main map mark marquee math menu menuitem meta
   meter nav nobr noembed noframes noscript object ol optgroup option output p
   param picture plaintext pre progress q rb rp rt rtc ruby s samp script search
   section select slot small source spacer span strike strong style sub summary
   sup svg table tbody td template textarea tfoot th thead time title tr track
   tt u ul var video wbr xmp`.split(/\s+/),
);

/** The elements that end a line of words rather than sitting inside one. */
const BLOCK_ELEMENTS = new Set(
  `address article aside blockquote br caption center dd details dialog dir div
   dl dt fieldset figcaption figure footer form h1 h2 h3 h4 h5 h6 header hgroup
   hr legend li main marquee menu nav ol p pre section summary table tbody td
   tfoot th thead tr ul`.split(/\s+/),
);
