import type { Metadata } from "next";
import Link from "next/link";
import { LegalPage, Unpublished } from "../LegalPage";

export const metadata: Metadata = { title: "Terms of Service · Illarin" };

export default function Terms() {
  return (
    <LegalPage
      href="/legal/terms"
      title="Terms of Service"
      lede={
        <>
          These Terms govern your access to and use of Illarin (the
          &ldquo;Service&rdquo;). By creating an account, uploading content, or
          otherwise using the Service, you agree to these Terms. If you
          don&rsquo;t agree, don&rsquo;t use the Service.
        </>
      }
    >
      <h2>1. Who we are</h2>
      <p>
        Illarin is operated as a personal project. The operator can be contacted
        at <Unpublished label="an address" />. References to &ldquo;we,&rdquo;
        &ldquo;us,&rdquo; and &ldquo;Illarin&rdquo; mean the operator.
      </p>

      <h2>2. Eligibility</h2>
      <p>
        You must be at least 13 years old to use the Service. You must be 18 or
        older to view, upload, or interact with content marked as adult content.
        By using the Service you represent that you meet these requirements and
        that you are not barred from using the Service under the laws of your
        jurisdiction.
      </p>

      <h2>3. Accounts</h2>
      <p>
        An account is reached by an email address and password, by a linked
        Discord identity, or by both. You are responsible for any activity that
        occurs under your account, including content you upload. Keep your
        password and your linked accounts secure. We may suspend or terminate
        accounts that violate these Terms or the{" "}
        <Link href="/legal/acceptable-use">Acceptable Use Policy</Link>.
      </p>

      <h2>4. Your content</h2>
      <h3>4.1 Your content stays yours</h3>
      <p>
        You retain ownership of the characters, lorebooks, presets, themes,
        packs, and other content you upload (&ldquo;Your Content&rdquo;). You
        are solely responsible for it.
      </p>

      <h3>4.2 License you grant us</h3>
      <p>
        To operate the Service, you grant Illarin a worldwide, non-exclusive,
        royalty-free license to host, store, reproduce, display, distribute,
        transmit, transform (for example, generate thumbnails or convert a
        source file into an export for another application), and make derivative
        copies of Your Content solely as needed to run, promote, and improve the
        Service. This license ends when you delete Your Content, except for
        copies retained in backups, caches, or as required by law.
      </p>

      <h3>4.3 Your representations</h3>
      <p>By uploading Your Content you represent that:</p>
      <ul>
        <li>
          You own it or have the rights to upload and license it as described
          above.
        </li>
        <li>
          It does not infringe anyone else&rsquo;s intellectual property or
          other rights.
        </li>
        <li>
          It complies with the{" "}
          <Link href="/legal/acceptable-use">Acceptable Use Policy</Link>.
        </li>
      </ul>

      <h2>5. Moderation</h2>
      <p>
        We may remove, hide, restrict, or refuse content at our sole discretion,
        including content that violates our policies or that we judge harmful,
        illegal, or off-brand. We may also act on reports from other users.
        Moderation decisions are not guaranteed to be perfect, fast, or
        appealable.
      </p>

      <h2>6. Acceptable use</h2>
      <p>
        Your use of the Service is also governed by our{" "}
        <Link href="/legal/acceptable-use">Acceptable Use Policy</Link>, which
        is incorporated into these Terms by reference. Violations may result in
        content removal, account suspension, or termination.
      </p>

      <h2>7. Third-party services</h2>
      <p>
        The Service may integrate with third-party services, including Discord
        for sign-in, hosting and network providers, and the applications you
        link to your account. Your use of those services is also governed by
        their terms. We are not responsible for the availability, content, or
        practices of third-party services.
      </p>

      <h2>8. Donations</h2>
      <p>
        Donations made through third-party platforms are voluntary,
        non-refundable unless required by law, and do not entitle you to any
        product, feature, or service beyond what is generally available.
        Donations do not exempt you from these Terms.
      </p>

      <h2>9. Intellectual property in the Service</h2>
      <p>
        The Service itself — including the Illarin name, design, code, and
        non-user content — is owned by us or our licensors and is protected by
        intellectual property laws. You may not copy, modify, or create
        derivative works of the Service except as permitted by law or by us in
        writing.
      </p>

      <h2>10. Copyright (DMCA)</h2>
      <p>
        If you believe content on the Service infringes your copyright, see our{" "}
        <Link href="/legal/dmca">DMCA / Copyright Policy</Link> for the takedown
        procedure and designated agent contact.
      </p>

      <h2>11. Termination</h2>
      <p>
        You may stop using the Service and delete your account at any time. We
        may suspend or terminate your access at any time, with or without
        notice, if we believe you have violated these Terms or if we discontinue
        the Service. Sections that by their nature should survive termination
        (ownership, licenses you have granted us for existing copies,
        disclaimers, liability limits, and dispute provisions) will survive.
      </p>

      <h2>12. Disclaimers</h2>
      <p>
        The Service is provided{" "}
        <strong>&ldquo;as is&rdquo; and &ldquo;as available&rdquo;</strong>{" "}
        without warranties of any kind, express or implied, including
        merchantability, fitness for a particular purpose, non-infringement, and
        any warranty arising out of course of dealing or usage of trade. We do
        not warrant that the Service will be uninterrupted, error-free, secure,
        or that content on it is accurate, safe, or appropriate.
      </p>

      <h2>13. Limitation of liability</h2>
      <p>
        To the maximum extent permitted by law, in no event will Illarin, its
        operator, or its contributors be liable for any indirect, incidental,
        special, consequential, or punitive damages, or any loss of profits,
        revenue, data, or goodwill, arising out of or related to your use of the
        Service. Our total liability for any claim arising out of or related to
        the Service will not exceed <strong>USD $100</strong> or the amount you
        paid us in the twelve months before the claim, whichever is greater.
      </p>

      <h2>14. Indemnification</h2>
      <p>
        You agree to indemnify and hold harmless Illarin and its operator from
        any claim, damages, liability, or expense (including reasonable
        attorneys&rsquo; fees) arising out of Your Content, your use of the
        Service, or your violation of these Terms.
      </p>

      <h2>15. Governing law and disputes</h2>
      <p>
        These Terms are governed by the laws of{" "}
        <Unpublished label="a jurisdiction" />, without regard to its
        conflict-of-laws rules. You and we agree to bring any dispute
        exclusively in the courts of that jurisdiction, and you consent to
        personal jurisdiction there. Nothing in this section limits any
        non-waivable rights you may have under the law of your country of
        residence.
      </p>

      <h2>16. Changes to these Terms</h2>
      <p>
        We may update these Terms from time to time. If we make material
        changes, we will update the effective date above and, where reasonable,
        provide notice through the Service. Continuing to use the Service after
        changes take effect means you accept the updated Terms.
      </p>

      <h2>17. Contact</h2>
      <p>
        Questions about these Terms? Contact <Unpublished label="an address" />.
      </p>
    </LegalPage>
  );
}
