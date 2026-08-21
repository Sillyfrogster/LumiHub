import { Suspense } from "react";
import { AuthPage } from "@/components/auth/AuthPage";
import { VerificationPanel } from "@/components/auth/VerificationPanel";

export default function VerifyEmailPage() {
  return (
    <AuthPage
      title="Verify your email"
      introduction="Verification is required before publishing or linking an application. You can keep browsing while the account is unverified."
    >
      <Suspense fallback={<p>Opening your verification link…</p>}>
        <VerificationPanel />
      </Suspense>
    </AuthPage>
  );
}
