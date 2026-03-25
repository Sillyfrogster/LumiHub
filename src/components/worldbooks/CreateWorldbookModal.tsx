import { useState, useRef, useCallback, useEffect } from 'react';
import { X, Upload, BookOpen, Check, FileCheck } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { createWorldbook } from '../../api/worldbooks';
import styles from '../characters/CreateCharacterModal.module.css';

interface Props {
  onClose: () => void;
}

const CreateWorldbookModal: React.FC<Props> = ({ onClose }) => {
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
  const [entryCount, setEntryCount] = useState<number | null>(null);
  const [formatLabel, setFormatLabel] = useState<string | null>(null);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const detectFormat = useCallback((json: any): { format: string; count: number; name?: string; description?: string } => {
    if (json.type === 'world_book' && json.entries) {
      const entries = Array.isArray(json.entries) ? json.entries : Object.values(json.entries);
      return { format: 'Lumiverse', count: entries.length, name: json.name, description: json.description };
    }
    if (json.entries) {
      const entries = Array.isArray(json.entries) ? json.entries : Object.values(json.entries);
      return { format: 'Character Book', count: entries.length, name: json.name };
    }
    if (Array.isArray(json)) {
      return { format: 'Entry Array', count: json.length };
    }
    return { format: 'Unknown', count: 0 };
  }, []);

  const handleFileChange = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0];
    if (!f) return;
    setError(null);

    try {
      const text = await f.text();
      const json = JSON.parse(text);
      const detected = detectFormat(json);

      setFile(f);
      setEntryCount(detected.count);
      setFormatLabel(detected.format);
      if (detected.name && !name) setName(detected.name);
      if (detected.description && !description) setDescription(detected.description);

      if (detected.count === 0) {
        setError('No lorebook entries found in this file.');
      }
    } catch {
      setError('Could not parse JSON file.');
      setFile(null);
      setEntryCount(null);
      setFormatLabel(null);
    }
  }, [name, description, detectFormat]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file) { setError('Please select a worldbook file.'); return; }
    if (!name.trim()) { setError('Worldbook name is required.'); return; }

    setSubmitting(true);
    setError(null);

    try {
      await createWorldbook(file, { name: name.trim(), description, tags });
      setSuccess(true);
      queryClient.invalidateQueries({ queryKey: ['worldbooks'] });
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

        <h2 className={styles.title}>Upload Worldbook</h2>
        <p className={styles.subtitle}>
          Upload a lorebook JSON file to share. Supports Lumiverse, SillyTavern, and CCSv3 character book formats.
        </p>

        {success ? (
          <div className={styles.successState}>
            <Check size={48} />
            <span>Worldbook uploaded!</span>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className={styles.form}>
            {/* File upload zone */}
            <div className={styles.imageUpload} onClick={() => fileInputRef.current?.click()}>
              <div className={styles.imagePlaceholder}>
                {file ? (
                  <>
                    <BookOpen size={32} />
                    <span>{file.name}</span>
                  </>
                ) : (
                  <>
                    <Upload size={32} />
                    <span>Drop a worldbook JSON file here, or click to browse</span>
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

            {entryCount !== null && entryCount > 0 && (
              <div className={styles.autofillBanner}>
                <FileCheck size={16} />
                Detected <strong>{formatLabel}</strong> format with <strong>{entryCount}</strong> entries
              </div>
            )}

            <div className={styles.field}>
              <label>Name *</label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="My Worldbook"
                required
              />
            </div>

            <div className={styles.field}>
              <label>Description</label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="A brief description of this worldbook"
                rows={3}
              />
            </div>

            <div className={styles.field}>
              <label>Tags</label>
              <input
                type="text"
                value={tags}
                onChange={(e) => setTags(e.target.value)}
                placeholder="fantasy, rpg, lore (comma separated)"
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
              {submitting ? 'Uploading...' : 'Upload Worldbook'}
            </button>
          </form>
        )}
      </div>
    </div>
  );
};

export default CreateWorldbookModal;
