/**
 * Apply casual anti-copy hardening to the canvas: block drag-to-desktop and the
 * right-click "Save image as…" menu, and disable text/image selection. Returns
 * a cleanup that detaches the listeners.
 */
export function harden(canvas: HTMLCanvasElement): () => void {
  canvas.draggable = false;
  canvas.setAttribute('draggable', 'false');

  const prevent = (event: Event): void => event.preventDefault();
  canvas.addEventListener('dragstart', prevent);
  canvas.addEventListener('contextmenu', prevent);

  const style = canvas.style;
  style.setProperty('user-select', 'none');
  style.setProperty('-webkit-user-select', 'none');
  style.setProperty('-webkit-user-drag', 'none');
  style.setProperty('-webkit-touch-callout', 'none');

  return (): void => {
    canvas.removeEventListener('dragstart', prevent);
    canvas.removeEventListener('contextmenu', prevent);
  };
}
