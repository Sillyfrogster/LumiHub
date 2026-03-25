import { useState } from 'react';
import { Link } from 'react-router-dom';
import { FileText, MessageSquare, Users, BookOpen, User, Smile, Images } from 'lucide-react';
import { useCharacterImages } from '../../hooks/useCharacterImages';
import type { UnifiedCharacterCard } from '../../types/character';
import type { ChubCharacterCard } from '../../types/chub';
import type { LumiHubCharacter } from '../../types/character';
import type { WorldBookEntry } from '../../types/worldbook';
import styles from './CharacterTabs.module.css';

type TabId = 'overview' | 'prompts' | 'greetings' | 'lorebook' | 'expressions' | 'gallery' | 'creator';

interface TabDef {
  id: TabId;
  label: string;
  icon: React.ElementType;
}

function normalizeImagePath(path: string | null): string | null {
  if (!path) return null;
  let normalized = path.replace(/\\/g, '/');
  if (!normalized.startsWith('uploads/')) {
    normalized = `uploads/${normalized}`;
  }
  return `/${normalized}`;
}

function TextBlock({ label, content }: { label: string; content?: string | null }) {
  if (!content?.trim()) return null;
  return (
    <div className={styles.textBlock}>
      <h4 className={styles.textBlockLabel}>{label}</h4>
      <pre className={styles.textBlockContent}>{content}</pre>
    </div>
  );
}

interface CharacterTabsProps {
  card: UnifiedCharacterCard;
  tabBarClassName?: string;
  tabContentClassName?: string;
}

