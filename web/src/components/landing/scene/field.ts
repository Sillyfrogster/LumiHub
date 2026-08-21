import type { InstancedMesh, Texture } from "three";
import type { ThreeModule } from "./types";

/**
 * The hub. A standing lattice of cards holds the centre, and more arrive out
 * of the dark from every direction until the lattice absorbs them.
 */
export type FieldRig = {
  mesh: InstancedMesh;
  setTexture: (texture: Texture) => void;
  setTone: (paper: number, envIntensity: number) => void;
  update: (time: number, gather: number) => void;
  dispose: () => void;
};

const SHELF_COLUMNS = 18;
const SHELF_ROWS = 7;
const SHELF_RINGS = 2;
const SHELF_COUNT = SHELF_COLUMNS * SHELF_ROWS * SHELF_RINGS;

const TRAFFIC_COUNT = 360;
const COUNT = SHELF_COUNT + TRAFFIC_COUNT;

const SHELF_RADIUS = 3.7;
const RING_GAP = 1.62;
const ROW_HEIGHT = 1.34;
const COLUMN_STEP = (Math.PI * 2) / SHELF_COLUMNS;

/** Arriving cards spawn beyond the fog and stop outside the lattice. */
const RADIUS_OUTER = 10.5;
const RADIUS_ARRIVE = SHELF_RADIUS + 1.3;
const SPREAD_Y = 5.4;

const CARD_WIDTH = 0.66;
const CARD_HEIGHT = 0.99;

const ATLAS_COLUMNS = 2;
const ATLAS_ROWS = 2;

const FLOW = 0.023;
const SWIRL = 1.5;

