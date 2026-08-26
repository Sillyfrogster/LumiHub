import { Suspense } from "react";
import { AuthPage } from "@/components/auth/AuthPage";
import { PasswordResetCompletionPanel } from "@/components/auth/PasswordResetPanel";

export const metadata = { title: "Choose a new password" };

export default function ResetPasswordPage() {
  return (
    <AuthPage
      title="Choose a new password"
      introduction="Set a new password for the verified email address on your Illarin account. The recovery link can only be used once."
    >
      <Suspense fallback={<p>Opening your password link…</p>}>
        <PasswordResetCompletionPanel />
      </Suspense>
    </AuthPage>
  );
}
