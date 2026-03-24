import {
  createCliRenderer,
  ScrollBoxRenderable,
  BoxRenderable,
  TextRenderable,
  RGBA,
  CliRenderer,
  createTimeline,
  engine,
} from '@opentui/core';
// @ts-ignore
import type { Subprocess } from 'bun';

// LumiHub colour palette
const COLORS = {
  // Core brand
  primary: RGBA.fromInts(125, 95, 190),
  primaryHover: RGBA.fromInts(140, 110, 205),
  primaryText: RGBA.fromInts(160, 130, 220),
  primaryMuted: RGBA.fromInts(125, 95, 190, 128),
  primaryGlow: RGBA.fromInts(147, 112, 219),
  bgDeep: RGBA.fromHex('#0a0812'),
  bg: RGBA.fromInts(28, 24, 38),
  bgElevated: RGBA.fromInts(35, 30, 48),
  bgHeader: RGBA.fromInts(10, 8, 18),
  border: RGBA.fromInts(147, 112, 219, 30),
  borderHover: RGBA.fromInts(147, 112, 219, 64),
  borderFocused: RGBA.fromInts(147, 112, 219, 180),
  text: RGBA.fromInts(255, 255, 255, 230),
  textMuted: RGBA.fromInts(255, 255, 255, 166),
  textDim: RGBA.fromInts(255, 255, 255, 102),
  textHint: RGBA.fromInts(255, 255, 255, 77),
  danger: RGBA.fromHex('#ef4444'),
  success: RGBA.fromHex('#22c55e'),
  warning: RGBA.fromHex('#f59e0b'),
  info: RGBA.fromInts(125, 95, 190),
};

// Loom symbols
const SYM = {
  thread: '\u2500',    // ─
  weave: '\u2504',     // ┄
  spindle: '\u25C8',   // ◈
  knot: '\u2731',      // ✱
  strand: '\u2502',    // │
  loom: '\u25C7',      // ◇
  fate: '\u2726',      // ✦
  spark: '\u2022',     // •
};

// CLI args
const isProduction = process.argv.includes('--production') || process.argv.includes('-p');

// State
let renderer: CliRenderer;
let backendProc: Subprocess | null = null;
let frontendProc: Subprocess | null = null;
let backendScrollbox: ScrollBoxRenderable;
let frontendScrollbox: ScrollBoxRenderable;
let backendPanel: BoxRenderable;
let frontendPanel: BoxRenderable;
let statusDot: TextRenderable;
let statusFocusText: TextRenderable;
let focusedPanel: 'backend' | 'frontend' = 'backend';

// Helpers

function logColor(line: string): RGBA {
  const l = line.toLowerCase();
  if (l.includes('error') || l.includes('err:') || l.includes('failed') || l.includes('exception')) return COLORS.danger;
  if (l.includes('warn') || l.includes('warning')) return COLORS.warning;
  if (l.includes('ready') || l.includes('success') || l.includes('connected') || l.includes('running on')) return COLORS.success;
  if (l.includes('info') || l.includes('starting') || l.includes('loaded')) return COLORS.primaryText;
  return COLORS.textMuted;
}

