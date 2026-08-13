import { AccountForm } from "@/components/auth/AccountForm";
import { AuthPage } from "@/components/auth/AuthPage";

export default function SignInPage() {
  return (
    <AuthPage
      eyebrow="Return to LumiHub"
      title={
        <>
          Pick up where <em>your story left off.</em>
        </>
      }
      introduction="Sign in with the address and password you chose. Unverified accounts can still browse and correct a mistyped address."
    >
      <AccountForm mode="sign-in" />
    </AuthPage>
  );
}
