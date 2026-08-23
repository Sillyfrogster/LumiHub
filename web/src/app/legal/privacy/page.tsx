import type { Metadata } from "next";
import Link from "next/link";
import { LegalPage } from "../LegalPage";

export const metadata: Metadata = { title: "Privacy Policy · Illarin" };

export default function Privacy() {
  return (
    <LegalPage
      href="/legal/privacy"
      title="Privacy Policy"
      lede={
        <>
          This policy says what Illarin collects, why, who else sees it, and
          what you can do about it. Illarin is a personal project that makes no
          money, sells nothing, and has no reason to collect anything it does
          not need.
        </>
      }
    >
      <h2>1. What we collect</h2>

      <h3>1.1 Your account</h3>
      <p>
        If you sign up with an email address, we store the address and a hash of
        your password. We never store the password itself. If you sign in with
        Discord, we store your Discord user ID, and we receive the username and
        avatar Discord exposes. We also receive the email address on your
        Discord account when Discord says it is verified, and we use it to fill
        in your Illarin address only if you have none.
      </p>
      <p>
        Every account also has the handle you chose, and the date it was
        created.
      </p>

      <h3>1.2 What you upload</h3>
      <p>
        The file you uploaded, kept exactly as you uploaded it. The name, blurb,
        tags, and adult content answer you attached. The images you added.
        Anything written inside the work itself.
      </p>

      <h3>1.3 Your settings</h3>
      <p>
        Whether you want to see adult content, and whether your adult work
        appears on your public profile.
      </p>

      <h3>1.4 Applications you connect</h3>
      <p>
        For each application you link: the name and version it reported, the
        permissions you granted it, when you linked it, and when it last used
        its access. Its access tokens are stored only as hashes.
      </p>

      <h3>1.5 Server logs</h3>
      <p>
        Our web server records the usual line for each request: the IP address
        it came from, the time, the address requested, the response status, and
        the browser&rsquo;s user agent string. One-time link codes are stripped
        out of that line before it is written.
      </p>

      <h2>2. What we do not collect</h2>
      <ul>
        <li>
          <strong>We do not record who downloads what.</strong> Illarin counts
          downloads for a creator&rsquo;s benefit. The record holds the asset,
          the format, the time, and whether the download came from a signed-in
          reader, the creator, a linked application, or nobody signed in at all.
          It holds no account, no IP address, and nothing else that could point
          back to a person.
        </li>
        <li>
          <strong>There is no analytics product on the site</strong>, no
          advertising, no advertising or cross-site tracking cookies, and no
          third-party script watching you read.
        </li>
        <li>
          <strong>
            Nothing you upload is scanned by a machine learning model.
          </strong>{" "}
          Illarin runs no content classifiers, and it does not use your work as
          training data.
        </li>
      </ul>

      <h2>3. Cookies and what your browser stores</h2>
      <p>
        Illarin sets a cookie holding your session when you sign in, and two
        short-lived cookies during a Discord sign-in so that it can bring you
        back to the page you started from. All three are strictly necessary to
        sign you in.
      </p>
      <p>
        Your browser also keeps two things locally, which never reach us: your
        light or dark appearance choice, and, for the length of the tab, whether
        you asked to see adult content.
      </p>

      <h2>4. Why we use it</h2>
      <ul>
        <li>
          To run Illarin: signing you in, showing your work, search, downloads,
          and exports.
        </li>
        <li>
          To hand your library to the applications you have linked, within what
          you granted.
        </li>
        <li>
          To send you the emails the account needs, which are address
          verification and password resets.
        </li>
        <li>
          To act on reports and enforce the{" "}
          <Link href="/legal/acceptable-use">Acceptable Use Policy</Link>.
        </li>
        <li>To keep Illarin working and to see what broke when it does not.</li>
      </ul>

      <h2>5. Legal bases, if you are in the EEA or UK</h2>
      <p>
        We rely on <strong>contract</strong> to give you the service you signed
        up for, <strong>legitimate interests</strong> to keep Illarin secure and
        to act on reports, <strong>consent</strong> where you have opted in,
        which is how adult content works, and <strong>legal obligation</strong>{" "}
        where the law requires us to respond.
      </p>

      <h2>6. Who else sees it</h2>
      <p>
        We do not sell your personal data, and we have no interest in doing so.
        It reaches:
      </p>
      <ul>
        <li>
          <strong>Our hosting provider</strong>, whose servers hold the
          database, the uploaded files, and the logs.
        </li>
        <li>
          <strong>Our email provider</strong>, which receives your address and
          the message when Illarin sends a verification or password-reset email.
        </li>
        <li>
          <strong>Our monitoring provider</strong>, which receives the server
          logs described above so that we can see errors and outages.
        </li>
        <li>
          <strong>Discord</strong>, if you choose to sign in or link with it.
        </li>
        <li>
          <strong>Applications you link</strong>, which receive the work they
          are allowed to fetch.
        </li>
        <li>
          <strong>Anyone</strong>, for work you publish and for your profile.
          That is the point of publishing.
        </li>
        <li>
          <strong>Authorities</strong>, where the law, a court order, or a
          genuine safety risk requires it.
        </li>
      </ul>

      <h2>7. How long we keep it</h2>
      <p>
        Account data lasts as long as your account. Work you publish lasts until
        you delete it or it is removed. Deleted work sits in a 30 day recovery
        window while you can still restore it, and is destroyed after that.
        Sign-in sessions, email verification links, password reset links, and
        application linking codes all expire on their own. Server logs are kept
        for a short rolling window.
      </p>

      <h2>8. Your rights</h2>
      <p>
        You can see and change most of your data in your account settings,
        including your email address, your password, your handle, your linked
        applications, and your content preferences. Deleting an asset or your
        account is a normal control, not a request you have to file.
      </p>
      <p>
        For anything else — a copy of your data, a correction you cannot make
        yourself, an objection, or withdrawing consent — write to{" "}
        <a href="mailto:team@illarin.xyz">team@illarin.xyz</a>. If you are in
        California you have further rights under the CCPA and CPRA. If you are
        in the EEA or UK you have further rights under the GDPR and UK GDPR,
        including the right to complain to your data protection authority.
      </p>

      <h2>9. Children</h2>
      <p>
        Illarin is not for children under 13, and we do not knowingly collect
        anything from them. If you believe a child has given us personal data,
        write to <a href="mailto:team@illarin.xyz">team@illarin.xyz</a> and we
        will delete it.
      </p>

      <h2>10. Where your data goes</h2>
      <p>
        Illarin&rsquo;s servers, and the email, monitoring, and sign-in services
        it depends on, operate across several countries including the United
        States. Using Illarin means your data travels to those places. Where the
        law requires safeguards for that transfer, we rely on the ones our
        providers put in place, such as Standard Contractual Clauses.
      </p>

      <h2>11. Security</h2>
      <p>
        Passwords are hashed, never stored in readable form. Session tokens,
        password reset links, email verification links, and application tokens
        are stored as hashes too, so a copy of the database does not hand
        someone your account. Traffic to Illarin is encrypted in transit.
      </p>
      <p>
        None of that makes a system perfectly secure. Use a password you use
        nowhere else, keep your Discord account secure, and write to{" "}
        <a href="mailto:team@illarin.xyz">team@illarin.xyz</a> if you think
        someone has got into your account.
      </p>

      <h2>12. Changes</h2>
      <p>
        We may update this policy. The effective date at the top says when the
        current wording took effect, and for a change that matters we will say
        something through Illarin itself.
      </p>

      <h2>13. Contact</h2>
      <p>
        Privacy questions and requests go to{" "}
        <a href="mailto:team@illarin.xyz">team@illarin.xyz</a>.
      </p>
    </LegalPage>
  );
}
