import { Star, Download, BookOpen, Eye, Heart } from 'lucide-react';
import { useState, useCallback } from 'react';
import LazyImage from '../shared/LazyImage';
import type { UnifiedWorldBook } from '../../types/worldbook';
import { useAuth } from '../../hooks/useAuth';
import { toggleFavorite } from '../../api/favorites';
import styles from '../characters/CharacterCard.module.css';

interface Props {
  worldbook: UnifiedWorldBook;
  blurNsfw?: boolean;
  onClick?: () => void;
}

const WorldbookCard: React.FC<Props> = ({ worldbook, blurNsfw = true, onClick }) => {
  const { isAuthenticated } = useAuth();
  const [revealed, setRevealed] = useState(false);
  const shouldBlur = worldbook.nsfw && blurNsfw && !revealed;

  const [favorited, setFavorited] = useState(false);
  const [favCount, setFavCount] = useState(worldbook.favorites ?? 0);
  const [favPending, setFavPending] = useState(false);

  const handleReveal = (e: React.MouseEvent) => {
    e.stopPropagation();
    setRevealed((r) => !r);
  };

  const handleFav = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!isAuthenticated || worldbook.source !== 'lumihub') return;
    if (favPending) return;
    setFavPending(true);
    const next = !favorited;
    setFavorited(next);
    setFavCount((c) => c + (next ? 1 : -1));
    try {
      const result = await toggleFavorite('worldbook', worldbook.id);
      setFavorited(result.favorited);
      setFavCount(result.favorites);
    } catch {
      setFavorited(!next);
      setFavCount((c) => c + (next ? -1 : 1));
    } finally {
      setFavPending(false);
    }
  }, [isAuthenticated, worldbook.source, worldbook.id, favorited, favPending]);

  const fmt = (n: number) => n > 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);

  const formattedStars =
    worldbook.downloads > 1000
      ? `${(worldbook.downloads / 1000).toFixed(1)}k`
      : String(worldbook.downloads);

  const formattedTokens =
    worldbook.tokenCount > 1000
      ? `${(worldbook.tokenCount / 1000).toFixed(1)}k`
      : String(worldbook.tokenCount);

  return (
    <div className={styles.card} onClick={onClick}>
      <div className={styles.imageArea}>
        {worldbook.previewUrl || worldbook.avatarUrl ? (
          <LazyImage
            src={worldbook.previewUrl || worldbook.avatarUrl}
            alt={worldbook.name}
            className={`${styles.image} ${shouldBlur ? styles.imageBlurred : ''}`}
            fallback={<div className={styles.placeholder}>{worldbook.name.charAt(0)}</div>}
          />
        ) : (
          <div className={styles.placeholder}>{worldbook.name.charAt(0)}</div>
        )}

        {shouldBlur && (
          <div className={styles.revealOverlay} onClick={handleReveal}>
            <Eye size={16} />
            <span>Reveal</span>
          </div>
        )}

        <div className={`${styles.sourceBadge} ${worldbook.source === 'chub' ? styles.sourceBadgeChub : ''}`}>
          {worldbook.source === 'lumihub' ? 'LumiHub' : 'Chub'}
        </div>

        {worldbook.rating !== null && worldbook.rating > 4.5 && (
          <div className={styles.ratingBadge}>
            <Star size={10} />
            {worldbook.rating.toFixed(1)}
          </div>
        )}

        {/* Favorite button — LumiHub worldbooks only */}
        {worldbook.source === 'lumihub' && (
          <button
            className={`${styles.favBtn} ${favorited ? styles.favBtnActive : ''}`}
            onClick={handleFav}
            title={isAuthenticated ? (favorited ? 'Remove favorite' : 'Add to favorites') : 'Log in to favorite'}
            aria-label={favorited ? 'Remove favorite' : 'Add to favorites'}
            aria-pressed={favorited}
            disabled={favPending}
          >
            <Heart size={12} fill={favorited ? 'currentColor' : 'none'} />
            {favCount > 0 && <span className={styles.favCount}>{fmt(favCount)}</span>}
          </button>
        )}

        <div className={styles.scrim}>
          <h3 className={styles.name}>{worldbook.name}</h3>
          <div className={styles.meta}>
            <span className={styles.creator}>
              <BookOpen size={10} style={{ marginRight: 3 }} />
              {formattedTokens} tokens
            </span>
            <span className={styles.downloads}>
              {worldbook.source === 'chub' ? <Star size={11} /> : <Download size={11} />}
              {formattedStars}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
};

export default WorldbookCard;
