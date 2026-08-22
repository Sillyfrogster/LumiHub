import { createHash } from "node:crypto";
import { mkdir, readFile, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import sharp from "sharp";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(scriptDirectory, "..");
const repositoryRoot = path.resolve(webRoot, "..");
const productionDirectory = path.join(webRoot, "src/assets/art/full");
const reviewDirectory = path.join(
  repositoryRoot,
  ".ai/issues/refractive-identity/assets",
);

const WIDTH_PHONE = 800;
const HEIGHT_PHONE = 1280;
const FRAGMENT_WIDTH = 1024;
const FRAGMENT_HEIGHT = 256;
const APERTURE_SIZE = 512;
const RELIEF_WIDTH = 1024;
const RELIEF_HEIGHT = 256;
const SEED = 0x1a11_a21;

const outputs = {
  darkPhone: path.join(
    productionDirectory,
    "illarin-browse-aperture-dark-phone-v1.webp",
  ),
  lightPhone: path.join(
    productionDirectory,
    "illarin-browse-aperture-light-phone-v1.webp",
  ),
  fragments: path.join(productionDirectory, "illarin-optical-fragments-v1.png"),
  apertureBmr: path.join(productionDirectory, "illarin-aperture-bmr-v1.png"),
  kindReliefBmr: path.join(
    productionDirectory,
    "illarin-kind-relief-bmr-v1.png",
  ),
};

const reviews = {
  phones: path.join(reviewDirectory, "phone-crops-check-v1.webp"),
  fragments: path.join(reviewDirectory, "optical-fragments-check-v1.png"),
  materials: path.join(reviewDirectory, "material-maps-check-v1.png"),
};

function clamp(value, minimum = 0, maximum = 1) {
  return Math.min(maximum, Math.max(minimum, value));
}

function smoothstep(edge0, edge1, value) {
  const progress = clamp((value - edge0) / (edge1 - edge0));
  return progress * progress * (3 - 2 * progress);
}

function byte(value) {
  return Math.round(clamp(value, 0, 255));
}

function hash2(x, y, seed = SEED) {
  let hash = Math.imul(x ^ seed, 0x45d9_f3b);
  hash = Math.imul(hash ^ (hash >>> 16) ^ y, 0x45d9_f3b);
  hash ^= hash >>> 16;
  return (hash >>> 0) / 4_294_967_295;
}

function segmentDistance(x, y, ax, ay, bx, by) {
  const dx = bx - ax;
  const dy = by - ay;
  const lengthSquared = dx * dx + dy * dy;
  const projection =
    lengthSquared === 0
      ? 0
      : clamp(((x - ax) * dx + (y - ay) * dy) / lengthSquared);
  return Math.hypot(x - (ax + dx * projection), y - (ay + dy * projection));
}

function lineMask(x, y, ax, ay, bx, by, width, feather = 1.25) {
  return (
    1 -
    smoothstep(width, width + feather, segmentDistance(x, y, ax, ay, bx, by))
  );
}

function ringMask(x, y, radius, width, feather = 1.25) {
  return (
    1 - smoothstep(width, width + feather, Math.abs(Math.hypot(x, y) - radius))
  );
}

function rectangleDistance(x, y, halfWidth, halfHeight) {
  const dx = Math.abs(x) - halfWidth;
  const dy = Math.abs(y) - halfHeight;
  return (
    Math.hypot(Math.max(dx, 0), Math.max(dy, 0)) + Math.min(Math.max(dx, dy), 0)
  );
}

function rectangleStrokeMask(
  x,
  y,
  halfWidth,
  halfHeight,
  width,
  feather = 1.25,
) {
  return (
    1 -
    smoothstep(
      width,
      width + feather,
      Math.abs(rectangleDistance(x, y, halfWidth, halfHeight)),
    )
  );
}

function polygonMask(x, y, points, feather = 1.25) {
  let inside = false;
  let edgeDistance = Number.POSITIVE_INFINITY;

  for (
    let index = 0, previous = points.length - 1;
    index < points.length;
    previous = index++
  ) {
    const [x1, y1] = points[index];
    const [x2, y2] = points[previous];
    const crosses = y1 > y !== y2 > y;
    if (crosses && x < ((x2 - x1) * (y - y1)) / (y2 - y1) + x1) {
      inside = !inside;
    }
    edgeDistance = Math.min(
      edgeDistance,
      segmentDistance(x, y, x1, y1, x2, y2),
    );
  }

  return inside ? 1 : 1 - smoothstep(0, feather, edgeDistance);
}

function rotate(x, y, angle) {
  const cosine = Math.cos(angle);
  const sine = Math.sin(angle);
  return [x * cosine - y * sine, x * sine + y * cosine];
}

async function generatePhoneCrop({ sourceName, outputPath, background }) {
  const sourcePath = path.join(productionDirectory, sourceName);
  const sourceMetadata = await sharp(sourcePath).metadata();

  if (
    sourceMetadata.width !== 1920 ||
    sourceMetadata.height !== 640 ||
    sourceMetadata.channels < 3
  ) {
    throw new Error(`${sourceName} is not the approved 1920x640 source plate.`);
  }

  const cropLeft = 260;
  const cropWidth = 1100;
  const resizedHeight = Math.round(
    (sourceMetadata.height * WIDTH_PHONE) / cropWidth,
  );
  const crop = await sharp(sourcePath)
    .extract({
      left: cropLeft,
      top: 0,
      width: cropWidth,
      height: sourceMetadata.height,
    })
    .resize({ width: WIDTH_PHONE, height: resizedHeight, fit: "fill" })
    .ensureAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true });

  const feathered = Buffer.from(crop.data);
  const topFeather = 30;
  const bottomFeather = 70;
  for (let y = 0; y < crop.info.height; y += 1) {
    const topAlpha = smoothstep(0, topFeather, y);
    const bottomAlpha = smoothstep(0, bottomFeather, crop.info.height - 1 - y);
    const alpha = byte(255 * topAlpha * bottomAlpha);
    for (let x = 0; x < crop.info.width; x += 1) {
      const offset = (y * crop.info.width + x) * 4;
      const luma = byte(
        feathered[offset] * 0.2126 +
          feathered[offset + 1] * 0.7152 +
          feathered[offset + 2] * 0.0722,
      );
      feathered[offset] = luma;
      feathered[offset + 1] = luma;
      feathered[offset + 2] = luma;
      feathered[offset + 3] = alpha;
    }
  }

  await sharp({
    create: {
      width: WIDTH_PHONE,
      height: HEIGHT_PHONE,
      channels: 4,
      background: { ...background, alpha: 1 },
    },
  })
    .composite([
      {
        input: feathered,
        raw: {
          width: crop.info.width,
          height: crop.info.height,
          channels: 4,
        },
        left: 0,
        top: 350,
      },
    ])
    .removeAlpha()
    .webp({ quality: 88, alphaQuality: 100, effort: 6, smartSubsample: true })
    .toFile(outputPath);
}

