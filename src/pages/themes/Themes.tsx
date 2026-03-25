import { Palette, Lock, Bell } from 'lucide-react';
import styles from './Themes.module.css';

const MOCK_THEMES = [
  {
    name: 'Rosewood Dusk',
    author: 'LumiHub',
    gradient: 'linear-gradient(135deg, #2a1520 0%, #3d1f2e 40%, #1a1020 100%)',
    dots: ['#d88c9a', '#b87482', '#8b4558', '#2a1520', '#f2c7d0'],
  },
  {
    name: 'Midnight Orchid',
    author: 'LumiHub',
    gradient: 'linear-gradient(135deg, #1a1030 0%, #2d1b4e 40%, #0f0a1e 100%)',
    dots: ['#b185db', '#8b5fbf', '#6a3fa0', '#1a1030', '#d4b8f0'],
  },
  {
    name: 'Arctic Glass',
    author: 'LumiHub',
    gradient: 'linear-gradient(135deg, #0a1520 0%, #152535 40%, #0a0f18 100%)',
    dots: ['#7ec8e3', '#4a9ec2', '#2d6f8e', '#0a1520', '#b8e4f5'],
  },
  {
    name: 'Ember Glow',
    author: 'LumiHub',
    gradient: 'linear-gradient(135deg, #201510 0%, #3d2518 40%, #1a100a 100%)',
    dots: ['#e8a56a', '#c47b42', '#8b5530', '#201510', '#f5d0a8'],
  },
  {
    name: 'Forest Canopy',
    author: 'LumiHub',
    gradient: 'linear-gradient(135deg, #0e1a12 0%, #1b3022 40%, #0a140e 100%)',
    dots: ['#6bcf8a', '#4aaa65', '#2d7d44', '#0e1a12', '#a8e8bd'],
  },
  {
    name: 'Slate Minimal',
    author: 'LumiHub',
    gradient: 'linear-gradient(135deg, #141416 0%, #1e1e22 40%, #0c0c0e 100%)',
    dots: ['#a1a1aa', '#71717a', '#52525b', '#141416', '#d4d4d8'],
  },
];

const Themes = () => {
  return (
    <div className={styles.page}>
      <div className={styles.hero}>
        <div className={styles.heroIcon}>
          <Palette size={28} />
        </div>
        <h1 className={styles.heroTitle}>
          <span className={styles.heroAccent}>Themes</span>
        </h1>
        <p className={styles.heroSub}>
          Customize your Lumiverse experience with curated color palettes
          and UI themes crafted by the community.
        </p>
      </div>

      <div className={styles.previewGrid}>
        {MOCK_THEMES.map((theme) => (
          <div key={theme.name} className={styles.previewCard}>
            <div
              className={styles.previewSwatch}
              style={{ background: theme.gradient }}
            />
            <div className={styles.previewBody}>
              <p className={styles.previewName}>{theme.name}</p>
              <p className={styles.previewMeta}>by {theme.author}</p>
              <div className={styles.colorDots}>
                {theme.dots.map((color, i) => (
                  <span
                    key={i}
                    className={styles.colorDot}
                    style={{ background: color }}
                  />
                ))}
              </div>
            </div>
            <div className={styles.lockedOverlay}>
              <span className={styles.lockedBadge}>
                <Lock size={14} />
                Coming Soon
              </span>
            </div>
          </div>
        ))}
      </div>

      <div className={styles.ctaSection}>
        <h2 className={styles.ctaTitle}>Want to know when themes launch?</h2>
        <p className={styles.ctaDesc}>
          Theme support is being built right now. Soon you'll be able to
          browse, install, and create your own themes for Lumiverse.
        </p>
        <span className={styles.ctaButton}>
          <Bell size={15} />
          Notifications coming soon
        </span>
      </div>
    </div>
  );
};

export default Themes;
