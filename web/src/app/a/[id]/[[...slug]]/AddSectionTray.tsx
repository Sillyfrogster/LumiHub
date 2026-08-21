"use client";

import { Check, Plus, Search, X } from "lucide-react";
import { useId, useMemo, useState } from "react";
import type { AddableSection, AssetBlock, ElementType } from "@/lib/api/query";
import styles from "./AddSectionTray.module.css";

type SectionOffer = { section: AddableSection; present: boolean };
type SectionGroup = { key: string; title: string; offers: SectionOffer[] };

export function AddSectionTray({
  sections,
  blocks,
  pending,
  onAdd,
  onClose,
}: {
  sections: AddableSection[];
  blocks: AssetBlock[];
  pending: boolean;
  onAdd: (definition: string, elementType: ElementType) => void;
  onClose: () => void;
}) {
  const [search, setSearch] = useState("");
  const searchId = useId();

  const offers = useMemo(() => {
    const present = new Set(blocks.map((block) => block.definition));
    return sections.map((section) => ({
      section,
      present: !section.repeatable && present.has(section.definition),
    }));
  }, [sections, blocks]);
  const wanted = search.trim().toLowerCase();
  const groups = useMemo(() => {
    const grouped = new Map<string, SectionGroup>();
    for (const offer of offers) {
      const { section } = offer;
      if (
        wanted &&
        !section.title.toLowerCase().includes(wanted) &&
        !section.summary.toLowerCase().includes(wanted)
      ) {
        continue;
      }
      const group = grouped.get(section.group);
      if (group) group.offers.push(offer);
      else {
        grouped.set(section.group, {
          key: section.group,
          title: section.groupTitle,
          offers: [offer],
        });
      }
    }
    return [...grouped.values()];
  }, [offers, wanted]);

  if (sections.length === 0) return null;

  return (
    <section className={styles.tray} aria-labelledby="add-block-title">
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
          Search the sections
        </label>
        <input
          id={searchId}
          type="search"
          value={search}
          placeholder="Search sections"
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
            {group.offers.map(({ section, present }) => (
              <li key={section.definition} data-present={present || undefined}>
                <div className={styles.itemText}>
                  <strong>{section.title}</strong>
                  <span>{section.summary}</span>
                  {present ? (
                    <span className={styles.alreadyOn}>
                      <Check size={14} aria-hidden="true" />
                      Already on this page
                    </span>
                  ) : null}
                </div>
                {present ? null : (
                  <div className={styles.itemActions}>
                    {section.choices.length === 1 ? (
                      <button
                        type="button"
                        className={styles.add}
                        disabled={pending}
                        onClick={() =>
                          onAdd(section.definition, section.choices[0].type)
                        }
                      >
                        <Plus size={16} aria-hidden="true" />
                        Add
                      </button>
                    ) : (
                      <>
                        <span className={styles.choiceLabel}>Start with</span>
                        <div className={styles.choices}>
                          {section.choices.map((choice) => (
                            <button
                              type="button"
                              key={choice.type}
                              disabled={pending}
                              onClick={() =>
                                onAdd(section.definition, choice.type)
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
