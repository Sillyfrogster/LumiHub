import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { nameSlot, orderSettings } from "./preset-slots";

describe("naming a preset slot", () => {
  test("a name Illarin knows reads as a person would say it", () => {
    expect(nameSlot("openai_max_tokens").name).toBe("Maximum reply length");
    expect(nameSlot("max_context_unlocked").name).toBe(
      "Unlock the context limit",
    );
  });

  test("the same setting under either app's spelling gets one name", () => {
    expect(nameSlot("top_p").name).toBe("Top P");
    expect(nameSlot("topP").name).toBe("Top P");
    expect(nameSlot("openai_max_context").name).toBe("Context size");
    expect(nameSlot("contextSize").name).toBe("Context size");
  });

  test("a setting whose name leaves a reader guessing carries a note", () => {
    expect(nameSlot("min_p").note).toBeTruthy();
  });

  test("a setting its own name explains carries none", () => {
    expect(nameSlot("enable_web_search").note).toBeUndefined();
  });

  test("a name Illarin does not know is kept as the file wrote it", () => {
    const unknown = nameSlot("marinara_secret_sauce");
    expect(unknown.name).toBe("marinara_secret_sauce");
    expect(unknown.rank).toBe("unrecognised");
    expect(unknown.note).toBeUndefined();
  });

  test("every slot carries the key the file uses", () => {
    expect(nameSlot("topK").key).toBe("topK");
    expect(nameSlot("mystery").key).toBe("mystery");
  });

  test("a key with no name in it is unrecognised", () => {
    expect(nameSlot("  ").rank).toBe("unrecognised");
  });
});

describe("ordering the settings a group shows", () => {
  const settings = [
    { name: "presence_penalty" },
    { name: "made_up" },
    { name: "openai_max_tokens" },
    { name: "temperature" },
  ];

  test("what a reader checks first comes first, and unknown names come last", () => {
    expect(orderSettings(settings).map((setting) => setting.slot.name)).toEqual(
      ["Maximum reply length", "Temperature", "Presence penalty", "made_up"],
    );
  });

  test("settings of one rank stay in the order the file wrote them", () => {
    const penalties = [
      { name: "repetition_penalty" },
      { name: "frequency_penalty" },
      { name: "presence_penalty" },
    ];
    expect(orderSettings(penalties).map((setting) => setting.name)).toEqual([
      "repetition_penalty",
      "frequency_penalty",
      "presence_penalty",
    ]);
  });
});

describe("the slots the two preset formats seed", () => {
  const source = readFileSync(
    join(import.meta.dir, "../../../api/internal/format/preset/slots.go"),
    "utf8",
  );
  const table = source.slice(
    source.indexOf("var slotsByApp"),
    source.indexOf("// Seed returns"),
  );
  const names = [...table.matchAll(/"([A-Za-z_][A-Za-z_0-9]*)"/g)].map(
    (match) => match[1],
  );

  test("the slot table was found", () => {
    expect(names.length).toBeGreaterThan(50);
  });

  test("every seeded slot has a name written for a person", () => {
    const missing = names.filter(
      (name) => nameSlot(name).rank === "unrecognised",
    );
    expect(missing).toEqual([]);
  });
});
