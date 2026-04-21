import { useState, useCallback } from 'react';
import { Download, Heart, Sparkles } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import { toggleFavorite } from '../../api/favorites';
import type { UnifiedPreset } from '../../types/preset';
import styles from './PresetCard.module.css';

interface Props {
  preset: UnifiedPreset;
  onClick?: () => void;
}

/** Known numeric generation parameters and their typical max values for bar scaling. */
const PARAM_CONFIG: Array<{ key: string; label: string; max: number }> = [
  { key: 'temperature', label: 'Temperature', max: 2 },
  { key: 'top_p', label: 'Top-P', max: 1 },
  { key: 'top_k', label: 'Top-K', max: 150 },
  { key: 'repetition_penalty', label: 'Rep. Penalty', max: 1.5 },
  { key: 'rep_pen', label: 'Rep. Penalty', max: 1.5 },
  { key: 'min_p', label: 'Min-P', max: 1 },
  { key: 'typical_p', label: 'Typical-P', max: 1 },
  { key: 'tfs', label: 'TFS', max: 1 },
];

function extractParams(settings: Record<string, any>) {
  const results: Array<{ label: string; value: string; fill: number }> = [];
  for (const config of PARAM_CONFIG) {
    const val = settings[config.key];
    if (typeof val === 'number') {
      results.push({
        label: config.label,
        value: String(val),
        fill: Math.min(100, Math.round((val / config.max) * 100)),
      });
    }
    if (results.length >= 4) break;
  }
  return results;
}

const PresetCard: React.FC<Props> = ({ preset, onClick }) => {
  const { isAuthenticated } = useAuth();
  const [favorited, setFavorited] = useState(false);
  const [favCount, setFavCount] = useState(preset.favorites ?? 0);
  const [favPending, setFavPending] = useState(false);

  const params = extractParams(preset.settings);
  const settingCount = Object.keys(preset.settings).length;
  const fmt = (n: number) => n > 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);

  const handleFav = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!isAuthenticated || favPending) return;
    setFavPending(true);
    const next = !favorited;
    setFavorited(next);
    setFavCount((c) => c + (next ? 1 : -1));
    try {
      const result = await toggleFavorite('preset', preset.id);
      setFavorited(result.favorited);
      setFavCount(result.favorites);
    } catch {
      setFavorited(!next);
      setFavCount((c) => c + (next ? -1 : 1));
    } finally {
      setFavPending(false);
    }
  }, [isAuthenticated, preset.id, favorited, favPending]);

  return (
    <div className={styles.card} onClick={onClick}>
      <div className={styles.header}>
        <div className={styles.iconWrap}>
          <Sparkles size={18} />
        </div>
        <div className={styles.info}>
          <p className={styles.name}>{preset.name}</p>
          <p className={styles.author}>
            by{' '}
            {preset.creatorDiscordId ? (
              <Link
                to={`/user/${preset.creatorDiscordId}`}
                className={styles.authorLink}
                onClick={(e) => e.stopPropagation()}
              >
                {preset.creator}
              </Link>
            ) : (
              preset.creator
            )}
          </p>
        </div>
      </div>

      {params.length > 0 ? (
        <div className={styles.paramList}>
          {params.map((param) => (
            <div key={param.label} className={styles.param}>
              <div className={styles.paramRow}>
                <span className={styles.paramLabel}>{param.label}</span>
                <span className={styles.paramValue}>{param.value}</span>
              </div>
              <div className={styles.paramTrack}>
                <div className={styles.paramFill} style={{ width: `${param.fill}%` }} />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <p className={styles.rawHint}>
          {settingCount} setting{settingCount !== 1 ? 's' : ''}
        </p>
      )}

      <div className={styles.footer}>
        <div className={styles.tags}>
          {preset.tags.slice(0, 3).map((tag) => (
            <span key={tag} className={styles.tag}>{tag}</span>
          ))}
        </div>
        <div className={styles.stats}>
          {preset.downloads > 0 && (
            <span className={styles.stat}><Download size={11} />{fmt(preset.downloads)}</span>
          )}
          <button
            className={`${styles.favBtn} ${favorited ? styles.favBtnActive : ''}`}
            onClick={handleFav}
            title={isAuthenticated ? (favorited ? 'Remove favorite' : 'Add to favorites') : 'Log in to favorite'}
            aria-label={favorited ? 'Remove favorite' : 'Add to favorites'}
            aria-pressed={favorited}
            disabled={favPending}
          >
            <Heart size={11} fill={favorited ? 'currentColor' : 'none'} />
            {favCount > 0 && <span>{fmt(favCount)}</span>}
          </button>
        </div>
      </div>
    </div>
  );
};

export default PresetCard;
