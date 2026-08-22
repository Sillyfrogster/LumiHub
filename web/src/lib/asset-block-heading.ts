type HeadingElement = {
  type: string;
  isEmpty: boolean;
  facts: string[];
};

type Total = {
  count: number;
  singular?: string;
  plural?: string;
};

const COUNTED = /^([\d,]+) (.+)$/;

export function blockCounts(elements: readonly HeadingElement[]): string {
  const speaking = elements.filter(
    (element) => !element.isEmpty && element.facts.length > 0,
  );
  if (speaking.length === 0) return "";
  if (speaking.length === 1) return speaking[0].facts.join(" · ");

  const totals = new Map<string, Total>();
  for (const element of speaking) {
    const [size] = element.facts;
    const counted = COUNTED.exec(size);
    if (!counted) return speaking.flatMap((item) => item.facts[0]).join(" · ");
    const count = Number(counted[1].replaceAll(",", ""));
    const noun = counted[2];
    const running = totals.get(nounStem(noun)) ?? { count: 0 };
    running.count += count;
    if (count === 1) running.singular = noun;
    else running.plural = noun;
    totals.set(nounStem(noun), running);
  }

  const written: string[] = [];
  for (const total of totals.values()) {
    const noun = total.count === 1 ? total.singular : total.plural;
    if (!noun) return speaking.map((element) => element.facts[0]).join(" · ");
    written.push(`${writeCount(total.count)} ${noun}`);
  }
  return written.join(" · ");
}

export function isTabularBlock(elements: readonly HeadingElement[]): boolean {
  return elements.some(
    (element) => element.type === "entry_table" && !element.isEmpty,
  );
}

function nounStem(noun: string): string {
  if (noun.endsWith("ies")) return `${noun.slice(0, -3)}y`;
  return noun.endsWith("s") ? noun.slice(0, -1) : noun;
}

function writeCount(count: number): string {
  let digits = String(count);
  for (let cut = digits.length - 3; cut > 0; cut -= 3) {
    digits = `${digits.slice(0, cut)},${digits.slice(cut)}`;
  }
  return digits;
}
