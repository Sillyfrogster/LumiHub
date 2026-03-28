import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { fromChub } from '../../types/character';
import { FileText, MessageSquare, Users, BookOpen, User, Smile, Images, Loader2, Code2 } from 'lucide-react';
import { useCharacterImages } from '../../hooks/useCharacterImages';
import { useChubCharacterDetail } from '../../hooks/useChubCharacterDetail';
import { useChubGallery } from '../../hooks/useChubGallery';
import { useChubCreator } from '../../hooks/useChubCreator';
import LazyImage from '../shared/LazyImage';
import Lightbox, { type LightboxImage } from '../shared/Lightbox';
import ScrollFadeRow from '../shared/ScrollFadeRow';
import type { UnifiedCharacterCard } from '../../types/character';
import type { ChubCharacterCard } from '../../types/chub';
import type { LumiHubCharacter } from '../../types/character';
import type { WorldBookEntry } from '../../types/worldbook';
import type { BundledRegexScript } from '../../utils/charxParser';
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

function LoadingBlock() {
  return (
    <div className={styles.loadingBlock}>
      <Loader2 size={18} className={styles.spinner} />
      <span>Loading from Chub...</span>
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
  const [lightbox, setLightbox] = useState<{ images: LightboxImage[]; index: number } | null>(null);
  const navigate = useNavigate();

  const isChub = card.source === 'chub';
  const chubCard = isChub ? (card.raw as ChubCharacterCard) : null;
  const lumiData = !isChub ? (card.raw as LumiHubCharacter) : null;

  // Lazy-load full Chub definition when viewing a Chub character
  const { data: chubDef, isLoading: chubDefLoading } = useChubCharacterDetail(
    isChub ? card.id : undefined,
  );

  const description = lumiData?.description || chubDef?.description || chubCard?.description || chubCard?.tagline || '';
  const personality = lumiData?.personality || chubDef?.personality || '';
  const scenario = lumiData?.scenario || chubDef?.scenario || '';
  const systemPrompt = lumiData?.system_prompt || chubDef?.system_prompt || '';
  const postHistory = lumiData?.post_history_instructions || chubDef?.post_history_instructions || '';
  const mesExample = lumiData?.mes_example || chubDef?.example_dialogs || '';
  const firstMessage = lumiData?.first_mes || chubDef?.first_message || '';
  const alternateGreetings = lumiData?.alternate_greetings || chubDef?.alternate_greetings || [];

  // Lorebook: LumiHub supports multiple world books via lumiverse_modules.world_books,
  // falls back to character_book (single merged book). Chub uses embedded_lorebook.
  const characterBook = lumiData?.character_book as { name?: string; entries?: WorldBookEntry[] } | null | undefined;
  const chubEntries = chubDef?.embedded_lorebook?.entries ?? [];

  // Lumiverse modules from extensions
  const lumiverseModules = lumiData?.extensions?.lumiverse_modules as {
    alternate_fields?: Record<string, Array<{ id: string; label: string; content: string }>>;
    world_books?: Array<{ name: string; description?: string; entries?: WorldBookEntry[] }>;
    regex_scripts?: BundledRegexScript[];
  } | undefined;

  // Build structured lorebook data: array of { name, entries }
  type LorebookGroup = { name: string; entries: WorldBookEntry[] };
  const lorebookGroups: LorebookGroup[] = (() => {
    // Prefer individual world_books from lumiverse_modules (lossless multi-book)
    const moduleBooks = lumiverseModules?.world_books;
    if (moduleBooks && moduleBooks.length > 0) {
      return moduleBooks
        .filter((b) => b.entries && b.entries.length > 0)
        .map((b) => ({ name: b.name || 'Unnamed Lorebook', entries: b.entries! }));
    }
    // Fall back to single character_book
    if (characterBook?.entries && characterBook.entries.length > 0) {
      return [{ name: characterBook.name || 'Lorebook', entries: characterBook.entries }];
    }
    // Fall back to Chub embedded lorebook
    if (chubEntries.length > 0) {
      return [{
        name: 'Lorebook',
        entries: chubEntries.map((e) => ({
          keys: e.keys,
          secondary_keys: e.secondary_keys,
          content: e.content,
          name: e.name,
          comment: e.comment,
          enabled: e.enabled,
          priority: e.priority,
          insertion_order: e.insertion_order,
          case_sensitive: e.case_sensitive,
          selective: e.selective,
          constant: e.constant,
          position: e.position as 'before_char' | 'after_char',
        })),
      }];
    }
    return [];
  })();

  const totalLorebookEntries = lorebookGroups.reduce((sum, g) => sum + g.entries.length, 0);
  const hasMultipleBooks = lorebookGroups.length > 1;
  const [activeBookIdx, setActiveBookIdx] = useState(0);

  // Fetch character images for LumiHub characters
  const { data: images } = useCharacterImages(lumiData?.id);

  // Fetch gallery images for Chub characters
  const { data: chubGallery, isLoading: chubGalleryLoading } = useChubGallery(
    chubCard?.projectId,
    chubCard?.hasGallery,
  );

  // Fetch Chub creator profile
  const { data: chubCreator } = useChubCreator(isChub ? card.creator : undefined);

  const expressionImages = images?.filter((img) => img.image_type === 'expression') ?? [];
  const galleryImages = images?.filter((img) => img.image_type === 'gallery') ?? [];

  const alternateFields = lumiverseModules?.alternate_fields;
  const hasAltFields = alternateFields && Object.values(alternateFields).some((arr) => arr.length > 0);
  const regexScripts = lumiverseModules?.regex_scripts ?? [];

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
  const hasGalleryTab = galleryImages.length > 0 || (chubCard?.hasGallery && (chubGallery?.length ?? 0) > 0);
  if (hasGalleryTab) {
    tabs.push({ id: 'gallery', label: 'Gallery', icon: Images });
  }

  tabs.push({ id: 'creator', label: 'Creator', icon: User });

  const greetingCount = (firstMessage ? 1 : 0) + alternateGreetings.length;

  return (
    <>
      <ScrollFadeRow as="nav" className={`${styles.tabBar} ${tabBarClassName || ''}`} role="tablist" aria-label="Character details">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            role="tab"
            aria-selected={activeTab === tab.id}
            aria-controls={`tabpanel-${tab.id}`}
            id={`tab-${tab.id}`}
            className={`${styles.tab} ${activeTab === tab.id ? styles.tabActive : ''}`}
            onClick={() => setActiveTab(tab.id)}
          >
            <tab.icon size={14} aria-hidden="true" />
            <span>{tab.label}</span>
            {tab.id === 'lorebook' && totalLorebookEntries > 0 && (
              <span className={styles.tabBadge} aria-label={`${totalLorebookEntries} entries`}>{totalLorebookEntries}</span>
            )}
            {tab.id === 'greetings' && greetingCount > 0 && (
              <span className={styles.tabBadge} aria-label={`${greetingCount} greetings`}>{greetingCount}</span>
            )}
            {tab.id === 'expressions' && expressionImages.length > 0 && (
              <span className={styles.tabBadge} aria-label={`${expressionImages.length} expressions`}>{expressionImages.length}</span>
            )}
            {tab.id === 'gallery' && hasGalleryTab && (
              <span className={styles.tabBadge} aria-label={`${galleryImages.length || chubGallery?.length || 0} images`}>
                {galleryImages.length || chubGallery?.length || 0}
              </span>
            )}
          </button>
        ))}
      </ScrollFadeRow>

      <div className={`${styles.tabContent} ${tabContentClassName || ''}`} role="tabpanel" id={`tabpanel-${activeTab}`} aria-labelledby={`tab-${activeTab}`}>
        {activeTab === 'overview' && (
          <div>
            {isChub && chubDefLoading ? (
              <LoadingBlock />
            ) : description ? (
              <pre className={styles.descriptionText}>{description}</pre>
            ) : (
              <p className={styles.emptyText}>No description provided.</p>
            )}
            {regexScripts.length > 0 && (
              <div className={styles.regexSection}>
                <h4 className={styles.regexSectionHeader}>
                  <Code2 size={14} />
                  Bundled Regex Scripts ({regexScripts.length})
                </h4>
                <div className={styles.regexList}>
                  {regexScripts.map((script, i) => (
                    <div key={i} className={styles.regexItem}>
                      <span className={styles.regexName}>{script.name}</span>
                      <span className={styles.regexTarget}>{script.target}</span>
                      <span className={styles.regexPlacement}>
                        {script.placement.join(', ')}
                      </span>
                      {script.disabled && <span className={styles.regexDisabled}>Disabled</span>}
                    </div>
                  ))}
                </div>
                <p className={styles.regexHint}>
                  These regex scripts will be installed alongside the character.
                </p>
              </div>
            )}
          </div>
        )}

        {activeTab === 'prompts' && (
          <div className={styles.promptsGrid}>
            {isChub && chubDefLoading ? (
              <LoadingBlock />
            ) : (
              <>
                <TextBlock label="Personality" content={personality} />
                <TextBlock label="Scenario" content={scenario} />
                <TextBlock label="System Prompt" content={systemPrompt} />
                <TextBlock label="Post-History Instructions" content={postHistory} />
                <TextBlock label="Message Examples" content={mesExample} />
                {!personality && !scenario && !systemPrompt && !postHistory && !mesExample && (
                  <p className={styles.emptyText}>No prompt data defined for this character.</p>
                )}

                {/* Alternate fields (LumiHub only) */}
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
            {isChub && chubDefLoading ? (
              <LoadingBlock />
            ) : (
              <>
                {firstMessage && (
                  <div className={styles.greetingCard}>
                    <div className={styles.greetingHeader}>
                      <span className={styles.greetingLabel}>First Message</span>
                      <span className={styles.greetingBadge}>Default</span>
                    </div>
                    <pre className={styles.greetingContent}>{firstMessage}</pre>
                  </div>
                )}
                {alternateGreetings.map((greeting, i) => (
                  <div key={i} className={styles.greetingCard}>
                    <div className={styles.greetingHeader}>
                      <span className={styles.greetingLabel}>Alternate Greeting {i + 1}</span>
                    </div>
                    <pre className={styles.greetingContent}>{greeting}</pre>
                  </div>
                ))}
                {!firstMessage && alternateGreetings.length === 0 && (
                  <p className={styles.emptyText}>No greetings defined for this character.</p>
                )}
              </>
            )}
          </div>
        )}

        {activeTab === 'lorebook' && (
          <div className={styles.lorebookList}>
            {isChub && chubDefLoading ? (
              <LoadingBlock />
            ) : lorebookGroups.length > 0 ? (
              <>
                {/* Book selector — only shown when multiple books exist */}
                {hasMultipleBooks && (
                  <div className={styles.bookSelector}>
                    <ScrollFadeRow className={styles.bookSelectorRow}>
                      {lorebookGroups.map((group, idx) => (
                        <button
                          key={idx}
                          type="button"
                          className={`${styles.bookPill} ${activeBookIdx === idx ? styles.bookPillActive : ''}`}
                          onClick={() => setActiveBookIdx(idx)}
                        >
                          <BookOpen size={12} aria-hidden="true" />
                          <span className={styles.bookPillName}>{group.name}</span>
                          <span className={styles.bookPillCount}>{group.entries.length}</span>
                        </button>
                      ))}
                    </ScrollFadeRow>
                  </div>
                )}

                {/* Entry list for active book (or all entries when single book) */}
                {(hasMultipleBooks ? [lorebookGroups[activeBookIdx]] : lorebookGroups).map((group, gi) => (
                  <div key={gi}>
                    {!hasMultipleBooks && lorebookGroups.length === 1 && null}
                    {group.entries.map((entry, i) => (
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
                    ))}
                  </div>
                ))}
              </>
            ) : (
              <p className={styles.emptyText}>No embedded lorebook for this character.</p>
            )}
          </div>
        )}

        {activeTab === 'expressions' && (
          <div>
            {expressionImages.length > 0 ? (
              <div className={styles.expressionsTabGrid}>
                {expressionImages.map((img, i) => (
                  <div
                    key={img.id}
                    className={styles.expressionTabItem}
                    onClick={() => setLightbox({
                      images: expressionImages.map((e) => ({ src: normalizeImagePath(e.file_path)!, alt: e.label || 'Expression' })),
                      index: i,
                    })}
                  >
                    <LazyImage
                      src={normalizeImagePath(img.file_path)}
                      alt={img.label || 'Expression'}
                      className={styles.expressionTabImg}
                      containerClassName={styles.expressionTabImgWrap}
                      spinnerSize={16}
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
                {galleryImages.map((img, i) => (
                  <LazyImage
                    key={img.id}
                    src={normalizeImagePath(img.file_path)}
                    alt="Gallery"
                    className={styles.galleryTabImg}
                    containerClassName={styles.galleryTabImgWrap}
                    spinnerSize={18}
                    onClick={() => setLightbox({
                      images: galleryImages.map((g) => ({ src: normalizeImagePath(g.file_path)!, alt: 'Gallery' })),
                      index: i,
                    })}
                  />
                ))}
              </div>
            ) : chubGalleryLoading ? (
              <LoadingBlock />
            ) : chubGallery && chubGallery.length > 0 ? (
              <div className={styles.galleryTabGrid}>
                {chubGallery.map((img, i) => (
                  <LazyImage
                    key={img.uuid}
                    src={img.primary_image_path}
                    alt={img.description || 'Gallery'}
                    className={styles.galleryTabImg}
                    containerClassName={styles.galleryTabImgWrap}
                    spinnerSize={18}
                    onClick={() => setLightbox({
                      images: chubGallery.map((g) => ({ src: g.primary_image_path, alt: g.description || 'Gallery' })),
                      index: i,
                    })}
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
                  <LazyImage src={lumiData.owner.avatar} alt={card.creator} />
                ) : chubCreator?.avatarUrl ? (
                  <LazyImage src={chubCreator.avatarUrl} alt={card.creator} />
                ) : (
                  <User size={32} />
                )}
              </div>
              <div className={styles.creatorMain}>
                <h3 className={styles.creatorName}>{chubCreator?.displayName || card.creator}</h3>
                <p className={styles.creatorSubtitle}>
                  {lumiData?.owner ? 'Verified Creator' : isChub ? 'Chub Creator' : 'Guest Contributor'}
                </p>
              </div>
              {card.creatorDiscordId && (
                <Link to={`/user/${card.creatorDiscordId}`} className={styles.viewProfileBtn}>
                  View Profile
                </Link>
              )}
              {isChub && (
                <a
                  href={`https://chub.ai/users/${card.creator}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={styles.viewProfileBtn}
                >
                  Chub Profile
                </a>
              )}
            </div>

            {/* Chub creator stats */}
            {chubCreator && (
              <div className={styles.creatorStats}>
                <div className={styles.statItem}>
                  <span className={styles.statValue}>{chubCreator.followers.toLocaleString()}</span>
                  <span className={styles.statLabel}>Followers</span>
                </div>
                <div className={styles.statItem}>
                  <span className={styles.statValue}>{chubCreator.following.toLocaleString()}</span>
                  <span className={styles.statLabel}>Following</span>
                </div>
                <div className={styles.statItem}>
                  <span className={styles.statValue}>{chubCreator.projectCount}</span>
                  <span className={styles.statLabel}>Characters</span>
                </div>
              </div>
            )}

            <div className={styles.creatorMeta}>
              <TextBlock label="Creator Notes" content={lumiData?.creator_notes || undefined} />
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

            {/* More by this creator */}
            {chubCreator && chubCreator.topCharacters.length > 0 && (
              <div className={styles.moreByCreator}>
                <h4 className={styles.moreByHeader}>More by {chubCreator.displayName}</h4>
                <div className={styles.moreByGrid}>
                  {chubCreator.topCharacters
                    .filter((c) => c.id !== card.id)
                    .slice(0, 5)
                    .map((c) => (
                      <div
                        key={c.id}
                        className={styles.moreByCard}
                        onClick={() => navigate(`/characters/${encodeURIComponent(c.id)}`, { state: { card: fromChub(c) } })}
                      >
                        <div className={styles.moreByImgWrap}>
                          <LazyImage
                            src={c.avatarUrl}
                            alt={c.name}
                            className={styles.moreByImg}
                          />
                        </div>
                        <div className={styles.moreByInfo}>
                          <span className={styles.moreByName}>{c.name}</span>
                          <span className={styles.moreByStars}>{c.starCount?.toLocaleString() ?? 0} stars</span>
                        </div>
                      </div>
                    ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {lightbox && (
        <Lightbox
          images={lightbox.images}
          startIndex={lightbox.index}
          onClose={() => setLightbox(null)}
        />
      )}
    </>
  );
};

export default CharacterTabs;
