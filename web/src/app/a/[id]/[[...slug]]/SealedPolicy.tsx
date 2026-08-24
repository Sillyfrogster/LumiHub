"use client";

import { ShieldCheck } from "lucide-react";
import type { AssetElement } from "@/lib/api/query";
import { protectedAppLabel } from "@/lib/protected-apps";
import styles from "./SealedPolicy.module.css";

export type AllowedApp = "lumiverse";

/** The allowed-app choice every surface that can seal a prompt has to offer. */
export type SealedPolicyState = {
  allowedApps: AllowedApp[];
  eligibleApps: AllowedApp[];
  onChange: (apps: AllowedApp[]) => void;
};

export const NO_ALLOWED_APP =
  "Choose at least one allowed app before saving a sealed prompt.";

export function hasSealedPrompts(elements: AssetElement[]): boolean {
  return elements.some(elementSealsAPrompt);
}

export function elementSealsAPrompt(element: AssetElement): boolean {
  return (
    element.type === "prompt_list" &&
    "fragments" in element.content &&
    element.content.fragments.some((fragment) => fragment.protected)
  );
}

export function SealedPolicy({
  policy,
  pending,
  unanswered,
  innerRef,
}: {
  policy: SealedPolicyState;
  pending: boolean;
  unanswered: boolean;
  innerRef?: React.Ref<HTMLFieldSetElement>;
}) {
  return (
    <fieldset
      ref={innerRef}
      className={`${styles.policy} ${unanswered ? styles.unanswered : ""}`}
      aria-describedby="sealed-policy-note"
    >
      <legend>
        <ShieldCheck size={15} aria-hidden="true" />
        Allowed apps
      </legend>
      <p id="sealed-policy-note">
        A sealed prompt leaves Illarin only through a linked application you
        allow here, and it arrives as plain text. Sealing is not encryption.
      </p>
      {policy.eligibleApps.length > 0 ? (
        <div className={styles.apps}>
          {policy.eligibleApps.map((app) => {
            const chosen = policy.allowedApps.includes(app);
            return (
              <label
                key={app}
                className={styles.app}
                data-chosen={chosen || undefined}
              >
                <input
                  type="checkbox"
                  checked={chosen}
                  onChange={(event) =>
                    policy.onChange(
                      event.target.checked
                        ? [
                            ...policy.allowedApps.filter((one) => one !== app),
                            app,
                          ]
                        : policy.allowedApps.filter((one) => one !== app),
                    )
                  }
                  disabled={pending}
                />
                {protectedAppLabel(app)}
              </label>
            );
          })}
        </div>
      ) : (
        <p className={styles.none}>
          No linked app can receive this preset in its current form.
        </p>
      )}
      {unanswered ? (
        <p className={styles.unansweredNote} role="alert">
          {NO_ALLOWED_APP}
        </p>
      ) : null}
    </fieldset>
  );
}
