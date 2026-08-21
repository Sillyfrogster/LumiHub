import type { Group, ShaderMaterial, Texture, Vector3 } from "three";
import type { ScenePalette, ThreeModule } from "./types";

/**
 * The host, standing at the near edge of the hub. A depth map gives the flat
 * art parallax and a surface normal, so the scene's light shades her.
 */
export type HostRig = {
  group: Group;
  setPalette: (palette: ScenePalette) => void;
  /** Takes the camera position in the host's own object space. */
  update: (time: number, presence: number, cameraLocal: Vector3) => void;
  dispose: () => void;
};

const ASPECT = 730 / 1022;
const HEIGHT = 7.4;

const HOST_VERTEX = /* glsl */ `
  uniform vec3 uCameraLocal;
  varying vec2 vUv;
  varying vec3 vView;

  void main() {
    vUv = uv;
    vView = normalize(uCameraLocal - position);
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
  }
`;

const HOST_FRAGMENT = /* glsl */ `
  uniform sampler2D uMap;
  uniform sampler2D uDepth;
  uniform vec3 uKey;
  uniform vec3 uAmbient;
  uniform vec3 uRim;
  uniform vec3 uLightDir;
  uniform float uParallax;
  uniform float uRelief;
  uniform float uOpacity;
  varying vec2 vUv;
  varying vec3 vView;

  vec3 toLinear(vec3 c) {
    return mix(c / 12.92, pow((c + 0.055) / 1.055, vec3(2.4)), step(vec3(0.04045), c));
  }

  void main() {
    // Near parts track across far parts as the camera moves.
    float centre = texture2D(uDepth, vUv).r;
    vec2 offset = (vView.xy / max(abs(vView.z), 0.35)) * (centre - 0.62) * uParallax;
    vec2 uv = vUv + offset;

    vec4 texel = texture2D(uMap, uv);
    if (texel.a < 0.02) discard;

    // Surface normal from the depth gradient.
    float e = 0.004;
    float dx = texture2D(uDepth, uv + vec2(e, 0.0)).r - texture2D(uDepth, uv - vec2(e, 0.0)).r;
    float dy = texture2D(uDepth, uv + vec2(0.0, e)).r - texture2D(uDepth, uv - vec2(0.0, e)).r;
    vec3 normal = normalize(vec3(-dx * uRelief, -dy * uRelief, 1.0));

    float ndl = max(dot(normal, normalize(uLightDir)), 0.0);
    vec3 shade = uAmbient + uKey * ndl;

    float rim = pow(1.0 - max(dot(normal, vec3(0.0, 0.0, 1.0)), 0.0), 2.2);

    vec3 colour = toLinear(texel.rgb) * shade + uRim * rim * 0.16;
    gl_FragColor = vec4(colour, texel.a * uOpacity);
  }
`;

export function createHost(
  THREE: ThreeModule,
  mapUrl: string,
  depthUrl: string,
  palette: ScenePalette,
  onFailure: () => void,
): HostRig {
  const group = new THREE.Group();

  const uniforms = {
    uAmbient: { value: new THREE.Color(palette.hostShadow) },
    uCameraLocal: { value: new THREE.Vector3(0, 0, 8) },
    uDepth: { value: null as Texture | null },
    uKey: { value: new THREE.Color(palette.hostKey) },
    uLightDir: { value: new THREE.Vector3(0.55, 0.5, 0.66) },
    uMap: { value: null as Texture | null },
    uOpacity: { value: 0 },
    uParallax: { value: 0.055 },
    uRelief: { value: 5.5 },
    uRim: { value: new THREE.Color(0xbfe4e6) },
  };

  const material = new THREE.ShaderMaterial({
    depthWrite: false,
    fragmentShader: HOST_FRAGMENT,
    transparent: true,
    uniforms,
    vertexShader: HOST_VERTEX,
  });

  const plane = new THREE.Mesh(
    new THREE.PlaneGeometry(HEIGHT * ASPECT, HEIGHT),
    material,
  );
  group.add(plane);

  const loader = new THREE.TextureLoader();
  let ready = 0;
  const textures: Texture[] = [];

  const receive = (slot: "uMap" | "uDepth") => (texture: Texture) => {
    texture.colorSpace = THREE.NoColorSpace;
    texture.anisotropy = 8;
    uniforms[slot].value = texture;
    textures.push(texture);
    ready += 1;
  };

  loader.load(mapUrl, receive("uMap"), undefined, onFailure);
  loader.load(depthUrl, receive("uDepth"), undefined, onFailure);

  const setPalette = (next: ScenePalette) => {
    uniforms.uKey.value.set(next.hostKey);
    uniforms.uAmbient.value.set(next.hostShadow);
  };

  const update = (time: number, presence: number, cameraLocal: Vector3) => {
    uniforms.uCameraLocal.value.copy(cameraLocal);
    uniforms.uOpacity.value = ready === 2 ? presence : 0;
    void time;
  };

  return {
    dispose: () => {
      plane.geometry.dispose();
      (material as ShaderMaterial).dispose();
      for (const texture of textures) texture.dispose();
    },
    group,
    setPalette,
    update,
  };
}
