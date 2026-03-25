import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { X, Check, AlertCircle, LogOut } from 'lucide-react';
import { useAuth } from '../../hooks/useAuth';
import styles from './Settings.module.css';

interface UserSettings {
  customDisplayName: string;
  nsfwEnabled: boolean;
  nsfwUnblurred: boolean;
  defaultIncludeTags: string[];
  defaultExcludeTags: string[];
}

const Settings = () => {
  const { user, isAuthenticated, isLoading: authLoading, logout } = useAuth();
  const navigate = useNavigate();

  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [includeInput, setIncludeInput] = useState('');
  const [excludeInput, setExcludeInput] = useState('');

  useEffect(() => {
    if (authLoading) return;
    if (!isAuthenticated) {
      navigate('/', { replace: true });
      return;
    }

    fetch('/api/v1/user/@me/settings', { credentials: 'include' })
      .then((res) => {
        if (!res.ok) throw new Error('Failed to load settings');
        return res.json();
      })
      .then((data) => setSettings(data))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [authLoading, isAuthenticated, navigate]);

  const update = useCallback(
    <K extends keyof UserSettings>(key: K, value: UserSettings[K]) => {
      setSettings((prev) => (prev ? { ...prev, [key]: value } : prev));
      setSaved(false);
    },
    [],
  );

  const addTag = (type: 'include' | 'exclude') => {
    const input = type === 'include' ? includeInput : excludeInput;
    const tag = input.trim().toLowerCase();
    if (!tag || !settings) return;

    const key = type === 'include' ? 'defaultIncludeTags' : 'defaultExcludeTags';
    if (settings[key].includes(tag)) return;

    update(key, [...settings[key], tag]);
    if (type === 'include') setIncludeInput('');
    else setExcludeInput('');
  };

  const removeTag = (type: 'include' | 'exclude', tag: string) => {
    if (!settings) return;
    const key = type === 'include' ? 'defaultIncludeTags' : 'defaultExcludeTags';
    update(key, settings[key].filter((t) => t !== tag));
  };

  const handleSave = async () => {
    if (!settings) return;
    setSaving(true);
    setSaved(false);
    setError(null);

    try {
      const res = await fetch('/api/v1/user/@me/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(settings),
      });
      if (!res.ok) throw new Error('Failed to save settings');
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  if (authLoading || loading) {
    return <div className={styles.loadingState}>Loading settings...</div>;
  }

  if (error && !settings) {
    return (
      <div className={styles.errorState}>
        <AlertCircle size={40} opacity={0.5} />
        <p>{error}</p>
      </div>
    );
  }

  if (!settings) return null;

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>Settings</h1>
        <p className={styles.subtitle}>Manage your LumiHub preferences</p>
      </div>

      {/* Profile */}
      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>Profile</h2>
        <div className={styles.field}>
          <label className={styles.fieldLabel}>Display Name</label>
          <input
            type="text"
            className={styles.textInput}
            value={settings.customDisplayName}
            onChange={(e) => update('customDisplayName', e.target.value)}
            placeholder={user?.displayName || 'Display name'}
            maxLength={64}
          />
          <p className={styles.handleDisplay}>
            Your handle <span className={styles.handleAt}>@{user?.username}</span> cannot be changed
          </p>
        </div>
      </div>

      {/* Content Preferences */}
      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>Content</h2>
        <div className={styles.toggleRow}>
          <div className={styles.toggleInfo}>
            <p className={styles.toggleLabel}>Enable NSFW/NSFL content</p>
            <p className={styles.toggleDesc}>Always show mature character cards in search results</p>
          </div>
          <label className={styles.toggle}>
            <input
              type="checkbox"
              className={styles.toggleInput}
              checked={settings.nsfwEnabled}
              onChange={(e) => update('nsfwEnabled', e.target.checked)}
            />
            <span className={styles.toggleTrack} />
          </label>
        </div>
        <div className={styles.toggleRow}>
          <div className={styles.toggleInfo}>
            <p className={styles.toggleLabel}>Never blur NSFW/NSFL cards</p>
            <p className={styles.toggleDesc}>Show card images without the blur overlay</p>
          </div>
          <label className={styles.toggle}>
            <input
              type="checkbox"
              className={styles.toggleInput}
              checked={settings.nsfwUnblurred}
              onChange={(e) => update('nsfwUnblurred', e.target.checked)}
            />
            <span className={styles.toggleTrack} />
          </label>
        </div>
      </div>

      {/* Default Tags */}
      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>Default Search Tags</h2>

        <div className={styles.tagField}>
          <label className={styles.fieldLabel}>Include Tags</label>
          <p className={styles.fieldHint}>Pre-fill gallery searches with these tags</p>
          <div className={styles.tagChips}>
            {settings.defaultIncludeTags.map((tag) => (
              <span key={tag} className={`${styles.tagChip} ${styles.tagChipInclude}`}>
                {tag}
                <button className={styles.tagChipRemove} onClick={() => removeTag('include', tag)}>
                  <X size={10} />
                </button>
              </span>
            ))}
          </div>
          <div className={styles.tagInputRow}>
            <input
              type="text"
              className={styles.tagTextInput}
              value={includeInput}
              onChange={(e) => setIncludeInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), addTag('include'))}
              placeholder="Type a tag and press Enter"
            />
            <button className={styles.tagAddBtn} onClick={() => addTag('include')}>Add</button>
          </div>
        </div>

        <div className={styles.tagField}>
          <label className={styles.fieldLabel}>Exclude Tags</label>
          <p className={styles.fieldHint}>Always filter out cards with these tags</p>
          <div className={styles.tagChips}>
            {settings.defaultExcludeTags.map((tag) => (
              <span key={tag} className={`${styles.tagChip} ${styles.tagChipExclude}`}>
                {tag}
                <button className={styles.tagChipRemove} onClick={() => removeTag('exclude', tag)}>
                  <X size={10} />
                </button>
              </span>
            ))}
          </div>
          <div className={styles.tagInputRow}>
            <input
              type="text"
              className={styles.tagTextInput}
              value={excludeInput}
              onChange={(e) => setExcludeInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), addTag('exclude'))}
              placeholder="Type a tag and press Enter"
            />
            <button className={styles.tagAddBtn} onClick={() => addTag('exclude')}>Add</button>
          </div>
        </div>
      </div>

      {/* Account */}
      <div className={styles.section}>
        <h2 className={styles.sectionTitle}>Account</h2>
        <button className={styles.logoutBtn} onClick={logout}>
          <LogOut size={16} />
          Log Out
        </button>
      </div>

      {/* Save */}
      <div className={styles.saveBar}>
        {error && settings && <span style={{ color: 'var(--lumihub-danger)', fontSize: 13 }}>{error}</span>}
        <span className={`${styles.saveStatus} ${saved ? styles.saveStatusVisible : ''}`}>
          <Check size={14} />
          Saved
        </span>
        <button className={styles.saveBtn} onClick={handleSave} disabled={saving}>
          {saving ? 'Saving...' : 'Save Changes'}
        </button>
      </div>
    </div>
  );
};

export default Settings;
