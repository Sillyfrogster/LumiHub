import type { UnifiedCharacterCard } from '../../types/character';
import { Star, Heart, Download, Eye } from 'lucide-react';
import { useState, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import { toggleFavorite } from '../../api/favorites';
import LazyImage from '../shared/LazyImage';
import styles from './CharacterCard.module.css';

interface Props {
  card: UnifiedCharacterCard;
  blurNsfw?: boolean;
  onClick?: () => void;
}

/** Renders a single character tile in the browse grid. */
const CharacterCard: React.FC<Props> = ({ card, blurNsfw = true, onClick }) => {
  const { isAuthenticated } = useAuth();
  const [revealed, setRevealed] = useState(false);
  const shouldBlur = card.nsfw && blurNsfw && !revealed;

  const [favorited, setFavorited] = useState(false);
  const [favCount, setFavCount] = useState(card.favorites ?? 0);
  const [favPending, setFavPending] = useState(false);

  const handleReveal = (e: React.MouseEvent) => {
    e.stopPropagation();
    setRevealed((r) => !r);
  };

  const handleFav = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!isAuthenticated || card.source !== 'lumihub') return;
    if (favPending) return;
    setFavPending(true);
    // Optimistic update
    const next = !favorited;
    setFavorited(next);
    setFavCount((c) => c + (next ? 1 : -1));
    try {
      const result = await toggleFavorite('character', card.id);
      setFavorited(result.favorited);
      setFavCount(result.favorites);
    } catch {
      // Revert
      setFavorited(!next);
      setFavCount((c) => c + (next ? -1 : 1));
    } finally {
      setFavPending(false);
    }
  }, [isAuthenticated, card.source, card.id, favorited, favPending]);

  const fmt = (n: number) => n > 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);

  return (
    <div className={styles.card} onClick={onClick}>
      <div className={styles.imageArea}>
        {card.previewUrl || card.avatarUrl ? (
          <LazyImage
            src={card.previewUrl || card.avatarUrl}
            alt={card.name}
            className={`${styles.image} ${shouldBlur ? styles.imageBlurred : ''}`}
            fallback={<div className={styles.placeholder}>{card.name.charAt(0)}</div>}
          />
        ) : (
          <div className={styles.placeholder}>{card.name.charAt(0)}</div>
        )}

        {/* NSFW reveal overlay */}
        {shouldBlur && (
          <div className={styles.revealOverlay} onClick={handleReveal}>
            <Eye size={16} />
            <span>Reveal</span>
          </div>
        )}

        {/* Source badge */}
        <div className={`${styles.sourceBadge} ${card.source === 'chub' ? styles.sourceBadgeChub : ''}`}>
          {card.source === 'lumihub' ? 'LumiHub' : 'Chub'}
        </div>

        {/* Rating badge */}
        {card.rating !== null && card.rating > 4.5 && (
          <div className={styles.ratingBadge}>
            <Star size={10} />
            {card.rating.toFixed(1)}
          </div>
        )}

        {/* Favorite button — LumiHub cards only */}
        {card.source === 'lumihub' && (
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

        {/* Gradient with overlaid text */}
        <div className={styles.scrim}>
          <h3 className={styles.name}>{card.name}</h3>
          <div className={styles.meta}>
            <span className={styles.creator}>
              by {card.creatorDiscordId ? (
                <Link
                  to={`/user/${card.creatorDiscordId}`}
                  className={styles.creatorLink}
                  onClick={(e) => e.stopPropagation()}
                >
                  {card.creator}
                </Link>
              ) : (
                card.creator
              )}
            </span>
            {card.source === 'chub' ? (
              <>
                {card.downloads > 0 && (
                  <span className={styles.downloads}><Download size={11} />{fmt(card.downloads)}</span>
                )}
                {(card.favorites ?? 0) > 0 && (
                  <span className={styles.downloads}><Heart size={11} />{fmt(card.favorites!)}</span>
                )}
              </>
            ) : (
              <span className={styles.downloads}><Download size={11} />{fmt(card.downloads)}</span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default CharacterCard;
