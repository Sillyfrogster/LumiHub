import type { Metadata } from "next";
import Link from "next/link";
import { LegalPage } from "../LegalPage";

export const metadata: Metadata = { title: "Acceptable Use · Illarin" };

export default function AcceptableUse() {
  return (
    <LegalPage
      href="/legal/acceptable-use"
      title="Acceptable Use Policy"
      lede={
        <>
          Illarin holds characters, lorebooks, presets, themes, and packs made
          by the people who use it. These rules apply to all of it, and to how
          you behave here. Breaking them can mean your work is withheld or
          removed, or your account is suspended or closed.
        </>
      }
    >
      <h2>1. Never allowed</h2>
      <p>
        None of the following belongs on Illarin, in any work, in any image, in
        a profile, or anywhere else:
      </p>
      <ul>
        <li>
          <strong>Sexual content involving minors</strong>, real or fictional.
          Any sexual or sexualised depiction of someone presented or implied to
          be under 18 is banned, however the character is framed. Calling a
          character ageless, ancient, or an adult-coded archetype does not get
          around this rule.
        </li>
        <li>
          <strong>
            Sexual content about real, identifiable people who have not
            consented
          </strong>
          , including deepfake-style material.
        </li>
        <li>
          <strong>
            Real-world violence aimed at identifiable people or groups
          </strong>
          : incitement, glorification, plans for an attack, or anything
          encouraging self-harm or suicide.
        </li>
        <li>
          <strong>Private personal information</strong> published without the
          person&rsquo;s consent, such as real names, home or work addresses,
          phone numbers, government identifiers, or financial details.
        </li>
        <li>
          <strong>Anything illegal</strong> where Illarin is hosted or where you
          are, including child sexual abuse material, terrorist material, and
          extreme gore.
        </li>
        <li>
          <strong>Instructions for serious real-world harm</strong>, including
          workable instructions for weapons capable of mass casualties, malware
          aimed at real systems, or attacks on infrastructure.
        </li>
      </ul>

      <h2>2. Adult content</h2>
      <p>
        Adult work is welcome on Illarin as long as it stays inside the rules
        above and is marked correctly.
      </p>
      <ul>
        <li>
          Adult content is <strong>hidden by default</strong>. A reader has to
          choose to see it, and confirm they are 18 or older.
        </li>
        <li>
          Every asset has to answer the adult content question before it can be
          published, and answering it wrongly is a violation on its own. If you
          are unsure, mark it as adult.
        </li>
        <li>
          Every sexual character has to be unmistakably 18 or older, in the
          writing and in any image.
        </li>
        <li>
          You can keep your adult work off your public profile with a setting,
          and that setting does not change any of the rules above.
        </li>
      </ul>

      <h2>3. Harassment and hate</h2>
      <ul>
        <li>
          No targeted harassment, threats, or stalking, of people here or
          anywhere else.
        </li>
        <li>
          Nothing that dehumanises people or incites hatred against them based
          on race, ethnicity, national origin, religion, disability, gender,
          gender identity, sexual orientation, or anything else of that kind.
        </li>
        <li>
          Hard criticism, satire, and plain disagreement are fine. Personal
          attacks and slurs used as insults are not.
        </li>
      </ul>

      <h2>4. Other people&rsquo;s work</h2>
      <ul>
        <li>
          Don&rsquo;t upload work that infringes someone&rsquo;s copyright,
          trademark, or other rights.
        </li>
        <li>
          Fanwork is welcome when it follows the rest of this policy.
          Don&rsquo;t claim work that isn&rsquo;t yours as your own.
        </li>
        <li>
          If you are the rights holder, the{" "}
          <Link href="/legal/dmca">DMCA and copyright policy</Link> explains how
          to get something taken down.
        </li>
      </ul>

      <h2>5. Spam and abusing the platform</h2>
      <ul>
        <li>
          No spam, bulk uploads of the same thing, link farming, or affiliate
          bait.
        </li>
        <li>
          No inflating your own download counts or manipulating search results.
        </li>
        <li>
          No scraping or crawling beyond what Illarin&rsquo;s rate limits allow.
        </li>
        <li>
          No probing, scanning, or attacking Illarin, and no trying to get into
          anyone else&rsquo;s account.
        </li>
        <li>
          No malware, phishing, or misleading links in uploads or profiles.
        </li>
        <li>
          Don&rsquo;t use a linked application, or a token from one, to reach
          work you would not be allowed to reach in a browser.
        </li>
      </ul>

      <h2>6. Being straight about who you are</h2>
      <ul>
        <li>
          Don&rsquo;t pretend to be someone else, including other creators here,
          public figures, or us.
        </li>
        <li>Don&rsquo;t make a new account to get around a suspension.</li>
      </ul>

      <h2>7. How this is enforced</h2>
      <p>
        Moderation on Illarin is a person reading a report. There are no
        automated classifiers, and nothing you upload is scored by a machine.
      </p>
      <p>
        We may withhold an asset, which takes it out of the catalog and makes it
        answer as missing to everyone but its creator, who can still read it,
        download it, and see why. We may also remove work outright, suspend an
        account, or close one. What happens depends on how serious it is and
        what came before. Anything in section 1 usually means immediate removal
        and can mean an immediate permanent ban.
      </p>

      <h2>8. Reporting something</h2>
      <p>
        Write to <a href="mailto:team@illarin.xyz">team@illarin.xyz</a>. Include
        the address of the work and what is wrong with it. For anything urgent —
        a credible threat, private information published about someone, or
        content covered by section 1 — say so in the subject line.
      </p>

      <h2>9. Changes</h2>
      <p>
        This policy changes as Illarin does. The effective date at the top says
        when the current wording took effect.
      </p>
    </LegalPage>
  );
}
