import type { PerspectiveCamera, Texture, WebGLRenderer } from "three";
import { createInvertedAtlas } from "./scene/atlas-theme";
import { createComposer } from "./scene/composer";
import { createStudioEnvironment } from "./scene/environment";
import { createField } from "./scene/field";
import { createHost } from "./scene/host";
import { clamp, damp, drift, mix } from "./scene/motion";
import { getPalette } from "./scene/palette";
import type {
  PointerPosition,
  SceneLayout,
  SceneTheme,
  ThreeModule,
} from "./scene/types";

export type LandingSceneTheme = SceneTheme;

export type LivingCatalogScene = {
  dispose: () => void;
  resize: (width: number, height: number) => void;
  setPointer: (pointer: PointerPosition) => void;
  setProgress: (progress: number) => void;
  setReducedMotion: (reduced: boolean) => void;
  setTheme: (theme: SceneTheme) => void;
  start: () => void;
  stop: () => void;
};

export type SceneOptions = {
  atlasUrl: string;
  hostUrl: string;
  hostDepthUrl: string;
  depthOfField: boolean;
  onAtlasFailure?: () => void;
};

const PROGRESS_SMOOTHING = 0.085;
const POINTER_SMOOTHING = 0.19;

/** The hub sits right of centre so the copy column stays clear. */
const FRAME_SHIFT = 0.16;