function fragmentMask(cell, x, y) {
  if (cell === 0) {
    const shard = polygonMask(x, y, [
      [0, -88],
      [9, -24],
      [43, -3],
      [13, 10],
      [0, 88],
      [-10, 24],
      [-43, 2],
      [-12, -10],
    ]);
    const spine = lineMask(x, y, 0, -82, 0, 82, 1.3);
    return Math.max(shard * 0.78, spine * 0.95);
  }

  if (cell === 1) {
    const blade = polygonMask(x, y, [
      [-48, 73],
      [-27, -73],
      [11, -89],
      [45, 60],
      [8, 86],
    ]);
    const facet = lineMask(x, y, -27, -70, 8, 82, 1.5);
    return Math.max(blade * 0.54, facet * 0.92);
  }

  if (cell === 2) {
    const radius = Math.hypot(x + 5, y);
    const cutout = Math.hypot(x - 17, y - 2);
    const crescent = radius < 58 && cutout > 51 ? 0.82 : 0;
    const edge = ringMask(x + 5, y, 58, 1.4);
    return Math.max(crescent, edge * 0.92);
  }

  if (cell === 3) {
    const upper = polygonMask(x, y, [
      [-43, -66],
      [23, -94],
      [8, -25],
    ]);
    const middle = polygonMask(x, y, [
      [-30, -5],
      [43, -34],
      [17, 33],
    ]);
    const lower = polygonMask(x, y, [
      [-45, 70],
      [18, 39],
      [37, 91],
    ]);
    return Math.max(upper * 0.78, middle * 0.58, lower * 0.7);
  }

  if (cell === 4) {
    const outer = ringMask(x, y, 50, 3.2);
    const inner = ringMask(x, y, 22, 1.6);
    const pin = 1 - smoothstep(8, 10, Math.hypot(x, y));
    const arm = lineMask(x, y, -46, 0, 46, 0, 1.4);
    return Math.max(outer * 0.72, inner * 0.92, pin * 0.65, arm * 0.45);
  }

  if (cell === 5) {
    const [rotatedX, rotatedY] = rotate(x, y, -0.18);
    const pane = polygonMask(rotatedX, rotatedY, [
      [-42, -82],
      [35, -66],
      [43, 72],
      [-33, 87],
    ]);
    const seam = lineMask(rotatedX, rotatedY, -33, -57, 36, 58, 1.2);
    const edge = Math.max(
      lineMask(rotatedX, rotatedY, -42, -82, 35, -66, 1.3),
      lineMask(rotatedX, rotatedY, 35, -66, 43, 72, 1.3),
      lineMask(rotatedX, rotatedY, 43, 72, -33, 87, 1.3),
      lineMask(rotatedX, rotatedY, -33, 87, -42, -82, 1.3),
    );
    return Math.max(pane * 0.2, seam * 0.8, edge * 0.94);
  }

  if (cell === 6) {
    const rayA = lineMask(x, y, -44, 75, 8, -88, 1.2);
    const rayB = lineMask(x, y, -20, 84, 11, -88, 2.2);
    const rayC = lineMask(x, y, 8, 85, 16, -88, 1.25);
    const rayD = lineMask(x, y, 32, 76, 21, -88, 1.1);
    const gate = lineMask(x, y, -36, 18, 41, -6, 1.1);
    return Math.max(rayA * 0.5, rayB * 0.75, rayC, rayD * 0.58, gate * 0.62);
  }

  const glints = [
    [-19, -60, 31],
    [17, -6, 52],
    [-29, 61, 23],
    [32, 81, 12],
  ];
  let result = 0;
  for (const [centerX, centerY, size] of glints) {
    const horizontal = lineMask(
      x,
      y,
      centerX - size,
      centerY,
      centerX + size,
      centerY,
      1.2,
    );
    const vertical = lineMask(
      x,
      y,
      centerX,
      centerY - size * 1.55,
      centerX,
      centerY + size * 1.55,
      1.2,
    );
    const core = 1 - smoothstep(2, 8, Math.hypot(x - centerX, y - centerY));
    result = Math.max(result, horizontal * 0.72, vertical * 0.9, core);
  }
  return result;
}

