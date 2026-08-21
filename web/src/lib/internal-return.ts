export function safeInternalReturnPath(
  value: string | string[] | null | undefined,
): string | undefined {
  const candidate = Array.isArray(value) ? value[0] : value;
  return candidate?.startsWith("/") &&
    !candidate.startsWith("//") &&
    !candidate.includes("\\") &&
    !Array.from(candidate).some((character) => {
      const code = character.charCodeAt(0);
      return code < 32 || code === 127;
    })
    ? candidate
    : undefined;
}
