import type { Texture, WebGLRenderer } from "three";
import type { SceneTheme, ThreeModule } from "./types";

type LightPanel = {
  position: [number, number, number];
  rotation: [number, number, number];
  scale: [number, number];
  color: number;
  intensity: number;
};

/** A studio built as geometry, so cards reflect something real. */
const PANELS: LightPanel[] = [
  // Broad key panel, matching the scene key light.
  {
    position: [5.4, 5.6, 3.4],
    rotation: [0, -0.86, 0.24],
    scale: [7, 9],
    color: 0xffffff,
    intensity: 5.4,
  },
  // Cold strip that gives every card edge its highlight.
  {
    position: [-6.6, 1.4, 1.2],
    rotation: [0, 1.24, 0],
    scale: [0.7, 12],
    color: 0xd6f2f4,
    intensity: 8.2,
  },
  // Low bounce, so undersides are not dead black.
  {
    position: [-1.4, -6.2, 2.6],
    rotation: [1.34, 0, 0],
    scale: [9, 5],
    color: 0x2f7f86,
    intensity: 2.1,
  },
  // Ceiling wash.
  {
    position: [0, 7.4, -1],
    rotation: [Math.PI / 2, 0, 0],
    scale: [8, 8],
    color: 0xeef7f9,
    intensity: 1.5,
  },
];

export function createStudioEnvironment(
  THREE: ThreeModule,
  renderer: WebGLRenderer,
  theme: SceneTheme,
): Texture {
  const room = new THREE.Scene();
  const dark = theme === "dark";

  const shell = new THREE.Mesh(
    new THREE.BoxGeometry(24, 24, 24),
    new THREE.MeshBasicMaterial({
      color: dark ? 0x05080b : 0x9fb2b8,
      side: THREE.BackSide,
    }),
  );
  room.add(shell);

  for (const panel of PANELS) {
    const material = new THREE.MeshBasicMaterial({ color: panel.color });
    material.color.multiplyScalar(panel.intensity * (dark ? 1 : 0.72));

    const light = new THREE.Mesh(new THREE.PlaneGeometry(1, 1), material);
    light.position.set(...panel.position);
    light.rotation.set(...panel.rotation);
    light.scale.set(panel.scale[0], panel.scale[1], 1);
    room.add(light);
  }

  const generator = new THREE.PMREMGenerator(renderer);
  generator.compileEquirectangularShader();
  const target = generator.fromScene(room, 0.028);

  generator.dispose();
  shell.geometry.dispose();
  (shell.material as { dispose: () => void }).dispose();
  for (const child of room.children) {
    if (child === shell) continue;
    const mesh = child as unknown as {
      geometry?: { dispose: () => void };
      material?: { dispose: () => void };
    };
    mesh.geometry?.dispose();
    mesh.material?.dispose();
  }

  return target.texture;
}
