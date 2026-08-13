import { Suspense } from "react";
import { AuthPage } from "@/components/auth/AuthPage";
import { PasswordResetCompletionPanel } from "@/components/auth/PasswordResetPanel";

export default function ResetPasswordPage() {
  return (
    <AuthPage
      eyebrow="Account recovery"
      title={
        <>
          Write yourself a <em>new way in.</em>
        </>
      }
      introduction="Set a private password for the verified address on your LumiHub account. The recovery link closes behind you."
    >
      <Suspense fallback={<p>Opening your password link…</p>}>
        <PasswordResetCompletionPanel />
      </Suspense>
    </AuthPage>
  );
}
