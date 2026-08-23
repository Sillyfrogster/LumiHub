import type { Metadata } from "next";
import Link from "next/link";
import { LegalPage, Unpublished } from "../LegalPage";

export const metadata: Metadata = { title: "Privacy Policy · Illarin" };

export default function Privacy() {
  return (
    <LegalPage
      href="/legal/privacy"
      title="Privacy Policy"
      lede={
        <>
          This Privacy Policy explains what data Illarin collects, how we use
          it, who we share it with, and the choices you have. It applies to your
          use of the Illarin website and any related services (the
          &ldquo;Service&rdquo;).
        </>
      }
    >
      <h2>1. Data we collect</h2>
      <h3>1.1 Account data</h3>
      <p>
        If you sign up with an email address, we store that address and a hash
        of your password. If you sign in through Discord, we receive your
        Discord user ID, username, avatar, and the email address on that Discord
        account when it is verified. We never receive your Discord password.
      </p>

      <h3>1.2 Content you upload</h3>
      <p>
        Characters, lorebooks, presets, themes, packs, and any other content you
        submit to the Service. This includes the source file you uploaded, the
        metadata you attach (names, tags, blurbs), and any text or images inside
        that content.
      </p>

      <h3>1.3 Usage and device data</h3>
      <p>
        When you access the Service we may collect: IP address, browser type and
        version, operating system, referrer URL, pages viewed, timestamps, and
        basic interaction events such as downloads. Some of this is collected
        automatically by our hosting and network providers for security and
        abuse-prevention purposes.
      </p>

      <h3>1.4 Cookies and local storage</h3>
      <p>
        We use cookies and browser local storage to keep you signed in, remember
        preferences such as appearance and adult-content visibility, and protect
        against abuse. We do not use advertising or cross-site tracking cookies.
      </p>

      <h3>1.5 Linked applications</h3>
      <p>
        When you link an application to your account, we store the identity that
        application reported, the permissions you granted it, and when it last
        used them, so that you can see and revoke the link.
      </p>

      <h2>2. How we use data</h2>
      <ul>
        <li>
          To operate the Service: authentication, displaying your work, search,
          downloads, and delivery to the applications you have linked.
        </li>
        <li>
          To moderate content and enforce our{" "}
          <Link href="/legal/acceptable-use">Acceptable Use Policy</Link>.
        </li>
        <li>To prevent abuse, fraud, and security incidents.</li>
        <li>
          To send you service messages such as email verification and password
          resets.
        </li>
        <li>To improve the Service based on aggregate usage patterns.</li>
      </ul>

      <h2>3. Legal bases (EEA/UK users)</h2>
      <p>
        If you are in the European Economic Area or UK, we process your personal
        data on the following bases: <strong>contract</strong> (to provide the
        Service you signed up for), <strong>legitimate interests</strong> (to
        keep the Service secure, prevent abuse, and moderate content),{" "}
        <strong>consent</strong> (where you have opted in, for example to view
        adult content), and <strong>legal obligations</strong> (for example,
        responding to lawful requests).
      </p>

      <h2>4. Sharing</h2>
      <p>We do not sell your personal data. We share data only with:</p>
      <ul>
        <li>
          <strong>Service providers</strong> we rely on to run Illarin, such as
          hosting, network, database, and email delivery. These providers
          process data on our behalf under appropriate confidentiality terms.
        </li>
        <li>
          <strong>Discord</strong>, if you choose to sign in or link with it, as
          described above.
        </li>
        <li>
          <strong>Other people and the public</strong> — content you choose to
          publish, and your profile, are visible to other people and may be
          visible to the public.
        </li>
        <li>
          <strong>Applications you link</strong>, which receive the work you ask
          them to fetch, within the permissions you granted.
        </li>
        <li>
          <strong>Authorities</strong> when required by law, court order, or to
          protect rights, safety, or the integrity of the Service.
        </li>
      </ul>

      <h2>5. Retention</h2>
      <p>
        We keep account data while your account is active. Content you publish
        is retained until you delete it or until it is removed for policy
        violations. Deleted work is held for a short window in which you can
        restore it, and is then destroyed. Moderation records may be kept longer
        to enforce repeat-offender rules and satisfy legal obligations. Server
        logs and security data are typically kept for a short rolling window.
      </p>

      <h2>6. Your rights</h2>
      <p>
        Depending on where you live, you may have rights to access, correct,
        delete, port, or restrict use of your personal data, and to object to
        processing or withdraw consent. You can exercise most of these rights
        through your account settings. For anything else, contact{" "}
        <Unpublished label="an address" />. If you are in California you have
        additional rights under the CCPA/CPRA; if you are in the EEA/UK you have
        additional rights under the GDPR/UK GDPR.
      </p>

      <h2>7. Children</h2>
      <p>
        Illarin is not directed to children under 13. We do not knowingly
        collect personal data from children under 13. If you believe a child has
        provided us personal data, contact us and we will delete it.
      </p>

      <h2>8. International transfers</h2>
      <p>
        The Service is hosted in the United States and other regions through our
        providers. Using the Service involves transferring your data to those
        regions. Where required, we rely on appropriate safeguards such as
        Standard Contractual Clauses.
      </p>

      <h2>9. Security</h2>
      <p>
        We take reasonable technical and organizational measures to protect your
        data, but no system is perfectly secure. Use a strong, unique password,
        keep your linked Discord account secure, and contact us if you suspect
        unauthorized access to your account.
      </p>

      <h2>10. Changes</h2>
      <p>
        We may update this Privacy Policy from time to time. We will update the
        effective date above and, for material changes, provide notice through
        the Service.
      </p>

      <h2>11. Contact</h2>
      <p>
        Privacy questions or requests: <Unpublished label="an address" />.
      </p>
    </LegalPage>
  );
}
