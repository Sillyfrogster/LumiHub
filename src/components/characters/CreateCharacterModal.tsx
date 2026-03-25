import { useState, useRef, useEffect } from 'react';
import { motion } from 'motion/react';
import { useQueryClient } from '@tanstack/react-query';
import { X, Upload, Image as ImageIcon, Check, AlertCircle, FileCheck, Package } from 'lucide-react';
import { createCharacter, createCharacterFromCharx } from '../../api/characters';
import { useCharacterStore } from '../../store/useCharacterStore';
import { parseCharacterPng } from '../../utils/pngParser';
import { parseCharxFile, revokeCharxUrls, type ParsedCharx } from '../../utils/charxParser';
import styles from './CreateCharacterModal.module.css';

interface Props {
  onClose: () => void;
}

/** Modal form for creating a character, with auto-fill from PNG or .charx. */
const CreateCharacterModal: React.FC<Props> = ({ onClose }) => {
  const queryClient = useQueryClient();
  const source = useCharacterStore((s) => s.source);

  useEffect(() => {
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = '';
    };
  }, []);

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [personality, setPersonality] = useState('');
  const [scenario, setScenario] = useState('');
  const [firstMes, setFirstMes] = useState('');
  const [tags, setTags] = useState('');
  const [creator, setCreator] = useState('');
  const [creatorNotes, setCreatorNotes] = useState('');
  const [image, setImage] = useState<File | null>(null);
  const [imagePreview, setImagePreview] = useState<string | null>(null);
  const [autofilled, setAutofilled] = useState(false);

  // Charx state
  const [charxData, setCharxData] = useState<ParsedCharx | null>(null);
  const [isCharx, setIsCharx] = useState(false);

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const fileInputRef = useRef<HTMLInputElement>(null);

  // Cleanup blob URLs on unmount
  useEffect(() => {
    return () => {
      if (charxData) revokeCharxUrls(charxData);
    };
  }, [charxData]);

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setError(null);

    // Detect .charx by extension or ZIP magic bytes
    if (file.name.endsWith('.charx') || file.type === 'application/zip') {
      await handleCharxFile(file);
      return;
    }

    // Also check ZIP magic bytes for files without proper extension
    const header = new Uint8Array(await file.slice(0, 4).arrayBuffer());
    if (header[0] === 0x50 && header[1] === 0x4b && header[2] === 0x03 && header[3] === 0x04) {
      await handleCharxFile(file);
      return;
    }

    // PNG path (existing logic)
    if (file.type !== 'image/png') {
      setError('File must be PNG or .charx format.');
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setError('Image must be under 5 MB.');
      return;
    }

    // Clean up previous charx data
    if (charxData) {
      revokeCharxUrls(charxData);
      setCharxData(null);
      setIsCharx(false);
    }

    setImage(file);
    const reader = new FileReader();
    reader.onload = () => setImagePreview(reader.result as string);
    reader.readAsDataURL(file);

    const parsed = await parseCharacterPng(file);
    if (parsed) {
      autoFillFromCard(parsed);
      setAutofilled(true);
    }
  };

  const handleCharxFile = async (file: File) => {
    if (file.size > 50 * 1024 * 1024) {
      setError('.charx file must be under 50 MB.');
      return;
    }

    const parsed = await parseCharxFile(file);
    if (!parsed) {
      setError('Failed to parse .charx file. Ensure it contains a valid card.json.');
      return;
    }

    // Clean up previous charx data
    if (charxData) revokeCharxUrls(charxData);

    setCharxData(parsed);
    setIsCharx(true);
    setImage(null);

    if (parsed.primaryAvatar) {
      setImagePreview(parsed.primaryAvatar.url);
    }

    autoFillFromCard(parsed.card);
    setAutofilled(true);
  };

  const autoFillFromCard = (card: { name: string; description: string; personality: string; scenario: string; first_mes: string; creator: string; creator_notes: string; tags: string[] }) => {
    setName(card.name || name);
    setDescription(card.description || description);
    setPersonality(card.personality || personality);
    setScenario(card.scenario || scenario);
    setFirstMes(card.first_mes || firstMes);
    setCreator(card.creator || creator);
    setCreatorNotes(card.creator_notes || creatorNotes);
    if (card.tags.length > 0) {
      setTags(card.tags.join(', '));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!name.trim()) {
      setError('Character name is required.');
      return;
    }

    setSubmitting(true);
    setError(null);

    try {
      const characterData = {
        name: name.trim(),
        description,
        personality,
        scenario,
        first_mes: firstMes,
        tags: tags
          .split(',')
          .map((t) => t.trim())
          .filter(Boolean),
        creator,
        creator_notes: creatorNotes,
        // Include raw card data for charx (extensions, character_book, etc.)
        ...(isCharx && charxData?.card._raw ? {
          alternate_greetings: charxData.card.alternate_greetings,
          system_prompt: charxData.card.system_prompt,
          post_history_instructions: charxData.card.post_history_instructions,
          mes_example: charxData.card.mes_example,
          character_version: charxData.card.character_version,
          character_book: charxData.card._raw.character_book,
          extensions: charxData.card._raw.extensions ?? {},
        } : {}),
      };

      if (isCharx && charxData) {
        await createCharacterFromCharx(characterData, charxData.rawFile);
      } else {
        await createCharacter(characterData, image ?? undefined);
      }

      setSuccess(true);

      if (source === 'lumihub') {
        queryClient.invalidateQueries({ queryKey: ['characters'] });
      }

      setTimeout(onClose, 1200);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create character.');
    } finally {
      setSubmitting(false);
    }
  };

  // Build summary badges for charx content
  const charxBadges: string[] = [];
  if (charxData) {
    if (charxData.expressions.length > 0)
      charxBadges.push(`${charxData.expressions.length} expression${charxData.expressions.length !== 1 ? 's' : ''}`);
    if (charxData.alternateAvatars.length > 0)
      charxBadges.push(`${charxData.alternateAvatars.length} alt avatar${charxData.alternateAvatars.length !== 1 ? 's' : ''}`);
    if (charxData.gallery.length > 0)
      charxBadges.push(`${charxData.gallery.length} gallery image${charxData.gallery.length !== 1 ? 's' : ''}`);
    const altFields = charxData.lumiverseModules?.alternate_fields;
    if (altFields) {
      const count = Object.values(altFields).reduce((sum, arr) => sum + arr.length, 0);
      if (count > 0) charxBadges.push(`${count} alt field${count !== 1 ? 's' : ''}`);
    }
    const altGreetings = charxData.card.alternate_greetings;
    if (altGreetings.length > 0)
      charxBadges.push(`${altGreetings.length} alt greeting${altGreetings.length !== 1 ? 's' : ''}`);
    const lorebook = charxData.card._raw?.character_book as { entries?: unknown[] } | undefined;
    if (lorebook?.entries?.length)
      charxBadges.push(`${lorebook.entries.length} lorebook entr${lorebook.entries.length !== 1 ? 'ies' : 'y'}`);
  }

  return (
    <motion.div
      className={styles.overlay}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      onClick={onClose}
    >
      <motion.div
        className={styles.modal}
        initial={{ scale: 0.95, opacity: 0, y: 20 }}
        animate={{ scale: 1, opacity: 1, y: 0 }}
        exit={{ scale: 0.95, opacity: 0, y: 20 }}
        onClick={(e) => e.stopPropagation()}
      >
        <button className={styles.closeBtn} onClick={onClose}>
          <X size={20} />
        </button>

        <h2 className={styles.title}>Create Character</h2>
        <p className={styles.subtitle}>
          Upload a character card PNG or .charx file to auto-fill, or fill in the fields manually. Only the name is required.
        </p>

        {success ? (
          <div className={styles.successState}>
            <Check size={48} />
            <span>Character created!</span>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className={styles.form}>
            <div className={styles.imageUpload} onClick={() => fileInputRef.current?.click()}>
              {imagePreview ? (
                <img src={imagePreview} alt="Preview" className={styles.imagePreview} />
              ) : (
                <div className={styles.imagePlaceholder}>
                  <ImageIcon size={32} />
                  <span>Drop a character card PNG or .charx to auto-fill, or click to browse</span>
                </div>
              )}
              <input
                ref={fileInputRef}
                type="file"
                accept="image/png,.charx,application/zip"
                onChange={handleFileChange}
                style={{ display: 'none' }}
              />
            </div>

            {autofilled && !isCharx && (
              <div className={styles.autofillBanner}>
                <FileCheck size={16} />
                Character data detected and auto-filled from PNG. Review and edit as needed.
              </div>
            )}

            {autofilled && isCharx && (
              <div className={styles.autofillBanner}>
                <Package size={16} />
                .charx archive loaded. All character data, assets, and metadata will be imported.
              </div>
            )}

            {/* Charx asset summary */}
            {isCharx && charxBadges.length > 0 && (
              <div className={styles.charxSummary}>
                {charxBadges.map((badge) => (
                  <span key={badge} className={styles.charxBadge}>{badge}</span>
                ))}
              </div>
            )}

            {/* Charx: Expressions preview */}
            {isCharx && charxData && charxData.expressions.length > 0 && (
              <div className={styles.charxSection}>
                <span className={styles.charxSectionLabel}>Expressions</span>
                <div className={styles.expressionsGrid}>
                  {charxData.expressions.map((expr) => (
                    <div key={expr.label} className={styles.expressionItem}>
                      <img src={expr.url} alt={expr.label} className={styles.expressionImg} />
                      <span className={styles.expressionLabel}>{expr.label}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Charx: Alternate avatars preview */}
            {isCharx && charxData && charxData.alternateAvatars.length > 0 && (
              <div className={styles.charxSection}>
                <span className={styles.charxSectionLabel}>Alternate Avatars</span>
                <div className={styles.altAvatarsRow}>
                  {charxData.alternateAvatars.map((avatar) => (
                    <div key={avatar.id} className={styles.altAvatarItem}>
                      <img src={avatar.url} alt={avatar.label} className={styles.altAvatarImg} />
                      <span className={styles.altAvatarLabel}>{avatar.label}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Charx: Gallery preview */}
            {isCharx && charxData && charxData.gallery.length > 0 && (
              <div className={styles.charxSection}>
                <span className={styles.charxSectionLabel}>Gallery</span>
                <div className={styles.galleryGrid}>
                  {charxData.gallery.map((img) => (
                    <img key={img.id} src={img.url} alt="Gallery" className={styles.galleryImg} />
                  ))}
                </div>
              </div>
            )}

            {/* Charx: Alternate fields preview */}
            {isCharx && charxData?.lumiverseModules?.alternate_fields && (
              <div className={styles.charxSection}>
                <span className={styles.charxSectionLabel}>Alternate Fields</span>
                {Object.entries(charxData.lumiverseModules.alternate_fields).map(([field, entries]) => (
                  entries.length > 0 && (
                    <div key={field} className={styles.altFieldSection}>
                      <span className={styles.altFieldHeader}>{field}</span>
                      {entries.map((entry) => (
                        <div key={entry.id} className={styles.altFieldEntry}>
                          <span className={styles.altFieldEntryLabel}>{entry.label}:</span>
                          {entry.content.slice(0, 120)}{entry.content.length > 120 ? '...' : ''}
                        </div>
                      ))}
                    </div>
                  )
                ))}
              </div>
            )}

            <div className={styles.fieldGrid}>
              <div className={styles.field}>
                <label>Name *</label>
                <input
                  type="text"
                  placeholder="Character name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
              </div>

              <div className={styles.field}>
                <label>Creator</label>
                <input
                  type="text"
                  placeholder="Your name or handle"
                  value={creator}
                  onChange={(e) => setCreator(e.target.value)}
                />
              </div>
            </div>

            <div className={styles.field}>
              <label>Description</label>
              <textarea
                placeholder="Who is this character? Background, appearance, etc."
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={3}
              />
            </div>

            <div className={styles.field}>
              <label>Personality</label>
              <textarea
                placeholder="Personality traits, quirks, behavior patterns"
                value={personality}
                onChange={(e) => setPersonality(e.target.value)}
                rows={2}
              />
            </div>

            <div className={styles.field}>
              <label>Scenario</label>
              <textarea
                placeholder="The setting or situation for conversations"
                value={scenario}
                onChange={(e) => setScenario(e.target.value)}
                rows={2}
              />
            </div>

            <div className={styles.field}>
              <label>First Message</label>
              <textarea
                placeholder="The opening message this character sends"
                value={firstMes}
                onChange={(e) => setFirstMes(e.target.value)}
                rows={3}
              />
            </div>

            <div className={styles.fieldGrid}>
              <div className={styles.field}>
                <label>Tags</label>
                <input
                  type="text"
                  placeholder="fantasy, romance, adventure"
                  value={tags}
                  onChange={(e) => setTags(e.target.value)}
                />
              </div>

              <div className={styles.field}>
                <label>Creator Notes</label>
                <input
                  type="text"
                  placeholder="Notes for users of this character"
                  value={creatorNotes}
                  onChange={(e) => setCreatorNotes(e.target.value)}
                />
              </div>
            </div>

            {error && (
              <div className={styles.errorBox}>
                <AlertCircle size={16} />
                {error}
              </div>
            )}

            <button type="submit" className={styles.submitBtn} disabled={submitting}>
              <Upload size={18} />
              {submitting ? 'Uploading...' : isCharx ? 'Import .charx' : 'Create Character'}
            </button>
          </form>
        )}
      </motion.div>
    </motion.div>
  );
};

export default CreateCharacterModal;
