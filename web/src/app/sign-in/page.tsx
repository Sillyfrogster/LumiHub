import { AccountForm } from "@/components/auth/AccountForm";
import { AuthPage } from "@/components/auth/AuthPage";
import { safeInternalReturnPath } from "@/lib/internal-return";

const DISCORD_ERRORS: Record<string, string> = {
  cancelled: "Discord sign-in was cancelled. Nothing changed.",
  "email-conflict":
    "That verified address already belongs to another Illarin account. No accounts were merged.",
  failed: "Discord could not sign you in. Please try again.",
};

export default async function SignInPage({
  searchParams,
}: {
  searchParams: Promise<{
    discord?: string | string[];
    returnTo?: string | string[];
  }>;
}) {
  const query = await searchParams;
  const discord = Array.isArray(query.discord)
    ? query.discord[0]
    : query.discord;
  const returnTo = safeInternalReturnPath(query.returnTo);

  return (
    <AuthPage
      title="Sign in to Illarin"
      introduction="Use Discord or the email address and password on your account. Unverified accounts can still browse and correct a mistyped address."
    >
      <AccountForm
        mode="sign-in"
        discordError={discord ? DISCORD_ERRORS[discord] : undefined}
        returnTo={returnTo}
      />
    </AuthPage>
  );
}
