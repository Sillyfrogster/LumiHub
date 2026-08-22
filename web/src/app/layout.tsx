import type { Metadata } from "next";
import { Bodoni_Moda, Manrope } from "next/font/google";
import { ArtFilters } from "@/components/art/ArtFilters";
import { SiteFooter } from "@/components/layout/SiteFooter";
import { SiteHeader } from "@/components/layout/SiteHeader";
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

/** The public origin, so link previews carry absolute URLs. */
const siteUrl = process.env.SITE_URL ?? "http://localhost:8000";

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: "Illarin",
  description:
    "Discover characters, lorebooks, presets, themes, and packs while keeping every creator's source file intact.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${bodoni.variable} ${manrope.variable}`}
      suppressHydrationWarning
    >
      <head>
        <meta name="color-scheme" content="light dark" />
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
