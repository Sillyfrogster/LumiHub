import { Star, Download, BookOpen, Eye } from 'lucide-react';
import { useState } from 'react';
import LazyImage from '../shared/LazyImage';
import type { UnifiedWorldBook } from '../../types/worldbook';
import styles from '../characters/CharacterCard.module.css';

interface Props {
  worldbook: UnifiedWorldBook;
  blurNsfw?: boolean;
  onClick?: () => void;
}

const WorldbookCard: React.FC<Props> = ({ worldbook, blurNsfw = true, onClick }) => {
  const [revealed, setRevealed] = useState(false);
  const shouldBlur = worldbook.nsfw && blurNsfw && !revealed;

  const handleReveal = (e: React.MouseEvent) => {
    e.stopPropagation();
    setRevealed((r) => !r);
  };

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
        {worldbook.avatarUrl ? (
          <LazyImage
            src={worldbook.avatarUrl}
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
