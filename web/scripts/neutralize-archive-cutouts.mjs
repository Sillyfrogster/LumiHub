import { createHash } from "node:crypto";
import { mkdir, readFile, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import sharp from "sharp";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(scriptDirectory, "..");
const repositoryRoot = path.resolve(webRoot, "..");
const sourceDirectory = path.join(
  repositoryRoot,
  ".ai/issues/refractive-identity/assets",
);
const productionDirectory = path.join(webRoot, "src/assets/art/full");
const reviewPath = path.join(
  sourceDirectory,
  "archive-neutral-edge-color-check-v1.webp",
);

const WIDTH = 952;
const HEIGHT = 1652;

const variants = [
  {
    theme: "dark",
    source: path.join(
      sourceDirectory,
      "illarin-mascot-archive-dark-cutout-v1.png",
    ),
    output: path.join(
      productionDirectory,
      "illarin-mascot-archive-dark-v1.webp",
    ),
    matte: { r: 5, g: 7, b: 9 },
  },
  {
    theme: "light",
    source: path.join(
      sourceDirectory,
      "illarin-mascot-archive-light-cutout-v1.png",
    ),
    output: path.join(
      productionDirectory,
      "illarin-mascot-archive-light-v1.webp",
    ),
    matte: { r: 244, g: 246, b: 247 },
  },
];

function clamp(value, minimum = 0, maximum = 1) {
  return Math.min(maximum, Math.max(minimum, value));
}

function smoothstep(edge0, edge1, value) {
  const progress = clamp((value - edge0) / (edge1 - edge0));
  return progress * progress * (3 - 2 * progress);
}

function rgbToHsv(red, green, blue) {
  const r = red / 255;
  const g = green / 255;
  const b = blue / 255;
  const maximum = Math.max(r, g, b);
  const minimum = Math.min(r, g, b);
  const delta = maximum - minimum;
  let hue = 0;

  if (delta > 0) {
    if (maximum === r) hue = 60 * (((g - b) / delta) % 6);
    else if (maximum === g) hue = 60 * ((b - r) / delta + 2);
    else hue = 60 * ((r - g) / delta + 4);
  }

  if (hue < 0) hue += 360;
  return {
    hue,
    saturation: maximum === 0 ? 0 : delta / maximum,
    value: maximum,
    chroma: delta * 255,
  };
}

function inTargetMaterial(x, y) {
  const heldPane = x <= 550 && y >= 600 && y <= 960;
  const leftCoatGlass = x <= 415 && y >= 850;
  const rightCoatGlass = x >= 555 && y >= 820;
  const upperRearGlass = x >= 500 && x <= 760 && y >= 470 && y <= 930;
  return heldPane || leftCoatGlass || rightCoatGlass || upperRearGlass;
}

function nearTransparentEdge(data, x, y) {
  const radius = 4;
  const points = [
    [x - radius, y],
    [x + radius, y],
    [x, y - radius],
    [x, y + radius],
  ];
  const alpha = data[(y * WIDTH + x) * 4 + 3];
  if (alpha < 245) return true;

  return points.some(([sampleX, sampleY]) => {
    if (sampleX < 0 || sampleX >= WIDTH || sampleY < 0 || sampleY >= HEIGHT) {
      return true;
    }
    return data[(sampleY * WIDTH + sampleX) * 4 + 3] < 24;
  });
}

function neutralizationWeight(red, green, blue, material) {
  const { hue, saturation, value, chroma } = rgbToHsv(red, green, blue);
  const coolHue = smoothstep(112, 145, hue) * (1 - smoothstep(255, 282, hue));
  const saturationWeight = material
    ? smoothstep(0.035, 0.19, saturation)
    : smoothstep(0.1, 0.3, saturation);
  const chromaWeight = material
    ? smoothstep(5, 28, chroma)
    : smoothstep(10, 34, chroma);
  const highlightProtection =
    1 - smoothstep(material ? 0.94 : 0.74, material ? 1 : 0.97, value);
  return (
    coolHue *
    Math.max(saturationWeight, chromaWeight) *
    highlightProtection *
    0.99
  );
}

function chromaticPixel(red, green, blue) {
  const { hue, saturation } = rgbToHsv(red, green, blue);
  return hue >= 150 && hue <= 270 && saturation >= 0.055;
}

async function neutralizeVariant(variant) {
  const source = await sharp(variant.source)
    .ensureAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true });

  if (
    source.info.width !== WIDTH ||
    source.info.height !== HEIGHT ||
    source.info.channels !== 4
  ) {
    throw new Error(
      `${path.basename(variant.source)} must be ${WIDTH}x${HEIGHT} RGBA.`,
    );
  }

  const processed = Buffer.from(source.data);
  const changedMask = Buffer.alloc(WIDTH * HEIGHT);
  let pixelsChanged = 0;
  let chromaticBefore = 0;
  let chromaBefore = 0;

  for (let y = 0; y < HEIGHT; y += 1) {
    for (let x = 0; x < WIDTH; x += 1) {
      const offset = (y * WIDTH + x) * 4;
      const alpha = source.data[offset + 3];
      if (alpha === 0) continue;

      const material = inTargetMaterial(x, y);
      const halo = nearTransparentEdge(source.data, x, y);
      if (!material && !halo) continue;

      const red = source.data[offset];
      const green = source.data[offset + 1];
      const blue = source.data[offset + 2];
      const weight = neutralizationWeight(red, green, blue, material);
      if (weight <= 0) continue;

      const hsv = rgbToHsv(red, green, blue);
      if (chromaticPixel(red, green, blue)) chromaticBefore += 1;
      chromaBefore += hsv.chroma;

      const neutral = red * 0.2126 + green * 0.7152 + blue * 0.0722;
      processed[offset] = Math.round(red + (neutral - red) * weight);
      processed[offset + 1] = Math.round(green + (neutral - green) * weight);
      processed[offset + 2] = Math.round(blue + (neutral - blue) * weight);
      changedMask[y * WIDTH + x] = 1;
      pixelsChanged += 1;
    }
  }

  for (let offset = 3; offset < processed.length; offset += 4) {
    if (processed[offset] !== source.data[offset]) {
      throw new Error(`${variant.theme} alpha changed before encoding.`);
    }
  }

  await sharp(processed, {
    raw: { width: WIDTH, height: HEIGHT, channels: 4 },
  })
    .webp({ quality: 88, alphaQuality: 100, effort: 6, smartSubsample: true })
    .toFile(variant.output);

  const decoded = await sharp(variant.output)
    .ensureAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true });
  let alphaMismatches = 0;
  let chromaticAfter = 0;
  let chromaAfter = 0;

  for (let y = 0; y < HEIGHT; y += 1) {
    for (let x = 0; x < WIDTH; x += 1) {
      const offset = (y * WIDTH + x) * 4;
      if (decoded.data[offset + 3] !== source.data[offset + 3])
        alphaMismatches += 1;
      if (changedMask[y * WIDTH + x] === 0) continue;

      const red = decoded.data[offset];
      const green = decoded.data[offset + 1];
      const blue = decoded.data[offset + 2];
      if (chromaticPixel(red, green, blue)) chromaticAfter += 1;
      chromaAfter += rgbToHsv(red, green, blue).chroma;
    }
  }

  if (alphaMismatches !== 0) {
    throw new Error(
      `${variant.theme} WebP changed ${alphaMismatches} alpha samples.`,
    );
  }

  const bytes = (await stat(variant.output)).size;
  const digest = createHash("sha256")
    .update(await readFile(variant.output))
    .digest("hex");

  return {
    theme: variant.theme,
    file: path.relative(repositoryRoot, variant.output),
    width: decoded.info.width,
    height: decoded.info.height,
    channels: decoded.info.channels,
    bytes,
    sha256: digest,
    pixelsChanged,
    alphaMismatches,
    chromaticPixelsBefore: chromaticBefore,
    chromaticPixelsAfter: chromaticAfter,
    meanTargetChromaBefore: Number((chromaBefore / pixelsChanged).toFixed(2)),
    meanTargetChromaAfter: Number((chromaAfter / pixelsChanged).toFixed(2)),
  };
}

async function generateReviewSheet() {
  const gutter = 16;
  const width = WIDTH * 2 + gutter;

  const mattes = await Promise.all(
    variants.map((variant) =>
      sharp({
        create: {
          width: WIDTH,
          height: HEIGHT,
          channels: 3,
          background: variant.matte,
        },
      })
        .composite([{ input: variant.output, left: 0, top: 0 }])
        .webp({ quality: 88, effort: 5 })
        .toBuffer(),
    ),
  );

  await sharp({
    create: {
      width,
      height: HEIGHT,
      channels: 3,
      background: { r: 86, g: 88, b: 90 },
    },
  })
    .composite([
      { input: mattes[0], left: 0, top: 0 },
      { input: mattes[1], left: WIDTH + gutter, top: 0 },
    ])
    .webp({ quality: 84, effort: 6, smartSubsample: true })
    .toFile(reviewPath);
}

async function main() {
  await mkdir(productionDirectory, { recursive: true });
  const results = await Promise.all(variants.map(neutralizeVariant));
  await generateReviewSheet();
  process.stdout.write(`${JSON.stringify({ outputs: results }, null, 2)}\n`);
}

await main();
