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
}: {
  sections: AddableSection[];
  blocks: AssetBlock[];
  pending: boolean;
  onAdd: (definition: string, elementType: ElementType) => void;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const searchId = useId();

  const offers = useMemo(() => {
    const present = new Set(blocks.map((block) => block.definition));
    return sections.map((section) => ({
      section,
      present: !section.repeatable && present.has(section.definition),
    }));
  }, [sections, blocks]);
  const available = offers.filter((offer) => !offer.present).length;

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

  if (!open) {
    return (
      <div className={styles.tray}>
        <button
          type="button"
          className={styles.opener}
          aria-expanded={false}
          onClick={() => setOpen(true)}
        >
          <span className={styles.openerMark} aria-hidden="true">
            <Plus size={18} />
          </span>
          <span className={styles.openerText}>
            <strong>Add a section</strong>
            {available === 0
              ? "Every section is on the page. A custom one can go on at any time."
              : `${countWord(available)} to choose from, and you can add them at any time.`}
          </span>
        </button>
      </div>
    );
  }

  return (
    <section className={styles.tray} aria-labelledby="add-section-title">
      <header className={styles.topline}>
        <div>
          <p className={styles.context}>Add a section</p>
          <h2 id="add-section-title">What goes on this page</h2>
          <p>
            Sections are grouped by where their content ends up. Nothing here is
            a decision you have to make now.
          </p>
        </div>
        <button
          type="button"
          className={styles.close}
          onClick={() => setOpen(false)}
        >
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
          No section is named that. Try “images”, “notes” or “links”.
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

const COUNT_WORDS = [
  "No sections",
  "One section",
  "Two sections",
  "Three sections",
  "Four sections",
  "Five sections",
  "Six sections",
  "Seven sections",
  "Eight sections",
  "Nine sections",
  "Ten sections",
];

function countWord(count: number): string {
  return COUNT_WORDS[count] ?? `${count} sections`;
}
