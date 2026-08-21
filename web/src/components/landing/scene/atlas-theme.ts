import type { CanvasTexture, Texture } from "three";
import type { ThreeModule } from "./types";

/**
 * Light theme needs dark cards. Inverting the dark-theme art keeps one set of
 * card designs instead of two.
 */
export function createInvertedAtlas(
  THREE: ThreeModule,
  source: Texture,
): CanvasTexture | null {
  const image = source.image as
    | HTMLImageElement
    | HTMLCanvasElement
    | ImageBitmap
    | undefined;
  if (!image || !("width" in image) || !image.width) return null;

  const canvas = document.createElement("canvas");
  canvas.width = image.width;
  canvas.height = image.height;

  const context = canvas.getContext("2d");
  if (!context) return null;

  context.drawImage(image as CanvasImageSource, 0, 0);

  // Inverting floods alpha, so the original alpha is masked back in after.
  context.globalCompositeOperation = "difference";
  context.fillStyle = "#ffffff";
  context.fillRect(0, 0, canvas.width, canvas.height);

  context.globalCompositeOperation = "destination-in";
  context.drawImage(image as CanvasImageSource, 0, 0);
  context.globalCompositeOperation = "source-over";

  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.anisotropy = 8;
  texture.needsUpdate = true;
  return texture;
}
