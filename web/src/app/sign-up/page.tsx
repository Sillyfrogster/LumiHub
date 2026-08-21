import { AccountForm } from "@/components/auth/AccountForm";
import { AuthPage } from "@/components/auth/AuthPage";
import { safeInternalReturnPath } from "@/lib/internal-return";

export default async function SignUpPage({
  searchParams,
}: {
  searchParams: Promise<{ returnTo?: string | string[] }>;
}) {
  const query = await searchParams;
  const returnTo = safeInternalReturnPath(query.returnTo);

  return (
    <AuthPage
      title="Create an Illarin account"
      introduction="Choose a handle and sign in with email. Verify the address before publishing under that handle."
    >
      <AccountForm mode="sign-up" returnTo={returnTo} />
    </AuthPage>
  );
}
