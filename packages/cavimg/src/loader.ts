/**
 * Load an image entirely in memory (no <img> mounted into the DOM), decode it
 * to an ImageBitmap, then discard the URL. The url is a local parameter and is
 * never stored. Error messages deliberately omit the url.
 */
export function loadImageBitmap(url: string): Promise<ImageBitmap> {
  return new Promise<ImageBitmap>((resolve, reject) => {
    const img = new Image();
    img.crossOrigin = 'anonymous';
    img.decoding = 'async';

    const cleanup = (): void => {
      img.onload = null;
      img.onerror = null;
    };

    img.onload = (): void => {
      createImageBitmap(img)
        .then((bitmap) => {
          cleanup();
          img.src = ''; // discard the source
          resolve(bitmap);
        })
        .catch(() => {
          cleanup();
          reject(new Error('cavimg: failed to decode image'));
        });
    };

    img.onerror = (): void => {
      cleanup();
      reject(new Error('cavimg: failed to load image'));
    };

    img.src = url;
  });
}