export function createLivingCatalogScene(
  THREE: ThreeModule,
  canvas: HTMLCanvasElement,
  initialTheme: SceneTheme,
  options: SceneOptions,
): LivingCatalogScene {
  const renderer: WebGLRenderer = new THREE.WebGLRenderer({
    alpha: false,
    antialias: false,
    canvas,
    powerPreference: "high-performance",
  });
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.ACESFilmicToneMapping;

  let theme = initialTheme;
  let palette = getPalette(theme);
  renderer.toneMappingExposure = palette.exposure;

  const scene = new THREE.Scene();
  scene.background = new THREE.Color(palette.background);
  scene.fog = new THREE.FogExp2(palette.fog, 0.021);

  let environment = createStudioEnvironment(THREE, renderer, theme);
  scene.environment = environment;

  const camera: PerspectiveCamera = new THREE.PerspectiveCamera(42, 1, 0.1, 90);

  const keyLight = new THREE.DirectionalLight(
    palette.keyLight,
    palette.keyIntensity,
  );
  keyLight.position.set(8, 9, 6);
  scene.add(keyLight);

  const fillLight = new THREE.PointLight(
    palette.fillLight,
    palette.fillIntensity,
    26,
    2,
  );
  fillLight.position.set(-6, -2, 4);
  scene.add(fillLight);

  // A cold rim, so card edges separate from the black.
  const rimLight = new THREE.DirectionalLight(0xb8dfe2, 0.85);
  rimLight.position.set(-9, 1.5, -7);
  scene.add(rimLight);

  /** What every card is arriving at. */
  const coreLight = new THREE.PointLight(palette.beam, 9, 11, 2);
  coreLight.position.set(0, 0, 0);
  scene.add(coreLight);

  let invertedAtlas: Texture | null = null;

  const atlas: Texture = new THREE.TextureLoader().load(
    options.atlasUrl,
    (loaded) => {
      invertedAtlas = createInvertedAtlas(THREE, loaded);
      applyCardTone();
    },
    undefined,
    options.onAtlasFailure,
  );
  atlas.colorSpace = THREE.SRGBColorSpace;
  atlas.anisotropy = 8;

  const field = createField(THREE, atlas);
  scene.add(field.mesh);

  const host = createHost(
    THREE,
    options.hostUrl,
    options.hostDepthUrl,
    palette,
    options.onAtlasFailure ?? (() => undefined),
  );
  scene.add(host.group);

  const composer = createComposer(
    THREE,
    renderer,
    scene,
    camera,
    palette,
    options.depthOfField,
  );

  /** Dark theme reads the paper cards, light theme the inverted ones. */
  function applyCardTone() {
    const dark = theme === "dark";
    const wanted = dark ? atlas : (invertedAtlas ?? atlas);
    field.setTexture(wanted);
    field.setTone(dark ? palette.glass : 0xffffff, dark ? 0.55 : 0.34);
  }

  const target = new THREE.Vector3();
  const cameraLocal = new THREE.Vector3();

  let width = 1;
  let height = 1;
  let layout: SceneLayout = "wide";
  let targetProgress = 0;
  let smoothProgress = 0;
  let targetPointer: PointerPosition = { x: 0, y: 0 };
  let pointer: PointerPosition = { x: 0, y: 0 };
  let reducedMotion = false;
  let elapsed = 0;
  let frame = 0;
  let running = false;

  const clock = new THREE.Clock();

  const step = (time: number) => {
    const life = reducedMotion ? 0 : 1;

    field.update(time * life, 1);

    // The camera closes on the hub as you scroll and drifts when you stop.
    const orbit =
      smoothProgress * 0.85 + time * 0.016 * life + pointer.x * 0.055;
    const radius = mix(25, 15, smoothProgress);
    const elevation =
      mix(3.4, 1.6, smoothProgress) +
      pointer.y * -0.5 +
      drift(time, 0.07, 1.4) * 0.12 * life;

    camera.position.set(
      Math.cos(orbit) * radius,
      elevation,
      Math.sin(orbit) * radius,
    );
    target.set(0, mix(0.2, 0.9, smoothProgress), 0);
    camera.lookAt(target);

    // The host keeps to the near side of the hub as the camera comes round,
    // so shelf cards sit behind her and arriving cards cross in front.
    const hostAngle = orbit + 0.82;
    host.group.position.set(
      Math.cos(hostAngle) * 8.8,
      -1.15 + drift(time, 0.08, 2.6) * 0.06 * life,
      Math.sin(hostAngle) * 8.8,
    );
    host.group.lookAt(camera.position);
    host.group.updateMatrixWorld();
    cameraLocal.copy(camera.position);
    host.group.worldToLocal(cameraLocal);
    host.update(time, mix(1, 0.5, smoothProgress), cameraLocal);

    composer.focus(
      mix(radius - 8.8, radius - 4.2, smoothProgress),
      mix(0.00026, 0.00042, smoothProgress),
    );
  };

  const renderFrame = () => {
    const delta = Math.min(clock.getDelta(), 0.05);
    elapsed += delta;

    smoothProgress = reducedMotion
      ? targetProgress
      : damp(smoothProgress, targetProgress, PROGRESS_SMOOTHING, delta);
    pointer = {
      x: damp(pointer.x, targetPointer.x, POINTER_SMOOTHING, delta),
      y: damp(pointer.y, targetPointer.y, POINTER_SMOOTHING, delta),
    };

    step(elapsed);
    composer.render();
  };

  const loop = () => {
    frame = requestAnimationFrame(loop);
    renderFrame();
  };

  const start = () => {
    if (running || reducedMotion) return;
    running = true;
    clock.getDelta();
    frame = requestAnimationFrame(loop);
  };

  const stop = () => {
    running = false;
    if (frame) cancelAnimationFrame(frame);
    frame = 0;
  };

  const resize = (nextWidth: number, nextHeight: number) => {
    width = Math.max(1, Math.floor(nextWidth));
    height = Math.max(1, Math.floor(nextHeight));
    const aspect = width / height;
    layout = aspect < 0.9 ? "portrait" : width < 1080 ? "tablet" : "wide";

    camera.aspect = aspect;
    camera.fov = layout === "portrait" ? 54 : layout === "tablet" ? 48 : 42;

    // Pan the frame so the hub sits beside the copy rather than under it.
    if (layout === "wide") {
      camera.setViewOffset(
        width,
        height,
        -width * FRAME_SHIFT,
        0,
        width,
        height,
      );
    } else {
      camera.clearViewOffset();
    }
    camera.updateProjectionMatrix();

    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 1.5));
    renderer.setSize(width, height, false);
    composer.resize(width, height);
    if (!running) renderFrame();
  };

  const setTheme = (nextTheme: SceneTheme) => {
    if (theme === nextTheme) return;
    theme = nextTheme;
    palette = getPalette(theme);

    renderer.toneMappingExposure = palette.exposure;
    (scene.background as { set: (value: number) => void }).set(
      palette.background,
    );
    (scene.fog as { color: { set: (value: number) => void } }).color.set(
      palette.fog,
    );

    environment.dispose();
    environment = createStudioEnvironment(THREE, renderer, theme);
    scene.environment = environment;

    keyLight.color.set(palette.keyLight);
    keyLight.intensity = palette.keyIntensity;
    fillLight.color.set(palette.fillLight);
    fillLight.intensity = palette.fillIntensity;
    coreLight.color.set(palette.beam);
    coreLight.intensity = theme === "dark" ? 9 : 3;
    rimLight.intensity = theme === "dark" ? 0.85 : 0.4;

    applyCardTone();
    host.setPalette(palette);
    composer.setPalette(palette);

    if (!running) renderFrame();
  };

  const setReducedMotion = (reduced: boolean) => {
    reducedMotion = reduced;
    if (reduced) {
      stop();
      smoothProgress = targetProgress;
      renderFrame();
    }
  };

  if (process.env.NODE_ENV !== "production") {
    // Lets the /landing-lab harness inspect what the scene actually holds.
    (window as unknown as { __illarinScene?: unknown }).__illarinScene = {
      camera,
      field,
      renderer,
      scene,
    };
  }

  return {
    dispose: () => {
      stop();
      field.dispose();
      host.dispose();
      composer.dispose();
      environment.dispose();
      atlas.dispose();
      invertedAtlas?.dispose();
      renderer.dispose();
    },
    resize,
    setPointer: (next) => {
      targetPointer = next;
    },
    setProgress: (progress) => {
      targetProgress = clamp(progress);
      if (reducedMotion) {
        smoothProgress = targetProgress;
        renderFrame();
      }
    },
    setReducedMotion,
    setTheme,
    start,
    stop,
  };
}
