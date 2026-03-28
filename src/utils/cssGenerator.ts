/**
 * Simple CSS string manipulation for the beginner mode visual editor.
 */

interface ParsedRule {
  selector: string;
  properties: Map<string, string>;
  startIndex: number;
  endIndex: number;
}

function parseRules(css: string): ParsedRule[] {
  const rules: ParsedRule[] = [];
  let depth = 0;
  let ruleStart = -1;
  let braceStart = -1;

  for (let i = 0; i < css.length; i++) {
    const ch = css[i];
    if (ch === '{') {
      if (depth === 0) {
        ruleStart = findRuleStart(css, i);
        braceStart = i;
      }
      depth++;
    } else if (ch === '}') {
      depth--;
      if (depth === 0 && ruleStart >= 0) {
        const selector = css.slice(ruleStart, braceStart).trim();
        const body = css.slice(braceStart + 1, i).trim();

        if (!selector.startsWith('@')) {
          const properties = parseProperties(body);
          rules.push({ selector, properties, startIndex: ruleStart, endIndex: i + 1 });
        }

        ruleStart = -1;
        braceStart = -1;
      }
    }
  }

  return rules;
}

function findRuleStart(css: string, braceIndex: number): number {
  let i = braceIndex - 1;
  while (i >= 0 && /\s/.test(css[i])) i--;
  while (i >= 0 && css[i] !== '}' && css[i] !== ';') i--;
  i++;
  while (i < braceIndex && /\s/.test(css[i])) i++;
  return i;
}

function parseProperties(body: string): Map<string, string> {
  const props = new Map<string, string>();
  const declarations = body.split(';').filter((d) => d.trim().length > 0);

  for (const decl of declarations) {
    const colonIndex = decl.indexOf(':');
    if (colonIndex === -1) continue;
    const prop = decl.slice(0, colonIndex).trim();
    const value = decl.slice(colonIndex + 1).trim();
    if (prop && value) {
      props.set(prop, value);
    }
  }

  return props;
}

function buildRule(selector: string, properties: Map<string, string>): string {
  if (properties.size === 0) return '';
  const body = Array.from(properties.entries())
    .map(([prop, val]) => `  ${prop}: ${val};`)
    .join('\n');
  return `${selector} {\n${body}\n}`;
}

/**
 * Insert or update a CSS property within a rule for the given selector.
 * If the rule doesn't exist, appends a new one.
 */
export function upsertCssRule(css: string, selector: string, property: string, value: string): string {
  const rules = parseRules(css);
  const existing = rules.find((r) => r.selector === selector);

  if (existing) {
    existing.properties.set(property, value);
    const newRule = buildRule(existing.selector, existing.properties);
    return css.slice(0, existing.startIndex) + newRule + css.slice(existing.endIndex);
  }

  const newRule = buildRule(selector, new Map([[property, value]]));
  const separator = css.trim().length > 0 ? '\n\n' : '';
  return css + separator + newRule;
}

/**
 * Remove a CSS property from a rule. If the rule becomes empty, removes the entire rule.
 */
export function removeCssProperty(css: string, selector: string, property: string): string {
  const rules = parseRules(css);
  const existing = rules.find((r) => r.selector === selector);

  if (!existing) return css;

  existing.properties.delete(property);
  if (existing.properties.size === 0) {
    let start = existing.startIndex;
    let end = existing.endIndex;
    while (start > 0 && css[start - 1] === '\n') start--;
    while (end < css.length && css[end] === '\n') end++;
    return css.slice(0, start) + css.slice(end);
  }

  const newRule = buildRule(existing.selector, existing.properties);
  return css.slice(0, existing.startIndex) + newRule + css.slice(existing.endIndex);
}

/**
 * Get the current value of a CSS property for a selector, or null if not set.
 */
export function getCssPropertyValue(css: string, selector: string, property: string): string | null {
  const rules = parseRules(css);
  const existing = rules.find((r) => r.selector === selector);
  if (!existing) return null;
  return existing.properties.get(property) ?? null;
}
