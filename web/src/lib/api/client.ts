import createClient from "openapi-fetch";
import type { paths } from "./schema";

const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export const api = createClient<paths>({ baseUrl });

export type Asset =
  paths["/v1/assets"]["get"]["responses"]["200"]["content"]["application/json"]["items"][number];
