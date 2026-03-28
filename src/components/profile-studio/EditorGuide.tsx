import { useState } from 'react';
import { X, ChevronLeft, ChevronRight, Wand2, Code, Crosshair, MousePointer, Paintbrush, FileCode, Save } from 'lucide-react';
import styles from './EditorGuide.module.css';

interface EditorGuideProps {
  onClose: () => void;
}

interface GuideStep {
  title: string;
  icon: React.ReactNode;
  content: React.ReactNode;
}

const STEPS: GuideStep[] = [
  {
    title: 'Welcome to Profile Studio',
    icon: <Paintbrush size={20} />,
    content: (
      <>
        <p>Profile Studio lets you fully customize your LumiHub profile with <strong>HTML</strong> and <strong>CSS</strong>. You can restyle everything on this page — including the navbar.</p>
        <p>There are two editing modes to choose from, and a set of tools to help you target and style any element.</p>
      </>
    ),
  },
  {
    title: 'Visual Mode vs Code Mode',
    icon: <Wand2 size={20} />,
    content: (
      <>
        <div className={styles.modeRow}>
          <div className={styles.modeCard}>
            <div className={styles.modeIcon}><Wand2 size={16} /></div>
            <h4>Visual Mode</h4>
            <p>Click any element on your profile to select it. A floating panel appears with controls for colors, spacing, borders, fonts, and effects — no CSS knowledge needed.</p>
          </div>
          <div className={styles.modeCard}>
            <div className={styles.modeIcon}><Code size={16} /></div>
            <h4>Code Mode</h4>
            <p>Write raw CSS and HTML directly. Clicking an element auto-inserts a CSS selector stub for it. Full control for experienced users.</p>
          </div>
        </div>
        <p className={styles.hint}>Toggle between modes using the <Wand2 size={12} /> / <Code size={12} /> buttons in the title bar.</p>
      </>
    ),
  },
  {
    title: 'Element Picker',
    icon: <Crosshair size={20} />,
    content: (
      <>
        <p>Click the <strong><Crosshair size={12} /> crosshair</strong> button in the title bar to activate the element picker.</p>
        <p>Hover over any element on the page — it highlights with a tooltip showing its name. Click to select it.</p>
        <ul>
          <li><strong>Visual Mode:</strong> Opens a floating control panel for that element</li>
          <li><strong>Code Mode:</strong> Inserts a CSS rule stub into your stylesheet</li>
        </ul>
        <p className={styles.hint}>You can also use the <strong><MousePointer size={12} /> Element List</strong> dropdown to pick elements by name.</p>
      </>
    ),
  },
  {
    title: 'The CSS Tab',
    icon: <Paintbrush size={20} />,
    content: (
      <>
        <p>Your custom CSS is injected into the page globally — it can target <strong>anything</strong>, including the navbar, body, and layout wrapper.</p>
        <p>Profile elements use <code>data-studio</code> attributes as stable CSS hooks:</p>
        <div className={styles.codeBlock}>
          <code>{`[data-studio="banner"] {\n  height: 400px;\n  filter: saturate(1.5);\n}\n\n[data-studio="header"] {\n  background: transparent !important;\n}`}</code>
        </div>
        <p className={styles.hint}>CSS Module classes get hashed at build time — always target <code>data-studio</code> attributes instead of class names.</p>
      </>
    ),
  },
  {
    title: 'The HTML Tab',
    icon: <FileCode size={20} />,
    content: (
      <>
        <p>Write custom HTML to completely replace the default profile layout. When you save HTML, it replaces the standard template — your avatar, stats, and characters won't render automatically.</p>
        <p><strong>Leave this empty</strong> to keep the default profile (recommended). Use the CSS tab to restyle the existing layout instead.</p>
        <p className={styles.hint}>Custom HTML is sanitized — no scripts, iframes, or forms. Allowed tags include div, span, p, h1-h6, a, img, video, section, and more.</p>
      </>
    ),
  },
  {
    title: 'Saving & Recovery',
    icon: <Save size={20} />,
    content: (
      <>
        <p>Your changes are previewed live as you type. Nothing is saved to the server until you click <strong>Save</strong>.</p>
        <p>If you break something and can't access the editor:</p>
        <ul>
          <li>Add <code>?studio=1</code> to your profile URL — forces the editor open with the default template visible</li>
          <li>Add <code>?studio=reset</code> — wipes all custom HTML and CSS</li>
          <li>Press <code>Ctrl+Shift+E</code> on any profile page to toggle the editor</li>
        </ul>
        <p className={styles.hint}>The <strong>Reset</strong> button in the editor footer clears everything back to the default profile.</p>
      </>
    ),
  },
];

const EditorGuide = ({ onClose }: EditorGuideProps) => {
  const [step, setStep] = useState(0);
  const current = STEPS[step];
  const isFirst = step === 0;
  const isLast = step === STEPS.length - 1;

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div className={styles.header}>
          <div className={styles.headerIcon}>{current.icon}</div>
          <h2 className={styles.headerTitle}>{current.title}</h2>
          <button className={styles.closeBtn} onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className={styles.body}>
          {current.content}
        </div>

        <div className={styles.footer}>
          <div className={styles.dots}>
            {STEPS.map((_, i) => (
              <button
                key={i}
                className={`${styles.dot} ${i === step ? styles.dotActive : ''}`}
                onClick={() => setStep(i)}
              />
            ))}
          </div>

          <div className={styles.navBtns}>
            <button
              className={styles.navBtn}
              onClick={() => setStep(step - 1)}
              disabled={isFirst}
            >
              <ChevronLeft size={16} /> Back
            </button>
            {isLast ? (
              <button className={styles.doneBtn} onClick={onClose}>
                Got it
              </button>
            ) : (
              <button className={styles.nextBtn} onClick={() => setStep(step + 1)}>
                Next <ChevronRight size={16} />
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default EditorGuide;
