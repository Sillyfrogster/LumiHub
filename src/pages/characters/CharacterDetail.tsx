import { useEffect, useState } from 'react';
import { useParams, useLocation, useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, Download, Star, Heart, Users, ExternalLink, Image, FileText, Package, Trash2, ThumbsUp } from 'lucide-react';
import { useCharacterImages } from '../../hooks/useCharacterImages';
import { useAuth } from '../../hooks/useAuth';
import { useQueryClient } from '@tanstack/react-query';
import type { UnifiedCharacterCard } from '../../types/character';
import type { ChubCharacterCard } from '../../types/chub';
import type { LumiHubCharacter } from '../../types/character';
import { getCharacter, deleteCharacter, viewCharacter } from '../../api/characters';
import { searchChubCharacters, transformChubCharacter } from '../../api/chub';
import { fromLumiHub, fromChub } from '../../types/character';
import CharacterTabs from '../../components/characters/CharacterTabs';
import InstallButton from '../../components/characters/InstallButton';
import LazyImage from '../../components/shared/LazyImage';
import Lightbox from '../../components/shared/Lightbox';
import ScrollFadeRow from '../../components/shared/ScrollFadeRow';
import styles from './CharacterDetail.module.css';

function normalizeImagePath(p: string | null): string | null {
  if (!p) return null;
  let n = p.replace(/\\/g, '/');
  if (!n.startsWith('uploads/')) n = `uploads/${n}`;
  return `/${n}`;
}

const CharacterDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const passedCard = (location.state as { card?: UnifiedCharacterCard })?.card ?? null;

  const [card, setCard] = useState<UnifiedCharacterCard | null>(passedCard);
  const [loading, setLoading] = useState(!passedCard);
  const [error, setError] = useState<string | null>(null);
  const [heroUrl, setHeroUrl] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [heroLightbox, setHeroLightbox] = useState(false);

  // Sync card state when navigating between cards (same component, new route state).
  // Keyed on `id` so it only fires when the URL actually changes.
  useEffect(() => {
    if (!passedCard) return;
    setCard(passedCard);
    setLoading(false);
    setError(null);
    setHeroUrl(null);
    setHeroLightbox(false);
  }, [id]); // eslint-disable-line react-hooks/exhaustive-deps

  // Derive character data (may be null while loading)
  const isChub = card?.source === 'chub';
  const chubData = isChub ? (card?.raw as ChubCharacterCard) : null;
  const lumiData = !isChub && card ? (card.raw as LumiHubCharacter) : null;

  // Hooks must be called unconditionally (before any early returns)
  const { data: images } = useCharacterImages(lumiData?.id);

  // Fire view increment once per UUID card visit
  useEffect(() => {
    if (lumiData?.id) viewCharacter(lumiData.id);
  }, [lumiData?.id]);

  useEffect(() => {
    if (passedCard || !id) return;

    const decodedId = decodeURIComponent(id);
    const isUUID = /^[0-9a-f-]{36}$/i.test(decodedId);

    setLoading(true);

    if (isUUID) {
      getCharacter(decodedId)
        .then((res) => setCard(fromLumiHub(res.data)))
        .catch(() => setError('Character not found.'))
        .finally(() => setLoading(false));
    } else {
      // Chub fullPath (creator/card-name) — search for the exact card
      const chubPath = decodedId;
      searchChubCharacters({ search: chubPath, limit: 10 })
        .then((result) => {
          const match = result.nodes.find((n) => n.fullPath === chubPath);
          if (match) {
            setCard(fromChub(transformChubCharacter(match)));
          } else {
            setError('Character not found on Chub.');
          }
        })
        .catch(() => setError('Failed to load character from Chub.'))
        .finally(() => setLoading(false));
    }
  }, [id, passedCard]);

  if (loading) {
    return (
      <div className={styles.page}>
        <div className={styles.loadingState}>
          <div className={styles.loadingSkeleton} />
        </div>
      </div>
    );
  }

  if (error || !card) {
    return (
      <div className={styles.page}>
        <div className={styles.errorState}>
          <p>{error || 'Character not found.'}</p>
          <button className={styles.backBtnLarge} onClick={() => navigate('/characters')}>
            <ArrowLeft size={16} /> Back to Characters
          </button>
        </div>
      </div>
    );
  }

  const altAvatars = images?.filter((img) => img.image_type === 'avatar_alt') ?? [];
  const hasCharxAssets = (images?.length ?? 0) > 1;
  const isOwner = !isChub && user && lumiData?.owner?.id === user.id;

  // Detect embedded lorebook for the install dropdown (single character_book or multi world_books)
  const lumiCharBook = lumiData?.character_book as { entries?: unknown[] } | null | undefined;
  const lumiWorldBooks = (lumiData?.extensions?.lumiverse_modules as any)?.world_books as Array<{ entries?: unknown[] }> | undefined;
  const chubDef = isChub ? (card.raw as any)?.definition : null;
  const chubHasLorebook = !!(chubDef?.character_book?.entries?.length || chubDef?.embedded_lorebook?.entries?.length);
  const hasEmbeddedLorebook = chubHasLorebook
    || (lumiCharBook?.entries?.length ?? 0) > 0
    || (lumiWorldBooks?.some((b) => (b.entries?.length ?? 0) > 0) ?? false);

  const displayAvatar = heroUrl || card.avatarUrl;

  const formattedDownloads = card.downloads > 1000
    ? `${(card.downloads / 1000).toFixed(1)}k`
    : String(card.downloads);

  const handleDownloadPng = () => {
    if (card.avatarUrl) {
      const a = document.createElement('a');
      a.href = card.avatarUrl;
      a.download = `${card.name}_card.png`;
      a.click();
    }
  };

  const handleDelete = async () => {
    if (!lumiData || !confirm(`Delete "${card.name}"? This cannot be undone.`)) return;
    setDeleting(true);
    try {
      await deleteCharacter(lumiData.id);
      queryClient.invalidateQueries({ queryKey: ['characters'] });
      navigate('/characters');
    } catch (err) {
      console.error('Delete failed:', err);
      setDeleting(false);
    }
  };

  return (
    <main className={styles.page} aria-label={`${card.name} character details`}>
      <button className={styles.backBtn} onClick={() => navigate(-1)} aria-label="Go back">
        <ArrowLeft size={16} aria-hidden="true" />
        <span>Back</span>
      </button>

      <div className={styles.layout}>
        <div className={styles.imageColumn}>
          <div className={styles.imageWrap}>
            {displayAvatar ? (
              <LazyImage
                src={displayAvatar}
                alt={card.name}
                className={styles.image}
                fallback={<div className={styles.imagePlaceholder}>{card.name.charAt(0)}</div>}
                onClick={() => setHeroLightbox(true)}
                style={{ cursor: 'pointer' }}
              />
            ) : (
              <div className={styles.imagePlaceholder}>{card.name.charAt(0)}</div>
            )}
            {altAvatars.length > 0 && (
              <div className={styles.altAvatarThumbs}>
                <img
                  src={card.avatarUrl ?? ''}
                  alt="Default"
                  className={`${styles.altAvatarThumb} ${!heroUrl ? styles.altAvatarThumbActive : ''}`}
                  onClick={() => setHeroUrl(null)}
                />
                {altAvatars.map((img) => (
                  <img
                    key={img.id}
                    src={normalizeImagePath(img.file_path) ?? ''}
                    alt={img.label || 'Alt'}
                    title={img.label || undefined}
                    className={`${styles.altAvatarThumb} ${heroUrl === normalizeImagePath(img.file_path) ? styles.altAvatarThumbActive : ''}`}
                    onClick={() => setHeroUrl(normalizeImagePath(img.file_path))}
                  />
                ))}
              </div>
            )}
          </div>
        </div>

        <div className={styles.contentColumn}>
          <h1 className={styles.name}>{card.name}</h1>
          <p className={styles.creator}>
            by{' '}
            {card.creatorDiscordId ? (
              <Link to={`/user/${card.creatorDiscordId}`} className={styles.creatorLink}>
                {card.creator}
              </Link>
            ) : (
              card.creator
            )}
            {lumiData?.character_version && (
              <span className={styles.version}> · v{lumiData.character_version}</span>
            )}
          </p>

          <ScrollFadeRow className={styles.stats}>
            {isChub ? (
              <>
                {card.downloads > 0 && (
                  <span className={styles.statChip}><Download size={13} />{card.downloads.toLocaleString()} downloads</span>
                )}
                {(card.favorites ?? 0) > 0 && (
                  <span className={styles.statChip}><Heart size={13} />{card.favorites!.toLocaleString()} favorites</span>
                )}
                {(card.stars ?? 0) > 0 && (
                  <span className={styles.statChip}><Star size={13} />{card.stars!.toLocaleString()} stars</span>
                )}
                {card.rating !== null && (
                  <span className={styles.statChip}><ThumbsUp size={13} />{card.rating.toFixed(1)}/5 rating</span>
                )}
                {chubData?.chats !== undefined && (
                  <span className={styles.statChip}><Users size={13} />{chubData.chats.toLocaleString()} chats</span>
                )}
                {chubData?.tokenCount !== undefined && (
                  <span className={styles.statChip}><FileText size={13} />{chubData.tokenCount.toLocaleString()} tokens</span>
                )}
              </>
            ) : (
              <>
                <span className={styles.statChip}><Download size={13} />{formattedDownloads} downloads</span>
              </>
            )}
          </ScrollFadeRow>

          {card.tags.length > 0 && (
            <ScrollFadeRow className={styles.tagsRow}>
              {card.tags.map((tag, i) => (
                <span key={i} className={styles.tag}>{tag}</span>
              ))}
            </ScrollFadeRow>
          )}

          <div className={styles.actions}>
            {isChub ? (
              <>
                <InstallButton
                  characterId={card.id}
                  source="chub"
                  card={card}
                  hasEmbeddedLorebook={hasEmbeddedLorebook}
                  className={styles.installBtn}
                />
                <a href={chubData?.pageUrl} target="_blank" rel="noreferrer" className={styles.secondaryBtn}>
                  <ExternalLink size={14} /> View on Chub
                </a>
              </>
            ) : (
              <>
                <InstallButton
                  characterId={card.id}
                  source="lumihub"
                  card={card}
                  hasEmbeddedLorebook={hasEmbeddedLorebook}
                  className={styles.installBtn}
                />
                <button className={styles.secondaryBtn} onClick={handleDownloadPng}>
                  <Image size={14} /> Download PNG
                </button>
                {hasCharxAssets && (
                  <a href={`/api/v1/characters/${card.id}/charx`} download className={styles.secondaryBtn}>
                    <Package size={14} /> .charx
                  </a>
                )}
                {isOwner && (
                  <button className={styles.dangerBtn} onClick={handleDelete} disabled={deleting}>
                    <Trash2 size={14} />
                    {deleting ? 'Deleting...' : 'Delete'}
                  </button>
                )}
              </>
            )}
          </div>

          <CharacterTabs card={card} />
        </div>
      </div>
      {heroLightbox && displayAvatar && (
        <Lightbox
          images={[{ src: displayAvatar, alt: card.name }]}
          onClose={() => setHeroLightbox(false)}
        />
      )}
    </main>
  );
};

export default CharacterDetail;
