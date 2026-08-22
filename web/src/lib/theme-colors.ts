const THEME_COLOR_NAMES: Record<string, string> = {
  main_text_color: "Main text",
  italics_text_color: "Italic text",
  underline_text_color: "Underlined text",
  quote_text_color: "Quoted text",
  blur_tint_color: "Blur tint",
  chat_tint_color: "Chat tint",
  user_mes_blur_tint_color: "User message tint",
  bot_mes_blur_tint_color: "Character message tint",
  shadow_color: "Shadow",
  border_color: "Border",
};

export function themeColorName(name: string): string {
  return (
    THEME_COLOR_NAMES[name] ??
    name.replaceAll("_", " ").replace(/^./, (letter) => letter.toUpperCase())
  );
}

export function pickerColor(value: string): string {
  if (/^#[0-9a-f]{6}$/i.test(value)) return value;
  const short = /^#([0-9a-f])([0-9a-f])([0-9a-f])$/i.exec(value);
  if (short) {
    return `#${short[1]}${short[1]}${short[2]}${short[2]}${short[3]}${short[3]}`;
  }
  const rgb =
    /^rgba?\(\s*(\d+(?:\.\d+)?)\s*,\s*(\d+(?:\.\d+)?)\s*,\s*(\d+(?:\.\d+)?)/i.exec(
      value,
    );
  if (!rgb) return "#7c5cff";
  return `#${rgb
    .slice(1, 4)
    .map((part) =>
      Math.max(0, Math.min(255, Number(part)))
        .toString(16)
        .padStart(2, "0"),
    )
    .join("")}`;
}

export function themeAccent(
  text: string | undefined,
): { css: string; label: string } | null {
  if (!text) return null;
  try {
    const value = JSON.parse(text) as { h?: unknown; s?: unknown; l?: unknown };
    if (
      typeof value.h !== "number" ||
      typeof value.s !== "number" ||
      typeof value.l !== "number"
    ) {
      return null;
    }
    return {
      css: `hsl(${value.h} ${value.s}% ${value.l}%)`,
      label: `${value.h}° · ${value.s}% · ${value.l}%`,
    };
  } catch {
    return null;
  }
}
