import { AccountSettings } from "@/components/auth/AccountSettings";
import { Shell } from "@/components/layout/Shell";
import { LinkedInstances } from "@/components/linking/LinkedInstances";
import styles from "./SettingsPage.module.css";

const DISCORD_NOTICES: Record<string, string> = {
  attached: "Discord is now another way into this account.",
  claimed:
    "That Discord identity cannot be attached here. No account details were revealed and nothing was merged.",
  "email-conflict":
    "Discord reported an address already verified on another account. Nothing was changed.",
  failed: "Discord could not be attached. Please try again.",
};

export const metadata = { title: "Account settings" };

export default async function SettingsPage({
  searchParams,
}: {
  searchParams: Promise<{ discord?: string | string[] }>;
}) {
  const query = await searchParams;
  const discord = Array.isArray(query.discord)
    ? query.discord[0]
    : query.discord;

  return (
    <section className={styles.page}>
      <Shell className={styles.layout}>
        <header className={styles.heading}>
          <h1>Account settings</h1>
          <p>
            Manage sign-in methods, email verification, and the applications
            that can access this account.
          </p>
        </header>
        <div className={styles.identityScene} aria-hidden="true" />
        <div className={styles.settingsGrid}>
          <section className={styles.accountColumn}>
            <header className={styles.regionHeading}>
              <h2>Sign-in methods</h2>
              <p>The independent ways back into this account.</p>
            </header>
            <AccountSettings
              discordNotice={discord ? DISCORD_NOTICES[discord] : undefined}
            />
          </section>
          <div className={styles.linkedColumn}>
            <LinkedInstances />
          </div>
        </div>
      </Shell>
    </section>
  );
}
