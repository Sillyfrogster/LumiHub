import { useEffect, useState } from 'react';
import { X, Download, Star, Heart, Users, ArrowLeft, ExternalLink, Image, FileText, Package, Trash2, Code2, ThumbsUp } from 'lucide-react';
import { useCharacterImages } from '../../hooks/useCharacterImages';
import { useAuth } from '../../hooks/useAuth';
import { useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import type { UnifiedCharacterCard } from '../../types/character';
import type { ChubCharacterCard } from '../../types/chub';
import type { LumiHubCharacter } from '../../types/character';
import { downloadCharacter, deleteCharacter } from '../../api/characters';
import CharacterTabs from './CharacterTabs';
import InstallButton from './InstallButton';
import LazyImage from '../shared/LazyImage';
import ScrollFadeRow from '../shared/ScrollFadeRow';
import styles from './CharacterModal.module.css';

interface Props {
  card: UnifiedCharacterCard;
  onClose: () => void;
}

function normalizeImagePath(path: string | null): string | null {
  if (!path) return null;
  let normalized = path.replace(/\\/g, '/');
  if (!normalized.startsWith('uploads/')) normalized = `uploads/${normalized}`;
  return `/${normalized}`;
}

const CharacterModal: React.FC<Props> = ({ card, onClose }) => {
  const isChub = card.source === 'chub';
  const chubData = isChub ? (card.raw as ChubCharacterCard) : null;
  const lumiData = !isChub ? (card.raw as LumiHubCharacter) : null;
  const [heroUrl, setHeroUrl] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const { user } = useAuth();
  const queryClient = useQueryClient();
  const { data: images } = useCharacterImages(lumiData?.id);
  const altAvatars = images?.filter((img) => img.image_type === 'avatar_alt') ?? [];
  const hasCharxAssets = (images?.length ?? 0) > 1;

  const isOwner = !isChub && user && lumiData?.owner?.id === user.id;
  const displayAvatar = heroUrl || card.avatarUrl;

  const lumiCharBook = lumiData?.character_book as { entries?: unknown[] } | null | undefined;
  const hasEmbeddedLorebook = isChub || (lumiCharBook?.entries?.length ?? 0) > 0;
  const lumiverseModules = lumiData?.extensions?.lumiverse_modules as { regex_scripts?: unknown[] } | undefined;
  const regexScriptCount = lumiverseModules?.regex_scripts?.length ?? 0;

  useEffect(() => {
    document.body.style.overflow = 'hidden';
    const handleEsc = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', handleEsc);
    return () => {
      document.body.style.overflow = '';
      window.removeEventListener('keydown', handleEsc);
    };
  }, [onClose]);

  const handleDownloadJson = async () => {
    if (!isChub && lumiData) {
      try { await downloadCharacter(lumiData.id); } catch (err) { console.error('Download failed:', err); }
    }
  };

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
      onClose();
    } catch (err) {
      console.error('Delete failed:', err);
      setDeleting(false);
    }
  };

  const formattedDownloads = card.downloads > 1000
    ? `${(card.downloads / 1000).toFixed(1)}k`
    : String(card.downloads);

  return (
    <div className={styles.overlay} onClick={onClose} role="dialog" aria-modal="true" aria-label={`${card.name} character details`}>
      <div className={styles.panel} onClick={(e) => e.stopPropagation()}>
        <div className={styles.panelHeader}>
          <button className={styles.backBtn} onClick={onClose} aria-label="Close character details">
            <ArrowLeft size={16} />
            <span>Back</span>
          </button>
          <button className={styles.closeBtn} onClick={onClose} aria-label="Close">
            <X size={18} />
          </button>
        </div>

        <div className={styles.panelScroll}>
          <div className={styles.hero}>
            <div className={styles.heroImage}>
              {displayAvatar ? (
                <LazyImage
                  src={displayAvatar}
                  alt={card.name}
                  className={styles.heroImg}
                  fallback={<div className={styles.heroPlaceholder}>{card.name.charAt(0)}</div>}
                />
              ) : (
                <div className={styles.heroPlaceholder}>{card.name.charAt(0)}</div>
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
            <div className={styles.heroInfo}>
              <h1 className={styles.heroName}>{card.name}</h1>
              <p className={styles.heroCreator}>
                by {card.creatorDiscordId ? (
                  <Link to={`/user/${card.creatorDiscordId}`} className={styles.heroCreatorLink}>
                    {card.creator}
                  </Link>
                ) : (
                  card.creator
                )}
                {lumiData?.character_version && <span className={styles.heroVersion}> · v{lumiData.character_version}</span>}
              </p>
              <ScrollFadeRow className={styles.heroStats}>
                {isChub ? (
                  <>
                    {card.downloads > 0 && <span className={styles.statChip}><Download size={13} />{card.downloads.toLocaleString()} Downloads</span>}
                    {(card.favorites ?? 0) > 0 && <span className={styles.statChip}><Heart size={13} />{card.favorites!.toLocaleString()} Favorites</span>}
                    {(card.stars ?? 0) > 0 && <span className={styles.statChip}><Star size={13} />{card.stars!.toLocaleString()} Stars</span>}
                    {card.rating !== null && <span className={styles.statChip}><ThumbsUp size={13} />{card.rating.toFixed(1)}/5</span>}
                    {chubData?.chats !== undefined && <span className={styles.statChip}><Users size={13} />{chubData.chats.toLocaleString()} Chats</span>}
                    {chubData?.tokenCount !== undefined && <span className={styles.statChip}><FileText size={13} />{chubData.tokenCount.toLocaleString()} Tokens</span>}
                  </>
                ) : (
                  <>
                    <span className={styles.statChip}><Download size={13} />{formattedDownloads} Downloads</span>
                    {regexScriptCount > 0 && <span className={styles.statChip}><Code2 size={13} />{regexScriptCount} Regex</span>}
                  </>
                )}
              </ScrollFadeRow>

              <div className={styles.heroActions}>
                {isChub ? (
                  <>
                    <InstallButton
                      characterId={card.id}
                      source="chub"
                      hasEmbeddedLorebook={hasEmbeddedLorebook}
                      className={styles.installBtn}
                    />
                    <a href={chubData?.pageUrl} target="_blank" rel="noreferrer" className={styles.secondaryBtn}>
                      <ExternalLink size={14} />
                      View on Chub
                    </a>
                  </>
                ) : (
                  <>
                    <InstallButton
                      characterId={card.id}
                      source="lumihub"
                      hasEmbeddedLorebook={hasEmbeddedLorebook}
                      className={styles.installBtn}
                    />
                    <button className={styles.secondaryBtn} onClick={handleDownloadJson}>
                      <Download size={14} />
                      JSON
                    </button>
                    {card.avatarUrl && (
                      <button className={styles.secondaryBtn} onClick={handleDownloadPng}>
                        <Image size={14} />
                        PNG
                      </button>
                    )}
                    {hasCharxAssets && (
                      <a href={`/api/v1/characters/${card.id}/charx`} download className={styles.secondaryBtn}>
                        <Package size={14} />
                        .charx
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
            </div>
          </div>

          {card.tags.length > 0 && (
            <ScrollFadeRow className={styles.tagsRow}>
              {card.tags.map((tag, i) => (
                <span key={i} className={styles.tag}>{tag}</span>
              ))}
            </ScrollFadeRow>
          )}

          <CharacterTabs
            card={card}
            tabBarClassName={styles.tabBarPadded}
            tabContentClassName={styles.tabContentPadded}
          />
        </div>
      </div>
    </div>
  );
};

export default CharacterModal;
