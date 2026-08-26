import { AuthPage } from "@/components/auth/AuthPage";
import { PasswordResetRequestPanel } from "@/components/auth/PasswordResetPanel";

export const metadata = { title: "Reset your password" };

export default function ForgotPasswordPage() {
  return (
    <AuthPage
      title="Reset your password"
      introduction="Send a recovery link to the verified email address on the account, including accounts first created through Discord."
    >
      <PasswordResetRequestPanel />
    </AuthPage>
  );
}
