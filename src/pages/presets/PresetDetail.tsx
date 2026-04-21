import { useEffect, useState } from 'react';
import { useParams, useLocation, useNavigate, Link } from 'react-router-dom';
import { ArrowLeft, Download, Heart, Trash2, Sparkles } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { useAuth } from '../../hooks/useAuth';
import { getPreset, deletePreset, viewPreset } from '../../api/presets';
import { fromLumiHub } from '../../types/preset';
import type { UnifiedPreset, LumiPreset } from '../../types/preset';
import ScrollFadeRow from '../../components/shared/ScrollFadeRow';
import styles from './PresetDetail.module.css';

const PresetDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const navigate = useNavigate();

  const { user } = useAuth();
  const queryClient = useQueryClient();

  const passedPreset = (location.state as { preset?: UnifiedPreset })?.preset ?? null;
  const [preset, setPreset] = useState<UnifiedPreset | null>(passedPreset);
  const [loading, setLoading] = useState(!passedPreset);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const lumiData = preset ? (preset.raw as LumiPreset) : null;

  useEffect(() => {
    if (passedPreset || !id) return;
    setLoading(true);
    getPreset(id)
      .then((res) => setPreset(fromLumiHub(res.data)))
      .catch(() => setError('Preset not found.'))
      .finally(() => setLoading(false));
  }, [id, passedPreset]);

  // Fire view increment once per visit
  useEffect(() => {
    if (lumiData?.id) viewPreset(lumiData.id);
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

  if (error || !preset) {
    return (
      <div className={styles.page}>
        <div className={styles.errorState}>
          <p>{error || 'Preset not found.'}</p>
          <button className={styles.backBtnLarge} onClick={() => navigate('/presets')}>
            <ArrowLeft size={16} /> Back to Presets
          </button>
        </div>
      </div>
    );
  }

  const isOwner = user && lumiData && lumiData.owner?.id === user.id;

  const handleDelete = async () => {
    if (!lumiData || !confirm(`Delete "${preset.name}"? This cannot be undone.`)) return;
    setDeleting(true);
    try {
      await deletePreset(lumiData.id);
      queryClient.invalidateQueries({ queryKey: ['presets'] });
      queryClient.invalidateQueries({ queryKey: ['presets-inf'] });
      navigate('/presets');
    } catch (err) {
      console.error('Delete failed:', err);
      setDeleting(false);
    }
  };

  const handleDownload = () => {
    if (!id) return;
    const link = document.createElement('a');
    link.href = `/api/v1/presets/${id}/export`;
    link.download = `${preset.name.replace(/[^a-zA-Z0-9_\-. ]/g, '_')}.json`;
    link.click();
    // Increment counter
    fetch(`/api/v1/presets/${id}/download`, { method: 'POST' }).catch(() => {});
  };

  // Build a sorted display of settings entries
  const settingsEntries = Object.entries(preset.settings).sort(([a], [b]) => a.localeCompare(b));

  return (
    <main className={styles.page} aria-label={`${preset.name} preset details`}>
      <button className={styles.backBtn} onClick={() => navigate(-1)} aria-label="Go back">
        <ArrowLeft size={16} aria-hidden="true" />
        <span>Back</span>
      </button>

      <div className={styles.layout}>
        <div className={styles.sidebar}>
          <div className={styles.iconBox}>
            <Sparkles size={36} />
          </div>

          <div className={styles.sidebarStats}>
            {preset.downloads > 0 && (
              <div className={styles.sidebarStat}>
                <Download size={14} />
                <span>{preset.downloads.toLocaleString()} downloads</span>
              </div>
            )}
            {preset.favorites > 0 && (
              <div className={styles.sidebarStat}>
                <Heart size={14} />
                <span>{preset.favorites.toLocaleString()} favorites</span>
              </div>
            )}
          </div>

          <button className={styles.downloadBtn} onClick={handleDownload}>
            <Download size={14} />
            Download Preset
          </button>

          {isOwner && (
            <button className={styles.dangerBtn} onClick={handleDelete} disabled={deleting}>
              <Trash2 size={14} />
              {deleting ? 'Deleting...' : 'Delete'}
            </button>
          )}
        </div>

        <div className={styles.content}>
          <h1 className={styles.name}>{preset.name}</h1>
          <p className={styles.creator}>
            by{' '}
            {preset.creatorDiscordId ? (
              <Link to={`/user/${preset.creatorDiscordId}`} className={styles.creatorLink}>
                {preset.creator}
              </Link>
            ) : (
              preset.creator
            )}
          </p>

          {preset.tags.length > 0 && (
            <ScrollFadeRow className={styles.tagsRow}>
              {preset.tags.map((tag, i) => (
                <span key={i} className={styles.tag}>{tag}</span>
              ))}
            </ScrollFadeRow>
          )}

          {preset.description && (
            <div className={styles.descriptionBlock}>
              <pre className={styles.descriptionText}>{preset.description}</pre>
            </div>
          )}

          <div className={styles.settingsSection}>
            <h2 className={styles.settingsTitle}>
              <Sparkles size={16} />
              Parameters
              <span className={styles.settingsCount}>{settingsEntries.length}</span>
            </h2>

            {settingsEntries.length > 0 ? (
              <div className={styles.settingsGrid}>
                {settingsEntries.map(([key, value]) => (
                  <div key={key} className={styles.settingRow}>
                    <span className={styles.settingKey}>{key}</span>
                    <span className={styles.settingValue}>
                      {typeof value === 'object'
                        ? JSON.stringify(value)
                        : String(value)}
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <p className={styles.emptyText}>No parameters found in this preset.</p>
            )}
          </div>
        </div>
      </div>
    </main>
  );
};

export default PresetDetail;