const CharacterTabs: React.FC<CharacterTabsProps> = ({ card, tabBarClassName, tabContentClassName }) => {
  const [activeTab, setActiveTab] = useState<TabId>('overview');

  const isChub = card.source === 'chub';
  const chubData = isChub ? (card.raw as ChubCharacterCard) : null;
  const lumiData = !isChub ? (card.raw as LumiHubCharacter) : null;

  const description = lumiData?.description || chubData?.description || chubData?.tagline || '';
  const characterBook = lumiData?.character_book as { entries?: WorldBookEntry[] } | null | undefined;
  const lorebookEntries: WorldBookEntry[] = characterBook?.entries ?? [];

  // Fetch character images for LumiHub characters
  const { data: images } = useCharacterImages(lumiData?.id);

  const expressionImages = images?.filter((img) => img.image_type === 'expression') ?? [];
  const galleryImages = images?.filter((img) => img.image_type === 'gallery') ?? [];

  // Lumiverse modules from extensions
  const lumiverseModules = lumiData?.extensions?.lumiverse_modules as {
    alternate_fields?: Record<string, Array<{ id: string; label: string; content: string }>>;
  } | undefined;
  const alternateFields = lumiverseModules?.alternate_fields;
  const hasAltFields = alternateFields && Object.values(alternateFields).some((arr) => arr.length > 0);

  // Build tabs dynamically
  const tabs: TabDef[] = [
    { id: 'overview', label: 'Overview', icon: FileText },
    { id: 'prompts', label: 'Prompts', icon: MessageSquare },
    { id: 'greetings', label: 'Greetings', icon: Users },
    { id: 'lorebook', label: 'Lorebook', icon: BookOpen },
  ];

  if (expressionImages.length > 0) {
    tabs.push({ id: 'expressions', label: 'Expressions', icon: Smile });
  }
  if (galleryImages.length > 0) {
    tabs.push({ id: 'gallery', label: 'Gallery', icon: Images });
  }

  tabs.push({ id: 'creator', label: 'Creator', icon: User });

  return (
    <>
      <div className={`${styles.tabBar} ${tabBarClassName || ''}`}>
        {tabs.map((tab) => (
          <button
            key={tab.id}
            className={`${styles.tab} ${activeTab === tab.id ? styles.tabActive : ''}`}
            onClick={() => setActiveTab(tab.id)}
          >
            <tab.icon size={14} />
            <span>{tab.label}</span>
            {tab.id === 'lorebook' && lorebookEntries.length > 0 && (
              <span className={styles.tabBadge}>{lorebookEntries.length}</span>
            )}
            {tab.id === 'greetings' && lumiData?.alternate_greetings && lumiData.alternate_greetings.length > 0 && (
              <span className={styles.tabBadge}>{lumiData.alternate_greetings.length + 1}</span>
            )}
            {tab.id === 'expressions' && expressionImages.length > 0 && (
              <span className={styles.tabBadge}>{expressionImages.length}</span>
            )}
            {tab.id === 'gallery' && galleryImages.length > 0 && (
              <span className={styles.tabBadge}>{galleryImages.length}</span>
            )}
          </button>
        ))}
      </div>

      <div className={`${styles.tabContent} ${tabContentClassName || ''}`}>
        {activeTab === 'overview' && (
          <div>
            {description ? (
              <pre className={styles.descriptionText}>{description}</pre>
            ) : (
              <p className={styles.emptyText}>No description provided.</p>
            )}
          </div>
        )}

        {activeTab === 'prompts' && (
          <div className={styles.promptsGrid}>
            {isChub ? (
              <p className={styles.emptyText}>Install this character to view full prompt data.</p>
            ) : (
              <>
                <TextBlock label="Personality" content={lumiData?.personality} />
                <TextBlock label="Scenario" content={lumiData?.scenario} />
                <TextBlock label="System Prompt" content={lumiData?.system_prompt} />
                <TextBlock label="Post-History Instructions" content={lumiData?.post_history_instructions} />
                <TextBlock label="Message Examples" content={lumiData?.mes_example} />
                {!lumiData?.personality && !lumiData?.scenario && !lumiData?.system_prompt && !lumiData?.post_history_instructions && !lumiData?.mes_example && (
                  <p className={styles.emptyText}>No prompt data defined for this character.</p>
                )}

                {/* Alternate fields */}
                {hasAltFields && (
                  <div className={styles.altFieldsSection}>
                    <h4 className={styles.altFieldsHeader}>Alternate Fields</h4>
                    {Object.entries(alternateFields!).map(([field, entries]) => (
                      entries.length > 0 && (
                        <div key={field} className={styles.altFieldBlock}>
                          <div className={styles.altFieldBlockHeader}>{field}</div>
                          {entries.map((entry) => (
                            <div key={entry.id} className={styles.altFieldEntry}>
                              <div className={styles.altFieldEntryLabel}>{entry.label}</div>
                              <div className={styles.altFieldEntryContent}>{entry.content}</div>
                            </div>
                          ))}
                        </div>
                      )
                    ))}
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {activeTab === 'greetings' && (
          <div className={styles.greetingsList}>
            {isChub ? (
              <p className={styles.emptyText}>Install this character to view greetings.</p>
            ) : (
              <>
                {lumiData?.first_mes && (
                  <div className={styles.greetingCard}>
                    <div className={styles.greetingHeader}>
                      <span className={styles.greetingLabel}>First Message</span>
                      <span className={styles.greetingBadge}>Default</span>
                    </div>
                    <pre className={styles.greetingContent}>{lumiData.first_mes}</pre>
                  </div>
                )}
                {lumiData?.alternate_greetings?.map((greeting, i) => (
                  <div key={i} className={styles.greetingCard}>
                    <div className={styles.greetingHeader}>
                      <span className={styles.greetingLabel}>Alternate Greeting {i + 1}</span>
                    </div>
                    <pre className={styles.greetingContent}>{greeting}</pre>
                  </div>
                ))}
                {!lumiData?.first_mes && (!lumiData?.alternate_greetings || lumiData.alternate_greetings.length === 0) && (
                  <p className={styles.emptyText}>No greetings defined for this character.</p>
                )}
              </>
            )}
          </div>
        )}

        {activeTab === 'lorebook' && (
          <div className={styles.lorebookList}>
            {lorebookEntries.length > 0 ? (
              lorebookEntries.map((entry, i) => (
                <div key={i} className={styles.loreEntry}>
                  <div className={styles.loreEntryHeader}>
                    <span className={styles.loreEntryName}>{entry.name || entry.comment || `Entry ${i + 1}`}</span>
                    <div className={styles.loreEntryMeta}>
                      {!entry.enabled && <span className={styles.loreDisabled}>Disabled</span>}
                      <span className={styles.lorePriority}>P{entry.priority ?? 0}</span>
                    </div>
                  </div>
                  {entry.keys.length > 0 && (
                    <div className={styles.loreKeys}>
                      {entry.keys.map((key, ki) => (
                        <span key={ki} className={styles.loreKey}>{key}</span>
                      ))}
                    </div>
                  )}
                  <pre className={styles.loreContent}>{entry.content}</pre>
                </div>
              ))
            ) : (
              <p className={styles.emptyText}>
                {isChub ? 'Install this character to view embedded lorebook data.' : 'No embedded lorebook for this character.'}
              </p>
            )}
          </div>
        )}

        {activeTab === 'expressions' && (
          <div>
            {expressionImages.length > 0 ? (
              <div className={styles.expressionsTabGrid}>
                {expressionImages.map((img) => (
                  <div key={img.id} className={styles.expressionTabItem}>
                    <img
                      src={normalizeImagePath(img.file_path) ?? ''}
                      alt={img.label || 'Expression'}
                      className={styles.expressionTabImg}
                      loading="lazy"
                    />
                    <span className={styles.expressionTabLabel}>{img.label || 'Unnamed'}</span>
                  </div>
                ))}
              </div>
            ) : (
              <p className={styles.emptyText}>No expression images.</p>
            )}
          </div>
        )}

        {activeTab === 'gallery' && (
          <div>
            {galleryImages.length > 0 ? (
              <div className={styles.galleryTabGrid}>
                {galleryImages.map((img) => (
                  <img
                    key={img.id}
                    src={normalizeImagePath(img.file_path) ?? ''}
                    alt="Gallery"
                    className={styles.galleryTabImg}
                    loading="lazy"
                  />
                ))}
              </div>
            ) : (
              <p className={styles.emptyText}>No gallery images.</p>
            )}
          </div>
        )}

        {activeTab === 'creator' && (
          <div className={styles.creatorTab}>
            <div className={styles.creatorHeader}>
              <div className={styles.creatorAvatar}>
                {lumiData?.owner?.avatar ? (
                  <img src={lumiData.owner.avatar} alt={card.creator} />
                ) : (
                  <User size={32} />
                )}
              </div>
              <div className={styles.creatorMain}>
                <h3 className={styles.creatorName}>{card.creator}</h3>
                <p className={styles.creatorSubtitle}>
                  {lumiData?.owner ? 'Verified Creator' : 'Guest Contributor'}
                </p>
              </div>
              {card.creatorDiscordId && (
                <Link to={`/user/${card.creatorDiscordId}`} className={styles.viewProfileBtn}>
                  View Profile
                </Link>
              )}
            </div>

            <div className={styles.creatorMeta}>
              <TextBlock label="Creator Notes" content={lumiData?.creator_notes || (isChub ? 'Not available from Chub API.' : 'No notes provided.')} />
              {lumiData?.character_version && (
                <div className={styles.metaRow}>
                  <span className={styles.metaLabel}>Version</span>
                  <span className={styles.metaValue}>{lumiData.character_version}</span>
                </div>
              )}
              {card.createdAt && (
                <div className={styles.metaRow}>
                  <span className={styles.metaLabel}>Uploaded</span>
                  <span className={styles.metaValue}>{new Date(card.createdAt).toLocaleDateString()}</span>
                </div>
              )}
              {lumiData?.creation_date && (
                <div className={styles.metaRow}>
                  <span className={styles.metaLabel}>Created</span>
                  <span className={styles.metaValue}>{new Date(lumiData.creation_date).toLocaleDateString()}</span>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </>
  );
};

export default CharacterTabs;
