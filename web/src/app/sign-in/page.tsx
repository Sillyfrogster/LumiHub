import { AccountForm } from "@/components/auth/AccountForm";
import { AuthPage } from "@/components/auth/AuthPage";

const DISCORD_ERRORS: Record<string, string> = {
  cancelled: "Discord sign-in was cancelled. Nothing changed.",
  "email-conflict":
    "That verified address already belongs to another LumiHub account. No accounts were merged.",
  failed: "Discord could not sign you in. Please try again.",
};

export default async function SignInPage({
  searchParams,
}: {
  searchParams: Promise<{ discord?: string | string[] }>;
}) {
  const query = await searchParams;
  const discord = Array.isArray(query.discord)
    ? query.discord[0]
    : query.discord;

  return (
    <AuthPage
      eyebrow="Return to LumiHub"
      title={
        <>
          Pick up where <em>your story left off.</em>
        </>
      }
      introduction="Return with Discord or with the address and password you chose. Unverified accounts can still browse and correct a mistyped address."
    >
      <AccountForm
        mode="sign-in"
        discordError={discord ? DISCORD_ERRORS[discord] : undefined}
      />
    </AuthPage>
  );
}
