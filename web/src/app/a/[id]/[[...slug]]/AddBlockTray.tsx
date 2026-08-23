"use client";

import { Check, Plus, Search, X } from "lucide-react";
import { useId, useMemo, useState } from "react";
import type { AddableBlock, AssetBlock, ElementType } from "@/lib/api/query";
import styles from "./AddBlockTray.module.css";

type Offer = { addable: AddableBlock; present: boolean };
type OfferGroup = { key: string; title: string; offers: Offer[] };

export function AddBlockTray({
  addable,
  blocks,
  pending,
  onAdd,
  onClose,
}: {
  addable: AddableBlock[];
  blocks: AssetBlock[];
  pending: boolean;
  onAdd: (definition: string, elementType: ElementType) => void;
  onClose: () => void;
}) {
  const [search, setSearch] = useState("");
  const searchId = useId();

  const offers = useMemo(() => {
    const present = new Set(blocks.map((block) => block.definition));
    return addable.map((candidate) => ({
      addable: candidate,
      present: !candidate.repeatable && present.has(candidate.definition),
    }));
  }, [addable, blocks]);
  const wanted = search.trim().toLowerCase();
  const groups = useMemo(() => {
    const grouped = new Map<string, OfferGroup>();
    for (const offer of offers) {
      const { addable: candidate } = offer;
      if (
        wanted &&
        !candidate.title.toLowerCase().includes(wanted) &&
        !candidate.summary.toLowerCase().includes(wanted)
      ) {
        continue;
      }
      const group = grouped.get(candidate.group);
      if (group) group.offers.push(offer);
      else {
        grouped.set(candidate.group, {
          key: candidate.group,
          title: candidate.groupTitle,
          offers: [offer],
        });
      }
    }
    return [...grouped.values()];
  }, [offers, wanted]);

  if (addable.length === 0) return null;

  return (
    <section
      id="add-block-tray"
      className={styles.tray}
      aria-labelledby="add-block-title"
    >
      <header className={styles.topline}>
        <div>
          <p className={styles.context}>Add a block</p>
          <h2 id="add-block-title">What goes on this page</h2>
          <p>
            Blocks are grouped by where their content ends up. Nothing here is a
            decision you have to make now.
          </p>
        </div>
        <button type="button" className={styles.close} onClick={onClose}>
          <X size={17} aria-hidden="true" />
          Close
        </button>
      </header>

      <div className={styles.search}>
        <Search size={16} aria-hidden="true" />
        <label className={styles.srOnly} htmlFor={searchId}>
          Search the blocks
        </label>
        <input
          id={searchId}
          type="search"
          value={search}
          placeholder="Search blocks"
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>

      {groups.length === 0 ? (
        <p className={styles.nothing}>
          No block is named that. Try “images”, “notes” or “links”.
        </p>
      ) : null}

      {groups.map((group) => (
        <section className={styles.group} key={group.key}>
          <h3>{group.title}</h3>
          <ul>
            {group.offers.map(({ addable: candidate, present }) => (
              <li
                key={candidate.definition}
                data-present={present || undefined}
              >
                <div className={styles.itemText}>
                  <strong>{candidate.title}</strong>
                  <span>{candidate.summary}</span>
                  {present ? (
                    <span className={styles.alreadyOn}>
                      <Check size={14} aria-hidden="true" />
                      Already on this page
                    </span>
                  ) : null}
                </div>
                {present ? null : (
                  <div className={styles.itemActions}>
                    {candidate.choices.length === 1 ? (
                      <button
                        type="button"
                        className={styles.add}
                        disabled={pending}
                        onClick={() =>
                          onAdd(candidate.definition, candidate.choices[0].type)
                        }
                      >
                        <Plus size={16} aria-hidden="true" />
                        Add
                      </button>
                    ) : (
                      <>
                        <span className={styles.choiceLabel}>Start with</span>
                        <div className={styles.choices}>
                          {candidate.choices.map((choice) => (
                            <button
                              type="button"
                              key={choice.type}
                              disabled={pending}
                              onClick={() =>
                                onAdd(candidate.definition, choice.type)
                              }
                            >
                              {choice.label}
                            </button>
                          ))}
                        </div>
                      </>
                    )}
                  </div>
                )}
              </li>
            ))}
          </ul>
        </section>
      ))}
    </section>
  );
}