async function generateOpticalFragments() {
  const channels = 4;
  const data = Buffer.alloc(FRAGMENT_WIDTH * FRAGMENT_HEIGHT * channels);
  const cellWidth = FRAGMENT_WIDTH / 8;

  for (let y = 0; y < FRAGMENT_HEIGHT; y += 1) {
    for (let x = 0; x < FRAGMENT_WIDTH; x += 1) {
      const cell = Math.floor(x / cellWidth);
      const localX = x - cell * cellWidth - (cellWidth - 1) / 2;
      const localY = y - (FRAGMENT_HEIGHT - 1) / 2;
      const mask = clamp(fragmentMask(cell, localX, localY));
      const grain = hash2(x, y, SEED ^ 0x5f37_59df);
      const directional = clamp(0.5 + localX / 180 - localY / 620);
      const luma = byte(208 + directional * 36 + grain * 5);
      const offset = (y * FRAGMENT_WIDTH + x) * channels;
      data[offset] = luma;
      data[offset + 1] = luma;
      data[offset + 2] = luma;
      data[offset + 3] = byte(mask * 245);
    }
  }

  await sharp(data, {
    raw: { width: FRAGMENT_WIDTH, height: FRAGMENT_HEIGHT, channels },
  })
    .png({ compressionLevel: 9, adaptiveFiltering: true })
    .toFile(outputs.fragments);
}

