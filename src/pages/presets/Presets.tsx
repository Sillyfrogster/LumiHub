import { Sparkles, Lock, Bell, Flame, Feather, Zap, Brain, Snowflake, Target } from 'lucide-react';
import styles from './Presets.module.css';

interface MockParam {
  label: string;
  value: string;
  fill: number; // 0-100%
}

interface MockPreset {
  name: string;
  author: string;
  icon: React.ElementType;
  accent: string;
  params: MockParam[];
}

const MOCK_PRESETS: MockPreset[] = [
  {
    name: 'Creative Blaze',
    author: 'LumiHub',
    icon: Flame,
    accent: '#e8865a',
    params: [
      { label: 'Temperature', value: '1.35', fill: 90 },
      { label: 'Top-P', value: '0.95', fill: 95 },
      { label: 'Top-K', value: '80', fill: 53 },
      { label: 'Rep. Penalty', value: '1.08', fill: 36 },
    ],
  },
  {
    name: 'Soft Prose',
    author: 'LumiHub',
    icon: Feather,
    accent: '#d88c9a',
    params: [
      { label: 'Temperature', value: '0.95', fill: 63 },
      { label: 'Top-P', value: '0.88', fill: 88 },
      { label: 'Top-K', value: '40', fill: 27 },
      { label: 'Rep. Penalty', value: '1.12', fill: 47 },
    ],
  },
  {
    name: 'Lightning Fast',
    author: 'LumiHub',
    icon: Zap,
    accent: '#e8c84a',
    params: [
      { label: 'Temperature', value: '0.70', fill: 47 },
      { label: 'Top-P', value: '0.80', fill: 80 },
      { label: 'Max Tokens', value: '256', fill: 12 },
      { label: 'Rep. Penalty', value: '1.15', fill: 58 },
    ],
  },
  {
    name: 'Deep Thinker',
    author: 'LumiHub',
    icon: Brain,
    accent: '#b185db',
    params: [
      { label: 'Temperature', value: '0.55', fill: 37 },
      { label: 'Top-P', value: '0.70', fill: 70 },
      { label: 'Top-K', value: '30', fill: 20 },
      { label: 'Rep. Penalty', value: '1.20', fill: 67 },
    ],
  },
  {
    name: 'Frozen Logic',
    author: 'LumiHub',
    icon: Snowflake,
    accent: '#7ec8e3',
    params: [
      { label: 'Temperature', value: '0.20', fill: 13 },
      { label: 'Top-P', value: '0.50', fill: 50 },
      { label: 'Top-K', value: '10', fill: 7 },
      { label: 'Rep. Penalty', value: '1.05', fill: 25 },
    ],
  },
  {
    name: 'Balanced Focus',
    author: 'LumiHub',
    icon: Target,
    accent: '#6bcf8a',
    params: [
      { label: 'Temperature', value: '0.80', fill: 53 },
      { label: 'Top-P', value: '0.90', fill: 90 },
      { label: 'Top-K', value: '50', fill: 33 },
      { label: 'Rep. Penalty', value: '1.10', fill: 42 },
    ],
  },
];

const Presets = () => {
  return (
    <div className={styles.page}>
      <div className={styles.hero}>
        <div className={styles.heroIcon}>
          <Sparkles size={28} />
        </div>
        <h1 className={styles.heroTitle}>
          <span className={styles.heroAccent}>Presets</span>
        </h1>
        <p className={styles.heroSub}>
          Fine-tuned generation settings shared by the community.
          One-click install into Lumiverse for the perfect output every time.
        </p>
      </div>

      <div className={styles.presetGrid}>
        {MOCK_PRESETS.map((preset) => (
          <div key={preset.name} className={styles.presetCard}>
            <div className={styles.presetHeader}>
              <div
                className={styles.presetIconWrap}
                style={{ background: `${preset.accent}15`, color: preset.accent }}
              >
                <preset.icon size={20} />
              </div>
              <div>
                <p className={styles.presetName}>{preset.name}</p>
                <p className={styles.presetAuthor}>by {preset.author}</p>
              </div>
            </div>

            <div className={styles.paramList}>
              {preset.params.map((param) => (
                <div key={param.label} className={styles.param}>
                  <div className={styles.paramRow}>
                    <span className={styles.paramLabel}>{param.label}</span>
                    <span className={styles.paramValue}>{param.value}</span>
                  </div>
                  <div className={styles.paramTrack}>
                    <div
                      className={styles.paramFill}
                      style={{
                        width: `${param.fill}%`,
                        background: preset.accent,
                        opacity: 0.6,
                      }}
                    />
                  </div>
                </div>
              ))}
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
        <h2 className={styles.ctaTitle}>Share your perfect settings</h2>
        <p className={styles.ctaDesc}>
          Preset sharing is on the way. You'll be able to publish, discover,
          and install generation presets directly into Lumiverse.
        </p>
        <span className={styles.ctaButton}>
          <Bell size={15} />
          Notifications coming soon
        </span>
      </div>
    </div>
  );
};

export default Presets;
