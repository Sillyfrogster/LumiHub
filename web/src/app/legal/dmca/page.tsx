import type { Metadata } from "next";
import { LegalPage, Unpublished } from "../LegalPage";

export const metadata: Metadata = { title: "DMCA / Copyright · Illarin" };

export default function Dmca() {
  return (
    <LegalPage
      href="/legal/dmca"
      title="DMCA / Copyright Policy"
      lede={
        <>
          Illarin respects the intellectual property rights of others and
          expects the people who use it to do the same. We respond to clear
          notices of alleged copyright infringement under the U.S. Digital
          Millennium Copyright Act (&ldquo;DMCA&rdquo;) and similar laws.
        </>
      }
    >
      <h2>1. How to submit a takedown notice</h2>
      <p>
        To report content you believe infringes your copyright, send a written
        notice to our designated agent containing all of the following:
      </p>
      <ol>
        <li>
          A physical or electronic signature of the copyright owner or someone
          authorized to act on their behalf.
        </li>
        <li>
          Identification of the copyrighted work claimed to be infringed, or,
          for multiple works at a single site, a representative list.
        </li>
        <li>
          Identification of the material on Illarin that is claimed to be
          infringing, with information reasonably sufficient to let us locate
          it, such as URLs.
        </li>
        <li>
          Your contact information: name, address, telephone number, and email
          address.
        </li>
        <li>
          A statement that you have a good-faith belief that the use of the
          material in the manner complained of is not authorized by the
          copyright owner, its agent, or the law.
        </li>
        <li>
          A statement, made under penalty of perjury, that the information in
          the notice is accurate and that you are the copyright owner or
          authorized to act on the owner&rsquo;s behalf.
        </li>
      </ol>

      <h2>2. Designated agent</h2>
      <p>Send notices to our DMCA-designated agent:</p>
      <ul>
        <li>
          <strong>Agent:</strong> <Unpublished label="a name" />
        </li>
        <li>
          <strong>Email:</strong> <Unpublished label="an address" />
        </li>
        <li>
          <strong>Mailing address:</strong> <Unpublished label="an address" />
        </li>
      </ul>
      <p>
        Email is the fastest channel. Notices that don&rsquo;t include the
        elements above may be delayed or rejected.
      </p>

      <h2>3. What happens after a valid notice</h2>
      <ul>
        <li>
          We will remove or disable access to the material identified in the
          notice within a reasonable time.
        </li>
        <li>
          We will make a reasonable attempt to notify the creator who uploaded
          the material and forward a copy of the notice.
        </li>
        <li>
          Repeat infringers&rsquo; accounts will be terminated in appropriate
          circumstances.
        </li>
      </ul>

      <h2>4. Counter-notice</h2>
      <p>
        If your content was removed and you believe the removal was a mistake or
        misidentification, you may send a counter-notice containing:
      </p>
      <ol>
        <li>Your physical or electronic signature.</li>
        <li>
          Identification of the material that was removed and the address where
          it appeared before removal.
        </li>
        <li>
          A statement under penalty of perjury that you have a good-faith belief
          the material was removed as a result of mistake or misidentification.
        </li>
        <li>
          Your name, address, and telephone number; a statement consenting to
          the jurisdiction of the federal district court for the judicial
          district where your address is located, or, if you are outside the
          U.S., the U.S. federal district court for{" "}
          <Unpublished label="a district" />; and a statement that you will
          accept service of process from the person who submitted the original
          notice.
        </li>
      </ol>
      <p>
        If we receive a valid counter-notice, we may forward it to the original
        complainant and, unless they file a court action seeking to restrain the
        activity within the window required by law, restore the removed
        material.
      </p>

      <h2>5. Misuse</h2>
      <p>
        Knowingly submitting false notices or counter-notices may result in
        liability under 17 U.S.C. § 512(f), including damages and
        attorneys&rsquo; fees. Don&rsquo;t file frivolous claims.
      </p>

      <h2>6. Other rights</h2>
      <p>
        For non-copyright IP issues, such as trademark or right of publicity,
        contact our DMCA agent above. We handle those case by case.
      </p>
    </LegalPage>
  );
}
