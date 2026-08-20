import { Art } from "@/components/art/Art";
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
      <Art name="wash" width={760} className={styles.wash} />
      <Art name="sprig" width={250} className={styles.sprig} loading="eager" />
      <Shell className={styles.layout}>
        <header className={styles.heading}>
          <h1>Account settings</h1>
          <p>
            Keep two independent ways back when you can. Discord and email may
            reach one account, but two accounts are never combined.
          </p>
        </header>
        <AccountSettings
          discordNotice={discord ? DISCORD_NOTICES[discord] : undefined}
        />
        <LinkedInstances />
      </Shell>
    </section>
  );
}
