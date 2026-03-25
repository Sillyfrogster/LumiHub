import { useEffect } from 'react';
import { useState } from 'react';
import { X, Download, Star, Users, ArrowLeft, ExternalLink, Image, FileText, Package } from 'lucide-react';
import { useCharacterImages } from '../../hooks/useCharacterImages';
import { Link } from 'react-router-dom';
import type { UnifiedCharacterCard } from '../../types/character';
import type { ChubCharacterCard } from '../../types/chub';
import type { LumiHubCharacter } from '../../types/character';
import { downloadCharacter } from '../../api/characters';
import CharacterTabs from './CharacterTabs';
import InstallButton from './InstallButton';
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

  const { data: images } = useCharacterImages(lumiData?.id);
  const altAvatars = images?.filter((img) => img.image_type === 'avatar_alt') ?? [];
  const hasCharxAssets = (images?.length ?? 0) > 1;

  const displayAvatar = heroUrl || card.avatarUrl;

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

  const formattedDownloads = card.downloads > 1000
    ? `${(card.downloads / 1000).toFixed(1)}k`
    : String(card.downloads);

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.panel} onClick={(e) => e.stopPropagation()}>
        <div className={styles.panelHeader}>
          <button className={styles.backBtn} onClick={onClose}>
            <ArrowLeft size={16} />
            <span>Back</span>
          </button>
          <button className={styles.closeBtn} onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className={styles.panelScroll}>
          <div className={styles.hero}>
            <div className={styles.heroImage}>
              {displayAvatar ? (
                <img src={displayAvatar} alt={card.name} className={styles.heroImg} />
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
              <div className={styles.heroStats}>
                <span className={styles.statChip}><Download size={13} />{formattedDownloads} Downloads</span>
                {card.rating !== null && <span className={styles.statChip}><Star size={13} />{card.rating.toFixed(1)}</span>}
                {isChub && chubData?.interactions !== undefined && <span className={styles.statChip}><Users size={13} />{chubData.interactions.toLocaleString()} Chats</span>}
                {chubData?.tokenCount !== undefined && <span className={styles.statChip}><FileText size={13} />{chubData.tokenCount.toLocaleString()} Tokens</span>}
              </div>

              <div className={styles.heroActions}>
                {isChub ? (
                  <>
                    <InstallButton
                      characterId={card.id}
                      source="chub"
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
                  </>
                )}
              </div>
            </div>
          </div>

          {card.tags.length > 0 && (
            <div className={styles.tagsRow}>
              {card.tags.map((tag, i) => (
                <span key={i} className={styles.tag}>{tag}</span>
              ))}
            </div>
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