async function generateApertureBmr() {
  const channels = 3;
  const data = Buffer.alloc(APERTURE_SIZE * APERTURE_SIZE * channels);
  const center = (APERTURE_SIZE - 1) / 2;

  for (let y = 0; y < APERTURE_SIZE; y += 1) {
    for (let x = 0; x < APERTURE_SIZE; x += 1) {
      const nx = (x - center) / center;
      const ny = (y - center) / center;
      const radius = Math.hypot(nx, ny);
      const angle = Math.atan2(ny, nx) + Math.PI;
      const sector = ((angle / (Math.PI * 2)) * 5 + 0.118) % 1;
      const facet = 1 - Math.abs(sector * 2 - 1);
      const bladeBoundary =
        1 - smoothstep(0.018, 0.055, Math.min(sector, 1 - sector));
      const apertureEdge =
        1 - smoothstep(0.012, 0.035, Math.abs(radius - 0.335));
      const outerBevel = 1 - smoothstep(0.01, 0.045, Math.abs(radius - 0.91));
      const innerVoid = 1 - smoothstep(0.315, 0.345, radius);
      const outside = smoothstep(0.91, 0.98, radius);
      const grain = hash2(x >> 1, y >> 1, SEED ^ 0x713e_a9d5) - 0.5;
      const brushed = Math.sin((nx * 0.77 + ny * 0.23) * 910 + grain * 2.2);
      const bladeSurface = clamp((radius - 0.31) / 0.65) * (1 - outside);

      let bump = 74 + bladeSurface * (49 + facet * 42);
      bump += bladeBoundary * 61 + apertureEdge * 68 + outerBevel * 42;
      bump += brushed * 3.5 + grain * 6;
      bump *= 1 - innerVoid * 0.38;

      let metalness = 142 + bladeSurface * 76;
      metalness += bladeBoundary * 25 + outerBevel * 22;
      metalness *= 1 - innerVoid * 0.42;

      let roughness = 139 - bladeSurface * 47;
      roughness += (1 - facet) * 18 + grain * 17 + brushed * 5;
      roughness -= bladeBoundary * 31 + apertureEdge * 25;
      roughness += innerVoid * 44 + outside * 31;

      const offset = (y * APERTURE_SIZE + x) * channels;
      data[offset] = byte(bump);
      data[offset + 1] = byte(metalness);
      data[offset + 2] = byte(roughness);
    }
  }

  await sharp(data, {
    raw: { width: APERTURE_SIZE, height: APERTURE_SIZE, channels },
  })
    .png({ compressionLevel: 9, adaptiveFiltering: true })
    .toFile(outputs.apertureBmr);
}

function characterRelief(x, y) {
  const upper = lineMask(x, y, -63, 5, 0, -25, 2.2, 1.8);
  const upperRight = lineMask(x, y, 0, -25, 63, 5, 2.2, 1.8);
  const lower = lineMask(x, y, -63, 5, 0, 31, 2.2, 1.8);
  const lowerRight = lineMask(x, y, 0, 31, 63, 5, 2.2, 1.8);
  const iris = ringMask(x, y + 1, 19, 2.3, 1.5);
  const pupil = 1 - smoothstep(6, 9, Math.hypot(x, y + 1));
  return Math.max(upper, upperRight, lower, lowerRight, iris, pupil * 0.88);
}

function lorebookRelief(x, y) {
  const spine = lineMask(x, y, 0, -54, 0, 58, 2.2);
  const leftTop = lineMask(x, y, -72, -41, -5, -57, 2.2);
  const rightTop = lineMask(x, y, 5, -57, 72, -41, 2.2);
  const leftEdge = lineMask(x, y, -72, -41, -72, 46, 2.2);
  const rightEdge = lineMask(x, y, 72, -41, 72, 46, 2.2);
  const leftBottom = lineMask(x, y, -72, 46, -4, 61, 2.2);
  const rightBottom = lineMask(x, y, 4, 61, 72, 46, 2.2);
  const pageLeft = lineMask(x, y, -57, 6, -14, 16, 1.4);
  const pageRight = lineMask(x, y, 14, 16, 57, 6, 1.4);
  return Math.max(
    spine,
    leftTop,
    rightTop,
    leftEdge,
    rightEdge,
    leftBottom,
    rightBottom,
    pageLeft * 0.68,
    pageRight * 0.68,
  );
}

function presetRelief(x, y) {
  const rows = [-48, 0, 48];
  const knobs = [-26, 37, -3];
  let result = 0;
  for (let index = 0; index < rows.length; index += 1) {
    result = Math.max(
      result,
      lineMask(x, y, -70, rows[index], 70, rows[index], 1.8) * 0.72,
      ringMask(x - knobs[index], y - rows[index], 9, 2.3),
      1 - smoothstep(4, 6, Math.hypot(x - knobs[index], y - rows[index])),
    );
  }
  return result;
}

function themeRelief(x, y) {
  let result = Math.max(ringMask(x, y, 25, 2.1), ringMask(x, y, 48, 1.8) * 0.7);
  for (let index = 0; index < 8; index += 1) {
    const angle = (index / 8) * Math.PI * 2;
    result = Math.max(
      result,
      lineMask(
        x,
        y,
        Math.cos(angle) * 59,
        Math.sin(angle) * 59,
        Math.cos(angle) * 78,
        Math.sin(angle) * 78,
        1.8,
      ) * 0.78,
    );
  }
  return result;
}

