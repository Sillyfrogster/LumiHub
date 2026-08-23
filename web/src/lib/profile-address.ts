const HANDLE = /^[a-z0-9._]{3,32}$/;

const PUNCTUATION_ONLY = /^[._]+$/;

export type ProfileAddress = {
  form: "canonical" | "legacy";
  handle: string;
};

/** Which profile address a root segment is. `@handle` serves, a bare handle is v1's and only redirects. */
export function readProfileAddress(segment: string): ProfileAddress | null {
  const isCanonical = segment.startsWith("@");
  const handle = (isCanonical ? segment.slice(1) : segment).toLowerCase();
  if (!HANDLE.test(handle) || PUNCTUATION_ONLY.test(handle)) return null;
  return { form: isCanonical ? "canonical" : "legacy", handle };
}
