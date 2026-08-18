import type { Metadata } from "next";
import { Inter, Lato, Playfair_Display } from "next/font/google";
import { PaperGrain } from "@/components/layout/PaperGrain";
import { SiteFooter } from "@/components/layout/SiteFooter";
import { SiteHeader } from "@/components/layout/SiteHeader";
import { Providers } from "./providers";
import "./globals.css";

const playfair = Playfair_Display({
  variable: "--font-playfair",
  subsets: ["latin"],
  style: ["normal", "italic"],
});

const lato = Lato({
  variable: "--font-lato",
  subsets: ["latin"],
  weight: ["300", "400", "700", "900"],
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
      className={`${playfair.variable} ${lato.variable} ${inter.variable}`}
    >
      <body>
        <PaperGrain />
        <Providers>
          <SiteHeader />
          <main>{children}</main>
          <SiteFooter />
        </Providers>
      </body>
    </html>
  );
}