export function createField(THREE: ThreeModule, atlas: Texture): FieldRig {
  const geometry = new THREE.PlaneGeometry(CARD_WIDTH, CARD_HEIGHT);

  const material = new THREE.MeshStandardMaterial({
    alphaTest: 0.5,
    color: 0xffffff,
    envMapIntensity: 0.55,
    map: atlas,
    metalness: 0.08,
    roughness: 0.54,
    side: THREE.DoubleSide,
  });

  const cells = new Float32Array(COUNT * 2);
  const tones = new Float32Array(COUNT);

  // Give each card its own atlas cell and tone.
  material.onBeforeCompile = (shader) => {
    shader.vertexShader = shader.vertexShader
      .replace(
        "#include <common>",
        `#include <common>
         attribute vec2 aCell;
         attribute float aTone;
         varying vec2 vCell;
         varying float vTone;`,
      )
      .replace(
        "#include <uv_vertex>",
        `#include <uv_vertex>
         vCell = aCell;
         vTone = aTone;`,
      );

    shader.fragmentShader = shader.fragmentShader
      .replace(
        "#include <common>",
        `#include <common>
         varying vec2 vCell;
         varying float vTone;`,
      )
      .replace(
        "#include <map_fragment>",
        `vec2 cellUv = vMapUv * vec2(${(1 / ATLAS_COLUMNS).toFixed(6)}, ${(
          1 / ATLAS_ROWS
        ).toFixed(6)}) + vCell;
         vec4 sampledDiffuseColor = texture2D( map, cellUv );
         diffuseColor *= sampledDiffuseColor;
         diffuseColor.rgb *= vTone;`,
      );
  };

  const mesh = new THREE.InstancedMesh(geometry, material, COUNT);
  mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
  mesh.frustumCulled = false;

  // Fixed slots, so the centre reads as a structure and not a crowd.
  const shelfTheta = new Float32Array(SHELF_COUNT);
  const shelfY = new Float32Array(SHELF_COUNT);
  const shelfRadius = new Float32Array(SHELF_COUNT);

  for (let index = 0; index < SHELF_COUNT; index += 1) {
    const ring = index % SHELF_RINGS;
    const rest = Math.floor(index / SHELF_RINGS);
    const column = rest % SHELF_COLUMNS;
    const row = Math.floor(rest / SHELF_COLUMNS);

    const rowNorm = (row - (SHELF_ROWS - 1) / 2) / ((SHELF_ROWS - 1) / 2);
    shelfRadius[index] =
      (SHELF_RADIUS + ring * RING_GAP) * Math.cos(rowNorm * 0.5);
    shelfTheta[index] = column * COLUMN_STEP + ring * COLUMN_STEP * 0.5;
    shelfY[index] = rowNorm * ((SHELF_ROWS - 1) / 2) * ROW_HEIGHT;
  }

  // Everything still on its way in.
  const theta0 = new Float32Array(TRAFFIC_COUNT);
  const height0 = new Float32Array(TRAFFIC_COUNT);
  const phase = new Float32Array(TRAFFIC_COUNT);
  const rate = new Float32Array(TRAFFIC_COUNT);
  const tumbleX = new Float32Array(TRAFFIC_COUNT);
  const tumbleY = new Float32Array(TRAFFIC_COUNT);
  const tumbleZ = new Float32Array(TRAFFIC_COUNT);

  for (let index = 0; index < TRAFFIC_COUNT; index += 1) {
    theta0[index] = Math.random() * Math.PI * 2;
    height0[index] = (Math.random() * 2 - 1) * SPREAD_Y;
    phase[index] = Math.random();
    rate[index] = 0.7 + Math.random() * 0.62;
    tumbleX[index] = (Math.random() * 2 - 1) * 2.6;
    tumbleY[index] = (Math.random() * 2 - 1) * 3.2;
    tumbleZ[index] = (Math.random() * 2 - 1) * 2.8;
  }

  for (let index = 0; index < COUNT; index += 1) {
    const column = Math.floor(Math.random() * ATLAS_COLUMNS);
    const row = Math.floor(Math.random() * ATLAS_ROWS);
    cells[index * 2] = column / ATLAS_COLUMNS;
    cells[index * 2 + 1] = row / ATLAS_ROWS;
    tones[index] = 0.8 + Math.random() * 0.3;
  }

  geometry.setAttribute("aCell", new THREE.InstancedBufferAttribute(cells, 2));
  geometry.setAttribute("aTone", new THREE.InstancedBufferAttribute(tones, 1));

  const matrix = new THREE.Matrix4();
  const position = new THREE.Vector3();
  const quaternion = new THREE.Quaternion();
  const euler = new THREE.Euler();
  const scale = new THREE.Vector3();

  const update = (time: number, gather: number) => {
    for (let index = 0; index < SHELF_COUNT; index += 1) {
      const sway = Math.sin(time * 0.5 + index * 0.7) * 0.012;
      const theta = shelfTheta[index] + time * 0.017 + sway;
      const radius =
        shelfRadius[index] * (1 + Math.sin(time * 0.3 + index) * 0.004);

      position.set(
        Math.cos(theta) * radius,
        shelfY[index] + Math.sin(time * 0.42 + index * 1.3) * 0.018,
        Math.sin(theta) * radius,
      );
      euler.set(sway * 1.6, theta + Math.PI / 2, sway * 2.2, "XYZ");
      quaternion.setFromEuler(euler);
      scale.setScalar(gather);

      matrix.compose(position, quaternion, scale);
      mesh.setMatrixAt(index, matrix);
    }

    for (let index = 0; index < TRAFFIC_COUNT; index += 1) {
      let journey = (phase[index] + time * FLOW * rate[index]) % 1;
      if (journey < 0) journey += 1;

      const settle = 1 - (1 - journey) ** 3;
      const radius = RADIUS_OUTER + (RADIUS_ARRIVE - RADIUS_OUTER) * settle;

      const drifted = theta0[index] + settle * SWIRL + time * 0.017;
      const column = Math.round(drifted / COLUMN_STEP) * COLUMN_STEP;
      const theta = drifted + (column - drifted) * settle;

      const shelf = Math.round(height0[index] / ROW_HEIGHT) * ROW_HEIGHT;
      const y = height0[index] + (shelf * 0.42 - height0[index]) * settle;

      position.set(Math.cos(theta) * radius, y, Math.sin(theta) * radius);

      const chaos = 1 - settle;
      euler.set(
        tumbleX[index] * chaos,
        theta + Math.PI / 2 + tumbleY[index] * chaos,
        tumbleZ[index] * chaos,
        "XYZ",
      );
      quaternion.setFromEuler(euler);

      // Scale hides both ends of the loop.
      const arrive = 1 - Math.max(0, (journey - 0.86) / 0.14) ** 2;
      const enter = Math.min(1, journey / 0.07);
      scale.setScalar(arrive * enter * gather);

      matrix.compose(position, quaternion, scale);
      mesh.setMatrixAt(SHELF_COUNT + index, matrix);
    }

    mesh.instanceMatrix.needsUpdate = true;
  };

  return {
    dispose: () => {
      geometry.dispose();
      material.dispose();
    },
    mesh,
    setTexture: (texture: Texture) => {
      material.map = texture;
      material.needsUpdate = true;
    },
    setTone: (paper: number, envIntensity: number) => {
      material.color.set(paper);
      material.envMapIntensity = envIntensity;
    },
    update,
  };
}
