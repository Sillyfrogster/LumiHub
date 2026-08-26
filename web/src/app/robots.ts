import type { MetadataRoute } from "next";
import { siteUrl } from "@/lib/site-metadata";

export const dynamic = "force-dynamic";

/** Asset pages stay crawlable on purpose. An unlisted one asks not to be indexed with a tag, and a crawler has to read the page to find it. */
export function buildRobots(origin: string): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: [
        "/api/",
        "/download/",
        "/link",
        "/settings",
        "/upload",
        "/sign-in",
        "/sign-up",
        "/verify-email",
        "/forgot-password",
        "/reset-password",
      ],
    },
    sitemap: new URL("/sitemap.xml", origin).href,
  };
}

export default function robots(): MetadataRoute.Robots {
  return buildRobots(siteUrl);
}
