function loadImage(source: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("decode failed"));
    image.src = source;
  });
}

/**
 * Produces a pre-blurred copy of the wallpaper. Rendering the blur once into a
 * static image keeps glass backgrounds smooth without re-filtering the source
 * on every composite. Runtime glass blur remains active on top.
 */
export async function blurWallpaper(source: string): Promise<string> {
  if (!source) return "";
  const image = await loadImage(source);
  const maxDimension = 1280;
  const scale = Math.min(1, maxDimension / Math.max(image.naturalWidth, image.naturalHeight));
  const width = Math.max(1, Math.round(image.naturalWidth * scale));
  const height = Math.max(1, Math.round(image.naturalHeight * scale));
  const blur = Math.max(10, Math.round(24 * scale));
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d");
  if (!context || !("filter" in context)) return source;
  context.filter = `blur(${blur}px)`;
  // Draw oversized so the blur never samples transparent pixels at the edges.
  context.drawImage(image, -blur, -blur, width + blur * 2, height + blur * 2);
  return canvas.toDataURL("image/jpeg", 0.8);
}
