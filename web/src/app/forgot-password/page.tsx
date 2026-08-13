import { AuthPage } from "@/components/auth/AuthPage";
import { PasswordResetRequestPanel } from "@/components/auth/PasswordResetPanel";

export default function ForgotPasswordPage() {
  return (
    <AuthPage
      eyebrow="Account recovery"
      title={
        <>
          Find another way <em>back to your work.</em>
        </>
      }
      introduction="A verified address can open a recovery route for every account, including one first created through Discord."
    >
      <PasswordResetRequestPanel />
    </AuthPage>
  );
}
