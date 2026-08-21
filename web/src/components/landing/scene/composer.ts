import type { Camera, Scene, WebGLRenderer } from "three";
import { BokehPass } from "three/addons/postprocessing/BokehPass.js";
import { EffectComposer } from "three/addons/postprocessing/EffectComposer.js";
import { OutputPass } from "three/addons/postprocessing/OutputPass.js";
import { RenderPass } from "three/addons/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/addons/postprocessing/UnrealBloomPass.js";
import type { ScenePalette, ThreeModule } from "./types";

/** BokehPass types its uniforms as a bare object, so name the two used here. */
type BokehUniforms = {
  aperture: { value: number };
  focus: { value: number };
};

export type SceneComposer = {
  render: () => void;
  resize: (width: number, height: number) => void;
  setPalette: (palette: ScenePalette) => void;
  /** Pulls focus to a world distance from the camera. */
  focus: (distance: number, aperture: number) => void;
  dispose: () => void;
};

export function createComposer(
  THREE: ThreeModule,
  renderer: WebGLRenderer,
  scene: Scene,
  camera: Camera,
  palette: ScenePalette,
  depthOfField: boolean,
): SceneComposer {
  const composer = new EffectComposer(renderer);
  composer.addPass(new RenderPass(scene, camera));

  const bokeh = depthOfField
    ? new BokehPass(scene, camera, {
        aperture: 0.00035,
        focus: 7,
        maxblur: 0.012,
      })
    : null;
  if (bokeh) composer.addPass(bokeh);

  const bloom = new UnrealBloomPass(
    new THREE.Vector2(1, 1),
    palette.bloomStrength,
    0.62,
    palette.bloomThreshold,
  );
  composer.addPass(bloom);

  composer.addPass(new OutputPass());

  const resize = (width: number, height: number) => {
    composer.setSize(width, height);
    bloom.setSize(width, height);
  };

  const setPalette = (next: ScenePalette) => {
    bloom.strength = next.bloomStrength;
    bloom.threshold = next.bloomThreshold;
  };

  const focus = (distance: number, aperture: number) => {
    if (!bokeh) return;
    const uniforms = bokeh.uniforms as unknown as BokehUniforms;
    uniforms.focus.value = distance;
    uniforms.aperture.value = aperture;
  };

  return {
    dispose: () => {
      composer.dispose();
      bloom.dispose();
    },
    focus,
    render: () => composer.render(),
    resize,
    setPalette,
  };
}
