import type { Metadata, Viewport } from "next";
import { Bodoni_Moda, Manrope } from "next/font/google";
import { ArtFilters } from "@/components/art/ArtFilters";
import { SiteFooter } from "@/components/layout/SiteFooter";
import { SiteHeader } from "@/components/layout/SiteHeader";
import {
  SITE_DESCRIPTION,
  SITE_NAME,
  siteOpenGraph,
  siteTwitter,
  siteUrl,
} from "@/lib/site-metadata";
import { THEME_BOOTSTRAP_SCRIPT } from "@/lib/theme";
import { Providers } from "./providers";
import "./globals.css";

const bodoni = Bodoni_Moda({
  variable: "--font-bodoni",
  subsets: ["latin"],
  style: ["normal", "italic"],
});

const manrope = Manrope({
  variable: "--font-manrope",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: { default: SITE_NAME, template: `%s \u00b7 ${SITE_NAME}` },
  description: SITE_DESCRIPTION,
  applicationName: SITE_NAME,
  openGraph: {
    ...siteOpenGraph(),
    title: SITE_NAME,
    description: SITE_DESCRIPTION,
    url: "/",
  },
  twitter: {
    ...siteTwitter(),
    title: SITE_NAME,
    description: SITE_DESCRIPTION,
  },
};

/** The browser chrome follows the reader's theme */
export const viewport: Viewport = {
  colorScheme: "light dark",
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f3f3f0" },
    { media: "(prefers-color-scheme: dark)", color: "#050505" },
  ],
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${bodoni.variable} ${manrope.variable}`}
      suppressHydrationWarning
    >
      <head>
        <script>{THEME_BOOTSTRAP_SCRIPT}</script>
      </head>
      <body>
        <ArtFilters />
        <Providers>
          <SiteHeader />
          <main>{children}</main>
          <SiteFooter />
        </Providers>
      </body>
    </html>
  );
}
