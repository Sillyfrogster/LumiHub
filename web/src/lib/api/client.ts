import createClient from "openapi-fetch";
import { markBrowserMutation } from "./browser-mutation";
import type { paths } from "./schema";

const baseUrl =
  typeof window === "undefined"
    ? (process.env.API_URL ?? "http://localhost:8080")
    : "/api";

export const api = createClient<paths>({ baseUrl });

api.use({
  onRequest({ request }) {
    markBrowserMutation(request.method, request.headers);
    return request;
  },
});

export type Asset =
  paths["/v1/assets"]["get"]["responses"]["200"]["content"]["application/json"]["items"][number];
