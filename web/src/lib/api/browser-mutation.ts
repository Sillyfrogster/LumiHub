export const BROWSER_MUTATION_HEADER = "X-Illarin-Request";

const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

export function markBrowserMutation(method: string, headers: Headers) {
  if (!SAFE_METHODS.has(method.toUpperCase())) {
    headers.set(BROWSER_MUTATION_HEADER, "1");
  }
}

export function browserFetch(input: RequestInfo | URL, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  markBrowserMutation(init.method ?? "GET", headers);
  return fetch(input, { ...init, headers });
}
