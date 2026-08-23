import type { Metadata } from "next";
import Link from "next/link";
import { LegalPage, Unpublished } from "../LegalPage";

export const metadata: Metadata = { title: "Acceptable Use · Illarin" };

export default function AcceptableUse() {
  return (
    <LegalPage
      href="/legal/acceptable-use"
      title="Acceptable Use Policy"
      lede={
        <>
          Illarin hosts characters, lorebooks, presets, themes, and packs made
          by the people who use it. To keep the Service usable and lawful for
          everyone, the following rules apply to all content and behaviour on
          it. Violations may result in content removal, restricted visibility,
          account suspension, or account termination at our discretion.
        </>
      }
    >
      <h2>1. Hard limits — never allowed</h2>
      <p>
        The following are <strong>never</strong> permitted on Illarin, in any
        work, profile, or other content:
      </p>
      <ul>
        <li>
          <strong>Sexual content involving minors</strong>, real or fictional.
          Any sexual or sexualized depiction of a person presented or implied to
          be under 18 is prohibited, regardless of whether the character is
          described as an adult-coded archetype, an &ldquo;ageless&rdquo;
          entity, or otherwise framed to evade this rule.
        </li>
        <li>
          <strong>
            Non-consensual sexual content involving real, identifiable people
          </strong>
          . This includes deepfake-style sexual content and similar depictions.
        </li>
        <li>
          <strong>
            Real-world violence targeting identifiable people or groups
          </strong>
          : incitement, glorification, instructions for attacks, or content that
          encourages self-harm or suicide.
        </li>
        <li>
          <strong>Doxxing</strong> — posting private personal information (real
          names, home or work addresses, phone numbers, government IDs,
          financial details) without the subject&rsquo;s consent.
        </li>
        <li>
          <strong>Content that is illegal</strong> where the Service is hosted
          or where you are located, including but not limited to child sexual
          abuse material, terrorism-related material, and certain categories of
          extreme gore.
        </li>
        <li>
          <strong>Instructions for serious real-world harm</strong>, including
          credible instructions for weapons capable of mass casualties, malware
          targeting real systems, or attacks on critical infrastructure.
        </li>
      </ul>

      <h2>2. Adult content</h2>
      <p>
        Adult content is permitted on Illarin when it complies with the hard
        limits above and is correctly marked.
      </p>
      <ul>
        <li>
          Adult content is <strong>hidden by default</strong>. A reader must opt
          in, and confirm they are 18 or older, to see it.
        </li>
        <li>
          Creators are responsible for answering the adult-content question
          accurately. Publishing unmarked adult content is a violation.
        </li>
        <li>
          All sexual characters and personas must be unambiguously 18 or older,
          both in description and in any image.
        </li>
      </ul>

      <h2>3. Harassment and hate</h2>
      <ul>
        <li>
          No targeted harassment, threats, or stalking of other people or third
          parties.
        </li>
        <li>
          No content that dehumanizes or incites hatred against people based on
          race, ethnicity, national origin, religion, disability, gender, gender
          identity, sexual orientation, or similar protected categories.
        </li>
        <li>
          Heated criticism, satire, and disagreement are fine. Personal attacks,
          slurs used as insults, and pile-ons are not.
        </li>
      </ul>

      <h2>4. Intellectual property</h2>
      <ul>
        <li>
          Don&rsquo;t upload content that infringes someone else&rsquo;s
          copyright, trademark, or other IP rights.
        </li>
        <li>
          Fanwork is welcome when it complies with the rest of this policy.
          Don&rsquo;t claim ownership of work that isn&rsquo;t yours.
        </li>
        <li>
          See the <Link href="/legal/dmca">DMCA / Copyright Policy</Link> for
          takedown procedures.
        </li>
      </ul>

      <h2>5. Spam and platform abuse</h2>
      <ul>
        <li>
          No spam, bulk uploads, link-farming, SEO spam, or affiliate-link bait.
        </li>
        <li>
          No content designed to manipulate moderation, search results, or
          download counts.
        </li>
        <li>
          No automated scraping or crawling beyond what our rate limits and
          robots policy permit.
        </li>
        <li>
          No attempts to probe, scan, or compromise the Service or other
          people&rsquo;s accounts.
        </li>
        <li>
          No malware, phishing, or deceptive redirects in uploads, links, or
          profiles.
        </li>
      </ul>

      <h2>6. Honesty about identity</h2>
      <ul>
        <li>
          Don&rsquo;t impersonate other people, including Illarin staff, public
          figures, or other creators.
        </li>
        <li>
          Ban evasion — creating new accounts to get around a suspension — is a
          violation.
        </li>
      </ul>

      <h2>7. Moderation</h2>
      <p>
        Content may be hidden, restricted, or removed at our discretion. We may
        tell an affected creator why their work was removed, but we are not
        obliged to provide a formal appeals process for every decision.
      </p>

      <h2>8. Reporting</h2>
      <p>
        Contact <Unpublished label="an address" /> for anything that needs
        direct attention, such as doxxing, credible threats, or off-platform
        abuse coordinated through Illarin.
      </p>

      <h2>9. Enforcement</h2>
      <p>
        Consequences scale with severity and history, and may include a warning,
        a required edit, content removal, reduced visibility, temporary
        suspension, or a permanent ban. Hard-limit violations typically result
        in immediate removal and may result in an immediate permanent ban.
      </p>

      <h2>10. Changes</h2>
      <p>
        We update this policy as the Service evolves. The effective date above
        reflects the current version.
      </p>
    </LegalPage>
  );
}
