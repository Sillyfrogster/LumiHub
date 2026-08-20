import type { Metadata } from "next";
import { Inter, Playfair_Display } from "next/font/google";
import { ArtFilters } from "@/components/art/ArtFilters";
import { PageMaterial } from "@/components/layout/PageMaterial";
import { SiteFooter } from "@/components/layout/SiteFooter";
import { SiteHeader } from "@/components/layout/SiteHeader";
import { THEME_BOOTSTRAP_SCRIPT } from "@/lib/theme";
import { Providers } from "./providers";
import "./globals.css";

const playfair = Playfair_Display({
  variable: "--font-playfair",
  subsets: ["latin"],
  style: ["normal", "italic"],
});

const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
});

/** The public origin, so link previews carry absolute URLs. */
const siteUrl = process.env.SITE_URL ?? "http://localhost:8000";

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: "LumiHub",
  description:
    "Discover characters, lorebooks, presets, and themes while keeping every creator's source file intact.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${playfair.variable} ${inter.variable}`}
      suppressHydrationWarning
    >
      <head>
        <meta name="color-scheme" content="light dark" />
        <script>{THEME_BOOTSTRAP_SCRIPT}</script>
      </head>
      <body>
        <ArtFilters />
        <PageMaterial />
        <Providers>
          <SiteHeader />
          <main>{children}</main>
          <SiteFooter />
        </Providers>
      </body>
    </html>
  );
}
