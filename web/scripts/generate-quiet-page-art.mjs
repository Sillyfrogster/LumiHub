// The artwork a detail page shows when the creator has displayed nothing, or
// close to nothing. One piece per kind, rendered twice. Light gets a dark
// object on a pearl ground and dark gets a lit object on a graphite ground.
//
// Every piece is the same idea, a chrome aperture standing on a plinth with
// the kind's own form waiting inside it, so five pieces read as one set. The
// forms come from the kind reliefs in generate-refractive-assets.mjs.
//
// Run it with `make quiet-page-art`.

import { createHash } from "node:crypto";
import { mkdir, readFile, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import sharp from "sharp";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(scriptDirectory, "..");
const repositoryRoot = path.resolve(webRoot, "..");
const artDirectory = path.join(webRoot, "src/assets/art/full");

const WIDTH = 1200;
const HEIGHT = 800;
const SUPERSAMPLE = 2;
const SEED = 0x1a11_a21;

/** Scene units. The frame spans x -100 to 100 and y -66 to 66, y down. */
const SCENE_HALF_WIDTH = 100;
const HORIZON_Y = 40;

const KINDS = ["character", "lorebook", "preset", "theme", "pack"];

/**
 * Light keeps the object darker than its ground so it reads on pearl. Dark
 * keeps every highlight well under white so a night reader is not flashed.
 */
const THEMES = {
  light: {
    skyTop: [0.973, 0.973, 0.965],
    skyBottom: [0.918, 0.922, 0.91],
    floorNear: [0.867, 0.871, 0.859],
    floorFar: [0.941, 0.945, 0.933],
    envHigh: [0.502, 0.51, 0.506],
    envLow: [0.145, 0.153, 0.165],
    specular: 0.32,
    specularTint: [0.2, 0.204, 0.212],
    rim: 0.12,
    glassBody: [0.596, 0.608, 0.616],
    glassBodyAlpha: 0.2,
    glassEdge: [0.208, 0.22, 0.235],
    shadow: 0.3,
    reflection: 0.16,
    ceilingSweep: 0.09,
  },
  dark: {
    skyTop: [0.086, 0.09, 0.098],
    skyBottom: [0.031, 0.031, 0.035],
    floorNear: [0.02, 0.02, 0.024],
    floorFar: [0.063, 0.067, 0.075],
    envHigh: [0.545, 0.553, 0.569],
    envLow: [0.055, 0.059, 0.067],
    specular: 0.26,
    specularTint: [0.741, 0.745, 0.757],
    rim: 0.3,
    glassBody: [0.271, 0.282, 0.298],
    glassBodyAlpha: 0.25,
    glassEdge: [0.741, 0.749, 0.765],
    shadow: 0.55,
    reflection: 0.24,
    ceilingSweep: 0.085,
  },
};

function clamp(value, minimum = 0, maximum = 1) {
  return Math.min(maximum, Math.max(minimum, value));
}

function mix(a, b, amount) {
  return a + (b - a) * amount;
}

function mixColor(a, b, amount) {
  return [
    mix(a[0], b[0], amount),
    mix(a[1], b[1], amount),
    mix(a[2], b[2], amount),
  ];
}

function smoothstep(edge0, edge1, value) {
  const progress = clamp((value - edge0) / (edge1 - edge0));
  return progress * progress * (3 - 2 * progress);
}

function byte(value) {
  return Math.round(clamp(value, 0, 1) * 255);
}

function hash2(x, y, seed = SEED) {
  let hash = Math.imul(x ^ seed, 0x45d9_f3b);
  hash = Math.imul(hash ^ (hash >>> 16) ^ y, 0x45d9_f3b);
  hash ^= hash >>> 16;
  return (hash >>> 0) / 4_294_967_295;
}

// Signed distances. Negative inside, in scene units.

function sdCircle(x, y, radius) {
  return Math.hypot(x, y) - radius;
}

function sdBox(x, y, halfWidth, halfHeight) {
  const dx = Math.abs(x) - halfWidth;
  const dy = Math.abs(y) - halfHeight;
  return (
    Math.hypot(Math.max(dx, 0), Math.max(dy, 0)) + Math.min(Math.max(dx, dy), 0)
  );
}

function sdRoundBox(x, y, halfWidth, halfHeight, radius) {
  return sdBox(x, y, halfWidth - radius, halfHeight - radius) - radius;
}

function sdSegment(x, y, ax, ay, bx, by, thickness) {
  const dx = bx - ax;
  const dy = by - ay;
  const lengthSquared = dx * dx + dy * dy;
  const projection =
    lengthSquared === 0
      ? 0
      : clamp(((x - ax) * dx + (y - ay) * dy) / lengthSquared);
  return (
    Math.hypot(x - (ax + dx * projection), y - (ay + dy * projection)) -
    thickness
  );
}

function sdRing(x, y, radius, thickness) {
  return Math.abs(Math.hypot(x, y) - radius) - thickness;
}

function shell(distance, thickness) {
  return Math.abs(distance) - thickness;
}

function union(...distances) {
  return Math.min(...distances);
}

function intersect(...distances) {
  return Math.max(...distances);
}

/** Distance to a polygon, negative inside, whichever way it is wound. */
function sdPolygon(x, y, points) {
  let distance = Number.POSITIVE_INFINITY;
  let inside = false;
  for (let index = 0; index < points.length; index += 1) {
    const [ax, ay] = points[index];
    const [bx, by] = points[(index + 1) % points.length];
    distance = Math.min(distance, sdSegment(x, y, ax, ay, bx, by, 0));
    if (ay > y !== by > y && x < ((bx - ax) * (y - ay)) / (by - ay) + ax) {
      inside = !inside;
    }
  }
  return inside ? -distance : distance;
}

/** A round-topped arch, the aperture every piece is built around. */
function sdArch(x, y, halfWidth, footY, headY) {
  const springY = headY + halfWidth;
  const column = sdBox(
    x,
    y - (springY + footY) / 2,
    halfWidth,
    (footY - springY) / 2,
  );
  const head = intersect(sdCircle(x, y - springY, halfWidth), y - springY);
  return union(column, head);
}

// The five kind forms. Each returns a list of solids drawn back to front.
//
// A solid is { sdf, material, bevel, depth }. `depth` shades a form that
// stands further back so the aperture keeps its recession.

function chrome(sdf, options = {}) {
  return { sdf, material: "chrome", bevel: 1.5, depth: 0, ...options };
}

function glass(sdf, options = {}) {
  return { sdf, material: "glass", bevel: 1.1, depth: 0, ...options };
}

/**
 * The plinth, the aperture and the plates every kind stands in, with a far
 * skyline seen through the opening. The aperture runs off the top of the
 * frame, so the piece reads as a room the reader is standing in rather than an
 * object on a white sweep.
 */
function apertureStage({ halfWidth, headY, centerX, plates, skyline }) {
  const footY = HORIZON_Y - 2;
  const interior = (x, y) =>
    sdArch(x - centerX, y, halfWidth - 2.4, footY - 1, headY + 1.8);
  const solids = [];

  for (const [offsetX, top, width, depth, lean] of plates) {
    solids.push(
      glass(
        (x, y) => {
          const skew = (y - footY) * lean;
          return sdBox(
            x - offsetX - skew,
            y - (footY + top) / 2,
            width,
            (footY - top) / 2,
          );
        },
        { depth, bevel: 0.85 },
      ),
    );
  }

  // A far city of glass, only ever visible through the opening.
  for (const [offsetX, top, width, depth] of skyline) {
    solids.push(
      glass(
        (x, y) =>
          intersect(
            sdBox(
              x - centerX - offsetX,
              y - (footY + top) / 2,
              width,
              (footY - top) / 2,
            ),
            interior(x, y),
          ),
        { depth, bevel: 0.7 },
      ),
    );
  }

  solids.push(
    chrome(
      (x, y) => shell(sdArch(x - centerX, y, halfWidth, footY, headY), 1.7),
      { bevel: 1.5 },
    ),
  );
  // A thinner line inside the frame gives the opening its depth.
  solids.push(
    chrome((x, y) => shell(interior(x, y), 0.42), {
      bevel: 0.45,
      depth: 0.22,
    }),
  );
  solids.push(glass(interior, { depth: 0.24, bevel: 1.2 }));

  return { solids, footY, centerX };
}

/** One long slab, running off both edges, that everything stands on. */
function plinth() {
  return chrome((x, y) => sdRoundBox(x, y - (HORIZON_Y + 2.6), 132, 4.6, 1.1), {
    bevel: 1.3,
  });
}

/** A bust waiting on its stand, head and shoulders cut in clear material. */
function characterForm(footY, centerX) {
  const baseY = footY - 5;
  const bust = (x, y) =>
    union(
      sdCircle(x - centerX, y - (baseY - 28.5), 9.2),
      sdPolygon(x - centerX, y, [
        [-15.5, baseY],
        [-10.5, baseY - 21],
        [10.5, baseY - 21],
        [15.5, baseY],
      ]),
    );
  return [
    glass(bust, { bevel: 1.3 }),
    chrome((x, y) => shell(bust(x, y), 0.42), { bevel: 0.45, depth: 0.06 }),
    chrome((x, y) => sdRoundBox(x - centerX, y - (baseY + 1.6), 17, 1.5, 0.6), {
      bevel: 0.7,
    }),
  ];
}

function lorebookForm(footY, centerX) {
  const centerY = footY - 27;
  const leaf = (side) => (x, y) =>
    sdPolygon(x - centerX, y - centerY, [
      [0, -11],
      [side * 21, -15],
      [side * 20, 10],
      [0, 11],
    ]);
  return [
    glass(leaf(-1), { bevel: 0.9, depth: 0.14 }),
    glass(leaf(1), { bevel: 0.9, depth: 0.14 }),
    chrome(
      (x, y) =>
        union(
          sdSegment(x - centerX, y, 0, centerY - 12, 0, centerY + 12, 0.7),
          sdSegment(x - centerX, y, -21, centerY + 13, 21, centerY + 13, 0.6),
          sdSegment(x - centerX, y, -13, centerY + 13, -9, footY - 2, 0.6),
          sdSegment(x - centerX, y, 13, centerY + 13, 9, footY - 2, 0.6),
        ),
      { bevel: 0.7 },
    ),
  ];
}

function presetForm(footY, centerX) {
  const rows = [-42, -30, -18, -6];
  const knobs = [-11, 13, -3, 16];
  const solids = [];
  for (let index = 0; index < rows.length; index += 1) {
    const railY = footY + rows[index];
    solids.push(
      chrome((x, y) => sdSegment(x - centerX, y, -22, railY, 22, railY, 0.45), {
        bevel: 0.45,
        depth: 0.18,
      }),
    );
    solids.push(
      glass(
        (x, y) =>
          sdRoundBox(x - centerX - knobs[index], y - railY, 3.6, 5.4, 1),
        { bevel: 0.8 },
      ),
    );
  }
  solids.push(
    chrome(
      (x, y) =>
        union(
          sdSegment(x - centerX, y, -24, footY - 46, -24, footY - 2, 0.65),
          sdSegment(x - centerX, y, 24, footY - 46, 24, footY - 2, 0.65),
        ),
      { bevel: 0.65, depth: 0.1 },
    ),
  );
  return solids;
}

function themeForm(footY, centerX) {
  const centerY = footY - 30;
  const solids = [
    glass((x, y) => sdCircle(x - centerX, y - centerY, 19), {
      bevel: 1.5,
      depth: 0.1,
    }),
    chrome((x, y) => sdRing(x - centerX, y - centerY, 19, 0.75), {
      bevel: 0.7,
    }),
  ];
  for (let index = 0; index < 5; index += 1) {
    const from = (index / 5) * Math.PI * 2 - Math.PI / 2;
    solids.push(
      chrome(
        (x, y) =>
          sdSegment(
            x - centerX,
            y - centerY,
            0,
            0,
            Math.cos(from) * 18.4,
            Math.sin(from) * 18.4,
            0.45,
          ),
        { bevel: 0.45, depth: 0.16 },
      ),
    );
  }
  solids.push(
    chrome((x, y) => sdCircle(x - centerX, y - centerY, 2.6), { bevel: 0.7 }),
  );
  solids.push(
    chrome(
      (x, y) => sdSegment(x - centerX, y, 0, centerY + 19, 0, footY - 2, 0.65),
      { bevel: 0.65, depth: 0.12 },
    ),
  );
  return solids;
}

function packForm(footY, centerX) {
  const cases = [
    [-14, -14, 12.5, 9.5, 0.24],
    [0, -25, 14, 10.5, 0.1],
    [14, -36, 12.5, 9.5, 0],
  ];
  const solids = [];
  for (const [offsetX, offsetY, halfWidth, halfHeight, depth] of cases) {
    solids.push(
      glass(
        (x, y) =>
          sdRoundBox(
            x - centerX - offsetX,
            y - (footY + offsetY),
            halfWidth,
            halfHeight,
            1.4,
          ),
        { bevel: 1, depth: depth + 0.12 },
      ),
    );
    solids.push(
      chrome(
        (x, y) =>
          shell(
            sdRoundBox(
              x - centerX - offsetX,
              y - (footY + offsetY),
              halfWidth,
              halfHeight,
              1.4,
            ),
            0.42,
          ),
        { bevel: 0.5, depth },
      ),
    );
  }
  return solids;
}

// Each kind gets its own aperture proportion, its own place in the frame and
// its own far skyline, so the five read as five rooms rather than one recolour.
// Plates and towers are [offset, top, halfWidth, depth, lean].
const SCENES = {
  character: {
    aperture: {
      halfWidth: 26,
      headY: -66,
      centerX: 14,
      plates: [
        [-52, -44, 14, 0.5, 0.1],
        [-16, -30, 9, 0.58, 0.05],
        [66, -54, 15, 0.46, -0.06],
      ],
      skyline: [
        [-17, -18, 4.5, 0.62],
        [-8, -34, 3, 0.7],
        [2, -12, 5.5, 0.58],
        [13, -27, 4, 0.66],
        [21, -8, 3.5, 0.74],
      ],
    },
    form: characterForm,
  },
  lorebook: {
    aperture: {
      halfWidth: 33,
      headY: -58,
      centerX: 10,
      plates: [
        [-58, -36, 17, 0.46, 0.08],
        [-24, -52, 11, 0.54, 0.03],
        [70, -30, 13, 0.5, -0.05],
      ],
      skyline: [
        [-24, -10, 5, 0.66],
        [-13, -24, 3.5, 0.6],
        [-2, -6, 6, 0.72],
        [10, -20, 4, 0.62],
        [21, -30, 3, 0.68],
        [28, -9, 4.5, 0.74],
      ],
    },
    form: lorebookForm,
  },
  preset: {
    aperture: {
      halfWidth: 28,
      headY: -70,
      centerX: 16,
      plates: [
        [-54, -50, 13, 0.52, 0.06],
        [-18, -28, 10, 0.6, 0.04],
        [64, -60, 14, 0.48, -0.07],
      ],
      skyline: [
        [-19, -14, 4, 0.64],
        [-9, -38, 3, 0.58],
        [1, -22, 5, 0.7],
        [12, -10, 4, 0.72],
        [20, -30, 3, 0.62],
      ],
    },
    form: presetForm,
  },
  theme: {
    aperture: {
      halfWidth: 29,
      headY: -62,
      centerX: 12,
      plates: [
        [-56, -32, 15, 0.48, 0.09],
        [-20, -48, 10, 0.56, 0.04],
        [68, -40, 12, 0.5, -0.06],
      ],
      skyline: [
        [-20, -8, 5, 0.7],
        [-10, -28, 3.5, 0.6],
        [1, -16, 4.5, 0.66],
        [12, -34, 3, 0.58],
        [21, -11, 4, 0.72],
      ],
    },
    form: themeForm,
  },
  pack: {
    aperture: {
      halfWidth: 33,
      headY: -56,
      centerX: 9,
      plates: [
        [-60, -42, 16, 0.5, 0.07],
        [-26, -26, 11, 0.6, 0.03],
        [72, -52, 12, 0.48, -0.05],
      ],
      skyline: [
        [-24, -12, 5.5, 0.68],
        [-12, -30, 3.5, 0.58],
        [0, -18, 4.5, 0.64],
        [12, -8, 5, 0.74],
        [23, -26, 3.5, 0.62],
      ],
    },
    form: packForm,
  },
};

function sceneSolids(kind) {
  const scene = SCENES[kind];
  const stage = apertureStage(scene.aperture);
  return [plinth(), ...stage.solids, ...scene.form(stage.footY, stage.centerX)];
}

/** The studio behind the object. A soft sky, a floor, and one wall sweep. */
function groundColor(x, y, theme) {
  const height = clamp((y + 66) / 132);
  if (y < HORIZON_Y) {
    const sky = mixColor(
      theme.skyTop,
      theme.skyBottom,
      smoothstep(0, 1, height),
    );
    const sweep =
      theme.ceilingSweep *
      smoothstep(60, -10, Math.hypot((x + 42) * 0.72, y + 54));
    return mixColor(sky, theme.envHigh, sweep);
  }
  const distance = smoothstep(0, 26, y - HORIZON_Y);
  return mixColor(theme.floorFar, theme.floorNear, distance);
}

/** A metal surface. An environment sample plus one soft key highlight. */
function chromeColor(normalX, normalY, normalZ, theme, depth) {
  const up = clamp(0.5 - normalY * 0.5 + normalZ * 0.16);
  const base = mixColor(theme.envLow, theme.envHigh, up ** 1.35);

  const lightX = -0.52;
  const lightY = -0.66;
  const lightZ = 0.54;
  const facing = clamp(normalX * lightX + normalY * lightY + normalZ * lightZ);
  const specular = theme.specular * facing ** 22;
  const sheen = 0.22 * facing ** 3;

  const rim = theme.rim * (1 - normalZ) ** 2.2;
  const lit = mixColor(base, theme.specularTint, clamp(specular + sheen + rim));
  return mixColor(lit, theme.envLow, depth * 0.55);
}

/** Clear material. A faint body, a bright edge, nothing in the middle. */
function glassColor(normalZ, theme, depth) {
  const edge = (1 - normalZ) ** 1.5;
  const body = mixColor(theme.glassBody, theme.envLow, depth * 0.5);
  const color = mixColor(body, theme.glassEdge, clamp(edge * 1.25));
  const alpha = clamp(theme.glassBodyAlpha + edge * 0.82);
  return [color, alpha];
}

function shadeSolid(solid, x, y, theme, scale) {
  const distance = solid.sdf(x, y);
  const antialias = 1.1 / scale;
  const coverage = 1 - smoothstep(-antialias, antialias, distance);
  if (coverage <= 0.002) return null;

  const step = 0.55;
  const gradientX = solid.sdf(x + step, y) - solid.sdf(x - step, y);
  const gradientY = solid.sdf(x, y + step) - solid.sdf(x, y - step);
  const gradientLength = Math.hypot(gradientX, gradientY) || 1;
  const bevel = 1 - smoothstep(0, solid.bevel, -distance);
  const slope = bevel * 0.94;
  const normalX = (gradientX / gradientLength) * slope;
  const normalY = (gradientY / gradientLength) * slope;
  const normalZ = Math.sqrt(
    Math.max(0, 1 - normalX * normalX - normalY * normalY),
  );

  if (solid.material === "glass") {
    const [color, alpha] = glassColor(normalZ, theme, solid.depth);
    return [color, alpha * coverage];
  }
  const color = chromeColor(normalX, normalY, normalZ, theme, solid.depth);
  return [color, coverage];
}

function renderScene(kind, themeName) {
  const theme = THEMES[themeName];
  const solids = sceneSolids(kind);
  const width = WIDTH * SUPERSAMPLE;
  const height = HEIGHT * SUPERSAMPLE;
  const scale = width / (SCENE_HALF_WIDTH * 2);
  const data = Buffer.alloc(width * height * 3);

  for (let pixelY = 0; pixelY < height; pixelY += 1) {
    const y = (pixelY - (height - 1) / 2) / scale;
    for (let pixelX = 0; pixelX < width; pixelX += 1) {
      const x = (pixelX - (width - 1) / 2) / scale;

      let color = groundColor(x, y, theme);

      // Contact shadow, then the floor reflection, then the object itself.
      if (y > HORIZON_Y) {
        const shadow =
          theme.shadow *
          smoothstep(1, 0, Math.hypot((x - 2) / 78, (y - HORIZON_Y - 3) / 13));
        color = mixColor(color, theme.envLow, shadow * 0.5);

        const mirroredY = HORIZON_Y * 2 - y + 3;
        const fade = theme.reflection * smoothstep(26, 0, y - HORIZON_Y);
        if (fade > 0.004) {
          for (const solid of solids) {
            const shaded = shadeSolid(solid, x, mirroredY, theme, scale);
            if (!shaded) continue;
            color = mixColor(color, shaded[0], shaded[1] * fade);
          }
        }
      }

      for (const solid of solids) {
        const shaded = shadeSolid(solid, x, y, theme, scale);
        if (!shaded) continue;
        color = mixColor(color, shaded[0], shaded[1]);
      }

      // A trace of grain so the wide gradients do not band.
      const grain =
        (hash2(pixelX >> 1, pixelY >> 1, SEED ^ 0x2f1b) - 0.5) * 0.006;

      const offset = (pixelY * width + pixelX) * 3;
      data[offset] = byte(color[0] + grain);
      data[offset + 1] = byte(color[1] + grain);
      data[offset + 2] = byte(color[2] + grain);
    }
  }

  return sharp(data, { raw: { width, height, channels: 3 } })
    .resize(WIDTH, HEIGHT, { kernel: "lanczos3" })
    .webp({ quality: 86, effort: 6, smartSubsample: true });
}

function outputPath(kind, themeName) {
  return path.join(
    artDirectory,
    `illarin-quiet-page-${kind}-${themeName}-v1.webp`,
  );
}

async function inspectOutput(filePath) {
  const metadata = await sharp(filePath).metadata();
  const fileStat = await stat(filePath);
  const digest = createHash("sha256")
    .update(await readFile(filePath))
    .digest("hex");

  if (
    metadata.width !== WIDTH ||
    metadata.height !== HEIGHT ||
    metadata.format !== "webp"
  ) {
    throw new Error(
      `${path.basename(filePath)} has unexpected metadata: ${JSON.stringify(metadata)}`,
    );
  }

  return {
    file: path.relative(repositoryRoot, filePath),
    bytes: fileStat.size,
    sha256: digest,
  };
}

async function main() {
  await mkdir(artDirectory, { recursive: true });

  // `bun scripts/generate-quiet-page-art.mjs theme` renders one kind while the
  // composition is still being worked out.
  const only = process.argv.slice(2).filter((kind) => KINDS.includes(kind));
  const kinds = only.length > 0 ? only : KINDS;

  const jobs = [];
  for (const kind of kinds) {
    for (const themeName of Object.keys(THEMES)) {
      const target = outputPath(kind, themeName);
      jobs.push(
        renderScene(kind, themeName)
          .toFile(target)
          .then(() => target),
      );
    }
  }

  const written = await Promise.all(jobs);
  const inspections = await Promise.all(written.map(inspectOutput));
  process.stdout.write(
    `${JSON.stringify({ seed: SEED, outputs: inspections }, null, 2)}\n`,
  );
}

await main();
