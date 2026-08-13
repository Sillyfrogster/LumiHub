import { AccountForm } from "@/components/auth/AccountForm";
import { AuthPage } from "@/components/auth/AuthPage";

export default function SignUpPage() {
  return (
    <AuthPage
      eyebrow="A place to publish"
      title={
        <>
          Sign your work. <em>Keep your way back.</em>
        </>
      }
      introduction="Create a LumiHub account without Discord. We will ask you to verify your address before anything can be published under your name."
    >
      <AccountForm mode="sign-up" />
    </AuthPage>
  );
}
