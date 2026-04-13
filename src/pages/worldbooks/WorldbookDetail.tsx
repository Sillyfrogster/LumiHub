import { useEffect, useState } from 'react';
import { useParams, useLocation, useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, BookOpen, Star, FileText, ExternalLink, Search, ChevronDown, ChevronUp, Trash2, Download, Heart, ThumbsUp } from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useAuth } from '../../hooks/useAuth';
import { getChubLorebookDetail } from '../../api/chub';
import { getWorldbook, deleteWorldbook, viewWorldbook } from '../../api/worldbooks';
import { fromLumiHub, normalizeWorldbookEntries } from '../../types/worldbook';
import type { UnifiedWorldBook, ChubWorldBook, LumiWorldBook, WorldBookEntry } from '../../types/worldbook';
import InstallButton from '../../components/characters/InstallButton';
import LazyImage from '../../components/shared/LazyImage';
import ScrollFadeRow from '../../components/shared/ScrollFadeRow';
import styles from './WorldbookDetail.module.css';

const WorldbookDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const navigate = useNavigate();

  const { user } = useAuth();
  const queryClient = useQueryClient();

  const passedBook = (location.state as { worldbook?: UnifiedWorldBook })?.worldbook ?? null;
  const [book, setBook] = useState<UnifiedWorldBook | null>(passedBook);
  const [loading, setLoading] = useState(!passedBook);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [expandedEntries, setExpandedEntries] = useState<Set<number>>(new Set());
  const [deleting, setDeleting] = useState(false);

  const isChub = book?.source === 'chub';
  const chubData = isChub ? (book?.raw as ChubWorldBook) : null;
  const lumiData = !isChub && book ? (book.raw as LumiWorldBook) : null;

  // Fetch full Chub lorebook entries
  const { data: chubDef, isLoading: chubLoading } = useQuery({
    queryKey: ['chub-lorebook-detail', book?.id],
    queryFn: () => getChubLorebookDetail(book!.id),
    enabled: !!book && isChub,
    staleTime: 5 * 60_000,
  });

  useEffect(() => {
    if (passedBook || !id) return;
    setLoading(true);
    getWorldbook(id)
      .then((res) => setBook(fromLumiHub(res.data)))
      .catch(() => setError('Worldbook not found.'))
      .finally(() => setLoading(false));
  }, [id, passedBook]);

  // Fire view increment once per LumiHub worldbook visit
  useEffect(() => {
    if (lumiData?.id) viewWorldbook(lumiData.id);
  }, [lumiData?.id]);

  if (loading) {
    return (
      <div className={styles.page}>
        <div className={styles.loadingState}>
          <div className={styles.loadingSkeleton} />
        </div>
      </div>
    );
  }

  if (error || !book) {
    return (
      <div className={styles.page}>
        <div className={styles.errorState}>
          <p>{error || 'Worldbook not found.'}</p>
          <button className={styles.backBtnLarge} onClick={() => navigate('/worldbooks')}>
            <ArrowLeft size={16} /> Back to Worldbooks
          </button>
        </div>
      </div>
    );
  }

  const isOwner = !isChub && user && lumiData && (lumiData as any).owner?.id === user.id;

  const handleDelete = async () => {
    if (!lumiData || !confirm(`Delete "${book.name}"? This cannot be undone.`)) return;
    setDeleting(true);
    try {
      await deleteWorldbook(lumiData.id);
      queryClient.invalidateQueries({ queryKey: ['worldbooks'] });
      navigate('/worldbooks');
    } catch (err) {
      console.error('Delete failed:', err);
      setDeleting(false);
    }
  };

  // Merge entries from different sources
  const chubEntries: WorldBookEntry[] = chubDef?.embedded_lorebook?.entries?.map((e: any) => ({
    keys: e.keys || [],
    secondary_keys: e.secondary_keys || [],
    content: e.content || '',
    name: e.name || e.comment || '',
    comment: e.comment,
    enabled: e.enabled ?? true,
    priority: e.priority ?? 0,
    insertion_order: e.insertion_order ?? 100,
    selective: e.selective,
    constant: e.constant,
  })) ?? [];

  const lumiEntries: WorldBookEntry[] = normalizeWorldbookEntries(lumiData?.entries);
  const entries = isChub ? chubEntries : lumiEntries;
  const entryCount = entries.length || book.entryCount;

  // Filter entries by search
  const filtered = searchQuery.trim()
    ? entries.filter((e) => {
        const q = searchQuery.toLowerCase();
        return (
          e.keys.some((k) => k.toLowerCase().includes(q)) ||
          (e.name && e.name.toLowerCase().includes(q)) ||
          e.content.toLowerCase().includes(q)
        );
      })
    : entries;

  const toggleEntry = (idx: number) => {
    setExpandedEntries((prev) => {
      const next = new Set(prev);
      if (next.has(idx)) next.delete(idx);
      else next.add(idx);
      return next;
    });
  };

  const formattedStars = book.downloads > 1000
    ? `${(book.downloads / 1000).toFixed(1)}k`
    : String(book.downloads);

  const formattedTokens = book.tokenCount > 1000
    ? `${(book.tokenCount / 1000).toFixed(1)}k`
    : String(book.tokenCount);

  const chubPageUrl = chubData?.fullPath
    ? `https://chub.ai/lorebooks/${chubData.fullPath.replace(/^lorebooks\//, '')}`
    : null;

  return (
    <main className={styles.page} aria-label={`${book.name} worldbook details`}>
      <button className={styles.backBtn} onClick={() => navigate(-1)} aria-label="Go back">
        <ArrowLeft size={16} aria-hidden="true" />
        <span>Back</span>
      </button>

      <div className={styles.layout}>
        <div className={styles.imageColumn}>
          <div className={styles.imageWrap}>
            {book.avatarUrl ? (
              <LazyImage
                src={book.avatarUrl}
                alt={book.name}
                className={styles.image}
                fallback={<div className={styles.imagePlaceholder}><BookOpen size={64} /></div>}
              />
            ) : (
              <div className={styles.imagePlaceholder}><BookOpen size={64} /></div>
            )}
          </div>
        </div>

        <div className={styles.contentColumn}>
          <h1 className={styles.name}>{book.name}</h1>
          <p className={styles.creator}>
            by{' '}
            {book.creatorDiscordId ? (
              <Link to={`/user/${book.creatorDiscordId}`} className={styles.creatorLink}>
                {book.creator}
              </Link>
            ) : (
              book.creator
            )}
          </p>

          <ScrollFadeRow className={styles.stats}>
            {entryCount > 0 && (
              <span className={styles.statChip}><BookOpen size={13} />{entryCount} entries</span>
            )}
            {book.tokenCount > 0 && (
              <span className={styles.statChip}><FileText size={13} />{formattedTokens} tokens</span>
            )}
            {isChub ? (
              <>
                {chubData && (chubData as any).n_favorites > 0 && (
                  <span className={styles.statChip}><Heart size={13} />{((chubData as any).n_favorites as number).toLocaleString()} favorites</span>
                )}
                {book.downloads > 0 && (
                  <span className={styles.statChip}><Star size={13} />{formattedStars} stars</span>
                )}
                {book.rating !== null && book.rating > 0 && (
                  <span className={styles.statChip}><ThumbsUp size={13} />{book.rating.toFixed(1)}/5 rating</span>
                )}
              </>
            ) : (
              <>
                {book.downloads > 0 && (
                  <span className={styles.statChip}><Download size={13} />{book.downloads} downloads</span>
                )}
              </>
            )}
          </ScrollFadeRow>

          {book.tags.length > 0 && (
            <ScrollFadeRow className={styles.tagsRow}>
              {book.tags.map((tag, i) => (
                <span key={i} className={styles.tag}>{tag}</span>
              ))}
            </ScrollFadeRow>
          )}

          <div className={styles.actions}>
            {isChub ? (
              <>
                <InstallButton characterId={book.id} source="chub" worldBook={book} className={styles.installBtn} />
                {chubPageUrl && (
                  <a href={chubPageUrl} target="_blank" rel="noreferrer" className={styles.secondaryBtn}>
                    <ExternalLink size={14} /> View on Chub
                  </a>
                )}
              </>
            ) : (
              <>
                <InstallButton characterId={book.id} source="lumihub" worldBook={book} className={styles.installBtn} />
                {isOwner && (
                  <button className={styles.dangerBtn} onClick={handleDelete} disabled={deleting}>
                    <Trash2 size={14} />
                    {deleting ? 'Deleting...' : 'Delete'}
                  </button>
                )}
              </>
            )}
          </div>

          {/* Description */}
          {book.description && (
            <div className={styles.descriptionBlock}>
              <pre className={styles.descriptionText}>{book.description}</pre>
            </div>
          )}

          {/* Entries section */}
          <div className={styles.entriesSection}>
            <div className={styles.entriesHeader}>
              <h2 className={styles.entriesTitle}>
                <BookOpen size={16} />
                Lorebook Entries
                {filtered.length > 0 && (
                  <span className={styles.entriesCount}>{filtered.length}</span>
                )}
              </h2>

              {entries.length > 5 && (
                <div className={styles.searchWrap}>
                  <Search size={14} className={styles.searchIcon} aria-hidden="true" />
                  <input
                    type="search"
                    placeholder="Search entries..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className={styles.searchInput}
                    aria-label="Search lorebook entries"
                  />
                </div>
              )}
            </div>

            {chubLoading ? (
              <div className={styles.entriesLoading}>
                <div className={styles.loadingSkeleton} style={{ height: 200 }} />
              </div>
            ) : filtered.length > 0 ? (
              <div className={styles.entriesList}>
                {filtered.map((entry, i) => {
                  const isExpanded = expandedEntries.has(i);
                  const isLong = entry.content.length > 200;

                  return (
                    <div key={i} className={styles.entryCard}>
                      <div
                        className={styles.entryHeader}
                        onClick={() => isLong && toggleEntry(i)}
                        style={isLong ? { cursor: 'pointer' } : undefined}
                      >
                        <div className={styles.entryTitleRow}>
                          <span className={styles.entryName}>
                            {entry.name || entry.comment || `Entry ${i + 1}`}
                          </span>
                          <div className={styles.entryMeta}>
                            {entry.constant && <span className={styles.entryBadge}>Constant</span>}
                            {entry.selective && <span className={styles.entryBadge}>Selective</span>}
                            {!entry.enabled && <span className={styles.entryBadgeDisabled}>Disabled</span>}
                            <span className={styles.entryPriority}>P{entry.priority ?? 0}</span>
                          </div>
                        </div>

                        {entry.keys.length > 0 && (
                          <div className={styles.entryKeys}>
                            {entry.keys.map((key, ki) => (
                              <span key={ki} className={styles.entryKey}>{key}</span>
                            ))}
                            {entry.secondary_keys && entry.secondary_keys.length > 0 && (
                              <>
                                <span className={styles.entryKeySeparator}>+</span>
                                {entry.secondary_keys.map((key, ki) => (
                                  <span key={`s${ki}`} className={styles.entryKeySecondary}>{key}</span>
                                ))}
                              </>
                            )}
                          </div>
                        )}
                      </div>

                      <pre className={`${styles.entryContent} ${!isExpanded && isLong ? styles.entryContentClamped : ''}`}>
                        {entry.content}
                      </pre>

                      {isLong && (
                        <button
                          className={styles.expandBtn}
                          onClick={() => toggleEntry(i)}
                          aria-expanded={isExpanded}
                          aria-label={isExpanded ? `Collapse ${entry.name || `entry ${i + 1}`}` : `Expand ${entry.name || `entry ${i + 1}`}`}
                        >
                          {isExpanded ? <ChevronUp size={14} aria-hidden="true" /> : <ChevronDown size={14} aria-hidden="true" />}
                          {isExpanded ? 'Show less' : 'Show more'}
                        </button>
                      )}
                    </div>
                  );
                })}
              </div>
            ) : entries.length === 0 ? (
              <p className={styles.emptyText}>
                {chubLoading ? 'Loading entries...' : 'No entries available for this worldbook.'}
              </p>
            ) : (
              <p className={styles.emptyText}>No entries match your search.</p>
            )}
          </div>
        </div>
      </div>
    </main>
  );
};

export default WorldbookDetail;
