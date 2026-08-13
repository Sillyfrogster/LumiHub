import { Suspense } from "react";
import { AuthPage } from "@/components/auth/AuthPage";
import { VerificationPanel } from "@/components/auth/VerificationPanel";

export default function VerifyEmailPage() {
  return (
    <AuthPage
      eyebrow="One last step"
      title={
        <>
          Make the address <em>truly yours.</em>
        </>
      }
      introduction="Verification protects every handle and every creation behind it. Until then, the whole catalog remains open to you."
    >
      <Suspense fallback={<p>Opening your verification link…</p>}>
        <VerificationPanel />
      </Suspense>
    </AuthPage>
  );
}