function packRelief(x, y) {
  const back = rectangleStrokeMask(x + 26, y - 24, 42, 36, 2.2);
  const middle = rectangleStrokeMask(x, y, 48, 41, 2.2);
  const front = rectangleStrokeMask(x - 24, y + 26, 42, 36, 2.2);
  const lock = lineMask(x, y, -25, -15, 25, 15, 1.6);
  return Math.max(back * 0.58, middle * 0.76, front, lock * 0.72);
}

function kindMotif(kind, x, y) {
  if (kind === 0) return characterRelief(x, y);
  if (kind === 1) return lorebookRelief(x, y);
  if (kind === 2) return presetRelief(x, y);
  if (kind === 3) return themeRelief(x, y);
  return packRelief(x, y);
}

async function generateKindReliefBmr() {
  const channels = 3;
  const data = Buffer.alloc(RELIEF_WIDTH * RELIEF_HEIGHT * channels);
  const cellEdges = [0, 205, 410, 615, 820, RELIEF_WIDTH];

  for (let y = 0; y < RELIEF_HEIGHT; y += 1) {
    for (let x = 0; x < RELIEF_WIDTH; x += 1) {
      const kind = cellEdges.findIndex(
        (_edge, index) => index < 5 && x < cellEdges[index + 1],
      );
      const left = cellEdges[kind];
      const right = cellEdges[kind + 1];
      const localX = x - (left + right - 1) / 2;
      const localY = y - (RELIEF_HEIGHT - 1) / 2;
      const motif = kindMotif(kind, localX, localY);
      const edgeX = Math.min(x - left, right - 1 - x);
      const edgeY = Math.min(y, RELIEF_HEIGHT - 1 - y);
      const panelBevel =
        (1 - smoothstep(0, 16, Math.min(edgeX, edgeY))) *
        smoothstep(1, 5, Math.min(edgeX, edgeY));
      const separation = edgeX < 2 ? 1 - edgeX / 2 : 0;
      const grain = hash2(x >> 1, y >> 1, SEED ^ (kind * 0x9e37_79b9)) - 0.5;
      const brushed = Math.sin(localY * 1.73 + localX * 0.11 + kind * 0.9);

      const bump = 88 + motif * 119 + panelBevel * 34 + grain * 7;
      const metalness = 171 + motif * 61 - panelBevel * 24 - separation * 76;
      const roughness =
        126 - motif * 57 + panelBevel * 38 + grain * 19 + brushed * 4;

      const offset = (y * RELIEF_WIDTH + x) * channels;
      data[offset] = byte(bump);
      data[offset + 1] = byte(metalness);
      data[offset + 2] = byte(roughness);
    }
  }

  await sharp(data, {
    raw: { width: RELIEF_WIDTH, height: RELIEF_HEIGHT, channels },
  })
    .png({ compressionLevel: 9, adaptiveFiltering: true })
    .toFile(outputs.kindReliefBmr);
}

async function generatePhoneReview() {
  const previewWidth = 430;
  const previewHeight = 688;
  const darkPreview = await sharp(outputs.darkPhone)
    .resize(previewWidth, previewHeight)
    .toBuffer();
  const lightPreview = await sharp(outputs.lightPhone)
    .resize(previewWidth, previewHeight)
    .toBuffer();

  await sharp({
    create: {
      width: 1040,
      height: 768,
      channels: 3,
      background: { r: 8, g: 10, b: 12 },
    },
  })
    .composite([
      { input: darkPreview, left: 60, top: 40 },
      { input: lightPreview, left: 550, top: 40 },
    ])
    .webp({ quality: 88, effort: 6 })
    .toFile(reviews.phones);
}

async function generateFragmentReview() {
  const fragmentRaw = await sharp(outputs.fragments)
    .ensureAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true });
  const darkInk = Buffer.from(fragmentRaw.data);
  for (let index = 0; index < darkInk.length; index += 4) {
    darkInk[index] = 18;
    darkInk[index + 1] = 20;
    darkInk[index + 2] = 22;
  }

  await sharp({
    create: {
      width: FRAGMENT_WIDTH,
      height: FRAGMENT_HEIGHT * 2,
      channels: 3,
      background: { r: 244, g: 246, b: 247 },
    },
  })
    .composite([
      {
        input: {
          create: {
            width: FRAGMENT_WIDTH,
            height: FRAGMENT_HEIGHT,
            channels: 3,
            background: { r: 7, g: 9, b: 11 },
          },
        },
        left: 0,
        top: 0,
      },
      { input: outputs.fragments, left: 0, top: 0 },
      {
        input: darkInk,
        raw: {
          width: fragmentRaw.info.width,
          height: fragmentRaw.info.height,
          channels: 4,
        },
        left: 0,
        top: FRAGMENT_HEIGHT,
      },
    ])
    .png({ compressionLevel: 9, adaptiveFiltering: true })
    .toFile(reviews.fragments);
}