function timestamp(): string {
  const d = new Date();
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`;
}

function appendLog(scrollbox: ScrollBoxRenderable, line: string, color?: RGBA) {
  const ts = timestamp();
  scrollbox.content.add(
    new TextRenderable(renderer, {
      content: ` ${ts} ${SYM.strand} ${line}`,
      fg: color ?? logColor(line),
      width: '100%',
    })
  );
}

async function pipeStream(
  stream: ReadableStream<Uint8Array> | null,
  scrollbox: ScrollBoxRenderable,
) {
  if (!stream) return;
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() ?? '';
      for (const line of lines) {
        if (line.trim()) appendLog(scrollbox, line);
      }
    }
    if (buffer.trim()) appendLog(scrollbox, buffer);
  } catch {
    // Thread severed
  }
}

// Process management

function spawnBackend() {
  if (backendProc) {
    backendProc.kill();
    backendProc = null;
  }
  for (const child of backendScrollbox.content.getChildren()) {
    backendScrollbox.content.remove(child.id);
  }
  const mode = isProduction ? 'production' : 'development';
  appendLog(backendScrollbox, `Starting backend (${mode})...`, COLORS.primaryText);

  const backendCmd = isProduction
    ? ['bun', 'src/index.ts']
    : ['bun', '--watch', 'src/index.ts'];

  // @ts-ignore
  backendProc = Bun.spawn(backendCmd, {
    // @ts-ignore
    cwd: `${import.meta.dir}/backend`,
    stdout: 'pipe',
    stderr: 'pipe',
    env: { ...process.env, FORCE_COLOR: '0', NODE_ENV: mode },
  });

  pipeStream(backendProc.stdout, backendScrollbox);
  pipeStream(backendProc.stderr, backendScrollbox);
}

function spawnFrontend() {
  if (isProduction) {
    appendLog(frontendScrollbox, 'Production mode — frontend served by backend', COLORS.success);
    return;
  }

  if (frontendProc) {
    frontendProc.kill();
    frontendProc = null;
  }
  for (const child of frontendScrollbox.content.getChildren()) {
    frontendScrollbox.content.remove(child.id);
  }
  appendLog(frontendScrollbox, 'Starting frontend...', COLORS.primaryText);

  // @ts-ignore
  frontendProc = Bun.spawn(['bunx', '--bun', 'vite'], {
    // @ts-ignore
    cwd: import.meta.dir,
    stdout: 'pipe',
    stderr: 'pipe',
    env: { ...process.env, FORCE_COLOR: '0' },
  });

  pipeStream(frontendProc.stdout, frontendScrollbox);
  pipeStream(frontendProc.stderr, frontendScrollbox);
}

function killAll() {
  backendProc?.kill();
  frontendProc?.kill();
}

// Focus management

function updateFocus() {
  const isBE = focusedPanel === 'backend';

  backendPanel.borderColor = isBE ? COLORS.borderFocused : COLORS.border;
  backendPanel.title = isBE ? ` ${SYM.fate} Backend ` : ` ${SYM.spark} Backend `;

  frontendPanel.borderColor = !isBE ? COLORS.borderFocused : COLORS.border;
  frontendPanel.title = !isBE ? ` ${SYM.fate} Frontend ` : ` ${SYM.spark} Frontend `;

  if (statusFocusText) {
    statusFocusText.content = isBE ? `${SYM.fate} Backend` : `${SYM.fate} Frontend`;
  }

  if (isBE) {
    renderer.focusRenderable(backendScrollbox);
  } else {
    renderer.focusRenderable(frontendScrollbox);
  }
}

// Build the UI

async function main() {
  renderer = await createCliRenderer({
    exitOnCtrlC: false,
    targetFps: 30,
  });

  engine.attach(renderer);

  const header = new BoxRenderable(renderer, {
    width: '100%',
    height: 3,
    flexShrink: 0,
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    gap: 2,
    backgroundColor: COLORS.bgHeader,
    borderStyle: 'rounded',
    border: ['bottom'] as any,
    borderColor: COLORS.border,
  });

  const threadLeft = new TextRenderable(renderer, {
    content: `${SYM.weave.repeat(6)}${SYM.fate}${SYM.thread}`,
    fg: COLORS.primaryMuted,
  });

  const titleText = new TextRenderable(renderer, {
    content: 'LumiHub',
    fg: COLORS.primaryGlow,
    attributes: 1,
  });

  const threadRight = new TextRenderable(renderer, {
    content: `${SYM.thread}${SYM.fate}${SYM.weave.repeat(6)}`,
    fg: COLORS.primaryMuted,
  });

  header.add(threadLeft);
  header.add(titleText);
  header.add(threadRight);

  backendScrollbox = new ScrollBoxRenderable(renderer, {
    id: 'backend-logs',
    flexGrow: 1,
    stickyScroll: true,
    stickyStart: 'bottom',
    backgroundColor: COLORS.bgDeep,
  });

  backendPanel = new BoxRenderable(renderer, {
    id: 'backend-panel',
    flexGrow: 1,
    flexDirection: 'column',
    borderStyle: 'rounded',
    borderColor: COLORS.borderFocused,
    title: ` ${SYM.fate} Backend `,
    titleAlignment: 'center',
    backgroundColor: COLORS.bg,
  });
  backendPanel.add(backendScrollbox);

  frontendScrollbox = new ScrollBoxRenderable(renderer, {
    id: 'frontend-logs',
    flexGrow: 1,
    stickyScroll: true,
    stickyStart: 'bottom',
    backgroundColor: COLORS.bgDeep,
  });

  frontendPanel = new BoxRenderable(renderer, {
    id: 'frontend-panel',
    flexGrow: 1,
    flexDirection: 'column',
    borderStyle: 'rounded',
    borderColor: COLORS.border,
    title: ` ${SYM.spark} Frontend `,
    titleAlignment: 'center',
    backgroundColor: COLORS.bg,
  });
  frontendPanel.add(frontendScrollbox);

  const panelsRow = new BoxRenderable(renderer, {
    width: '100%',
    flexGrow: 1,
    flexDirection: 'row',
    gap: 1,
    paddingLeft: 1,
    paddingRight: 1,
    paddingTop: 1,
    backgroundColor: COLORS.bgDeep,
  });
  panelsRow.add(backendPanel);
  panelsRow.add(frontendPanel);

  const statusBar = new BoxRenderable(renderer, {
    width: '100%',
    height: 3,
    minHeight: 3,
    flexShrink: 0,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    backgroundColor: COLORS.bgHeader,
    borderStyle: 'rounded',
    border: ['top'] as any,
    borderColor: COLORS.border,
    paddingLeft: 1,
    paddingRight: 1,
  });

  const statusLeft = new BoxRenderable(renderer, {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 1,
  });

  statusDot = new TextRenderable(renderer, {
    content: `${SYM.spindle}`,
    fg: COLORS.success,
  });

  const statusEnv = new TextRenderable(renderer, {
    content: isProduction ? 'production' : 'development',
    fg: isProduction ? COLORS.success : COLORS.textDim,
  });

  const statusSep1 = new TextRenderable(renderer, {
    content: `${SYM.strand}`,
    fg: COLORS.border,
  });

  statusFocusText = new TextRenderable(renderer, {
    content: `${SYM.fate} Backend`,
    fg: COLORS.primaryText,
  });

  statusLeft.add(statusDot);
  statusLeft.add(statusEnv);
  statusLeft.add(statusSep1);
  statusLeft.add(statusFocusText);

  const statusRight = new BoxRenderable(renderer, {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 1,
  });

  const keys = [
    { key: 'tab', label: 'Switch Panel' },
    { key: 'r', label: 'Restart Backend' },
    { key: 'f', label: 'Restart Frontend' },
    { key: 'q', label: 'Quit' },
  ];

  for (let i = 0; i < keys.length; i++) {
    if (i > 0) {
      statusRight.add(new TextRenderable(renderer, {
        content: SYM.strand,
        fg: COLORS.border,
      }));
    }
    statusRight.add(new TextRenderable(renderer, {
      content: keys[i].key,
      fg: COLORS.primaryText,
      attributes: 1,
    }));
    statusRight.add(new TextRenderable(renderer, {
      content: keys[i].label,
      fg: COLORS.textDim,
    }));
  }

  statusBar.add(statusLeft);
  statusBar.add(statusRight);

  const rootLayout = new BoxRenderable(renderer, {
    width: '100%',
    height: '100%',
    flexDirection: 'column',
    backgroundColor: COLORS.bgDeep,
  });
  rootLayout.add(header);
  rootLayout.add(panelsRow);
  rootLayout.add(statusBar);

  renderer.root.add(rootLayout);

  const glowTarget = { r: 147, g: 112, b: 219 };
  createTimeline({ loop: true })
    .add(glowTarget, {
      r: 200, g: 170, b: 245,
      duration: 3000,
      ease: 'inOutSine',
      alternate: true,
      loop: true,
      onUpdate: () => {
        titleText.fg = RGBA.fromInts(
          Math.round(glowTarget.r),
          Math.round(glowTarget.g),
          Math.round(glowTarget.b),
        );
      },
    })
    .play();

  const threadTarget = { a: 128 };
  createTimeline({ loop: true })
    .add(threadTarget, {
      a: 35,
      duration: 2500,
      ease: 'inOutSine',
      alternate: true,
      loop: true,
      onUpdate: () => {
        const c = RGBA.fromInts(125, 95, 190, Math.round(threadTarget.a));
        threadLeft.fg = c;
        threadRight.fg = c;
      },
    })
    .play();

  const dotTarget = { r: 34, g: 197, b: 94, a: 255 };
  createTimeline({ loop: true })
    .add(dotTarget, {
      a: 80,
      duration: 1800,
      ease: 'inOutSine',
      alternate: true,
      loop: true,
      onUpdate: () => {
        statusDot.fg = RGBA.fromInts(
          Math.round(dotTarget.r),
          Math.round(dotTarget.g),
          Math.round(dotTarget.b),
          Math.round(dotTarget.a),
        );
      },
    })
    .play();

  const borderTarget = { a: 180 };
  createTimeline({ loop: true })
    .add(borderTarget, {
      a: 80,
      duration: 2500,
      ease: 'inOutSine',
      alternate: true,
      loop: true,
      onUpdate: () => {
        const focused = focusedPanel === 'backend' ? backendPanel : frontendPanel;
        focused.borderColor = RGBA.fromInts(147, 112, 219, Math.round(borderTarget.a));
      },
    })
    .play();

  renderer.addInputHandler((sequence: string) => {
    if (sequence === '\x03' || sequence === 'q') {
      killAll();
      renderer.destroy();
      process.exit(0);
    }
    if (sequence === '\t') {
      focusedPanel = focusedPanel === 'backend' ? 'frontend' : 'backend';
      updateFocus();
      return true;
    }
    if (sequence === 'r') {
      spawnBackend();
      return true;
    }
    if (sequence === 'f') {
      spawnFrontend();
      return true;
    }
    return false;
  });

  updateFocus();
  spawnBackend();
  spawnFrontend();

  process.on('SIGINT', () => {
    killAll();
    renderer.destroy();
    process.exit(0);
  });
  process.on('SIGTERM', () => {
    killAll();
    renderer.destroy();
    process.exit(0);
  });

  renderer.start();
}

main().catch((err) => {
  console.error('TUI failed to start:', err);
  killAll();
  process.exit(1);
});
