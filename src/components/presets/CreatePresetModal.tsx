import { useState, useRef, useEffect } from 'react';
import { X, Upload, Sparkles, Check, FileCheck } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { createPreset } from '../../api/presets';
import styles from '../characters/CreateCharacterModal.module.css';

interface Props {
  onClose: () => void;
}

const CreatePresetModal: React.FC<Props> = ({ onClose }) => {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    document.body.style.overflow = 'hidden';
    return () => { document.body.style.overflow = ''; };
  }, []);

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [tags, setTags] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [settingCount, setSettingCount] = useState<number | null>(null);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0];
    if (!f) return;
    setError(null);

    try {
      const text = await f.text();
      const json = JSON.parse(text);
      if (typeof json !== 'object' || Array.isArray(json)) {
        setError('Preset file must be a JSON object.');
        setFile(null);
        setSettingCount(null);
        return;
      }
      setFile(f);
      setSettingCount(Object.keys(json).length);
      // Auto-fill name from filename (strip .json)
      if (!name) {
        setName(f.name.replace(/\.json$/i, '').replace(/[-_]/g, ' '));
      }
    } catch {
      setError('Could not parse JSON file.');
      setFile(null);
      setSettingCount(null);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file) { setError('Please select a preset JSON file.'); return; }
    if (!name.trim()) { setError('Preset name is required.'); return; }

    setSubmitting(true);
    setError(null);

    try {
      await createPreset(file, { name: name.trim(), description, tags });
      setSuccess(true);
      queryClient.invalidateQueries({ queryKey: ['presets'] });
      queryClient.invalidateQueries({ queryKey: ['presets-inf'] });
      setTimeout(onClose, 1200);
    } catch (err: any) {
      setError(err.message || 'Upload failed.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <button className={styles.closeBtn} onClick={onClose}><X size={20} /></button>

        <h2 className={styles.title}>Share Preset</h2>
        <p className={styles.subtitle}>
          Upload a generation preset JSON file to share with the community.
          Supports SillyTavern, KoboldAI, and custom preset formats.
        </p>

        {success ? (
          <div className={styles.successState}>
            <Check size={48} />
            <span>Preset shared!</span>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className={styles.form}>
            <div className={styles.imageUpload} onClick={() => fileInputRef.current?.click()}>
              <div className={styles.imagePlaceholder}>
                {file ? (
                  <>
                    <Sparkles size={32} />
                    <span>{file.name}</span>
                  </>
                ) : (
                  <>
                    <Upload size={32} />
                    <span>Drop a preset JSON file here, or click to browse</span>
                  </>
                )}
              </div>
              <input
                ref={fileInputRef}
                type="file"
                accept=".json,application/json"
                onChange={handleFileChange}
                style={{ display: 'none' }}
              />
            </div>

            {settingCount !== null && (
              <div className={styles.autofillBanner}>
                <FileCheck size={16} />
                Detected <strong>{settingCount}</strong> setting{settingCount !== 1 ? 's' : ''}
              </div>
            )}

            <div className={styles.field}>
              <label>Name *</label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="My Preset"
                required
              />
            </div>

            <div className={styles.field}>
              <label>Description</label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="A brief description of this preset and what it's good for"
                rows={3}
              />
            </div>

            <div className={styles.field}>
              <label>Tags</label>
              <input
                type="text"
                value={tags}
                onChange={(e) => setTags(e.target.value)}
                placeholder="creative, roleplay, koboldai (comma separated)"
              />
            </div>

            {error && (
              <div className={styles.errorBox}>{error}</div>
            )}

            <button
              type="submit"
              className={styles.submitBtn}
              disabled={submitting || !file}
            >
              {submitting ? 'Sharing...' : 'Share Preset'}
            </button>
          </form>
        )}
      </div>
    </div>
  );
};

export default CreatePresetModal;