async function grayscaleChannel(inputPath, channel, width, height) {
  return sharp(inputPath)
    .extractChannel(channel)
    .resize(width, height, { fit: "fill" })
    .png({ compressionLevel: 9, adaptiveFiltering: true })
    .toBuffer();
}

async function generateMaterialReview() {
  const aperturePreviews = await Promise.all(
    [0, 1, 2].map((channel) =>
      grayscaleChannel(outputs.apertureBmr, channel, 300, 300),
    ),
  );
  const reliefPreviews = await Promise.all(
    [0, 1, 2].map((channel) =>
      grayscaleChannel(outputs.kindReliefBmr, channel, 1024, 256),
    ),
  );

  await sharp({
    create: {
      width: 1024,
      height: 1156,
      channels: 3,
      background: { r: 10, g: 12, b: 14 },
    },
  })
    .composite([
      { input: aperturePreviews[0], left: 32, top: 28 },
      { input: aperturePreviews[1], left: 362, top: 28 },
      { input: aperturePreviews[2], left: 692, top: 28 },
      { input: reliefPreviews[0], left: 0, top: 356 },
      { input: reliefPreviews[1], left: 0, top: 628 },
      { input: reliefPreviews[2], left: 0, top: 900 },
    ])
    .png({ compressionLevel: 9, adaptiveFiltering: true })
    .toFile(reviews.materials);
}

async function inspectOutput(filePath, expected) {
  const metadata = await sharp(filePath).metadata();
  const fileStat = await stat(filePath);
  const fileBuffer = await readFile(filePath);
  const digest = createHash("sha256").update(fileBuffer).digest("hex");

  if (
    metadata.width !== expected.width ||
    metadata.height !== expected.height ||
    metadata.channels !== expected.channels ||
    metadata.format !== expected.format
  ) {
    throw new Error(
      `${path.basename(filePath)} has unexpected metadata: ${JSON.stringify(metadata)}`,
    );
  }

  return {
    file: path.relative(repositoryRoot, filePath),
    width: metadata.width,
    height: metadata.height,
    channels: metadata.channels,
    format: metadata.format,
    bytes: fileStat.size,
    sha256: digest,
  };
}

async function main() {
  await mkdir(productionDirectory, { recursive: true });
  await mkdir(reviewDirectory, { recursive: true });

  await Promise.all([
    generatePhoneCrop({
      sourceName: "illarin-browse-aperture-dark-v1.webp",
      outputPath: outputs.darkPhone,
      background: { r: 4, g: 6, b: 8 },
    }),
    generatePhoneCrop({
      sourceName: "illarin-browse-aperture-light-v1.webp",
      outputPath: outputs.lightPhone,
      background: { r: 243, g: 246, b: 247 },
    }),
    generateOpticalFragments(),
    generateApertureBmr(),
    generateKindReliefBmr(),
  ]);

  await Promise.all([
    generatePhoneReview(),
    generateFragmentReview(),
    generateMaterialReview(),
  ]);

  const inspections = await Promise.all([
    inspectOutput(outputs.darkPhone, {
      width: WIDTH_PHONE,
      height: HEIGHT_PHONE,
      channels: 3,
      format: "webp",
    }),
    inspectOutput(outputs.lightPhone, {
      width: WIDTH_PHONE,
      height: HEIGHT_PHONE,
      channels: 3,
      format: "webp",
    }),
    inspectOutput(outputs.fragments, {
      width: FRAGMENT_WIDTH,
      height: FRAGMENT_HEIGHT,
      channels: 4,
      format: "png",
    }),
    inspectOutput(outputs.apertureBmr, {
      width: APERTURE_SIZE,
      height: APERTURE_SIZE,
      channels: 3,
      format: "png",
    }),
    inspectOutput(outputs.kindReliefBmr, {
      width: RELIEF_WIDTH,
      height: RELIEF_HEIGHT,
      channels: 3,
      format: "png",
    }),
  ]);

  process.stdout.write(
    `${JSON.stringify({ seed: SEED, outputs: inspections }, null, 2)}\n`,
  );
}

await main();
