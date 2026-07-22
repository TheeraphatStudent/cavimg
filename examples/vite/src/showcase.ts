import 'cavimg';

declare global {
  interface Window {
    __ready?: boolean;
    __frame?: (n: number) => void;
  }
}

const FRAMES = 24;
const cursor = document.getElementById('cursor')!;
const menu = document.getElementById('menu')!;
const chip = document.getElementById('chip')!;
const tagL = document.getElementById('tag-left')!;
const tagR = document.getElementById('tag-right')!;

// Anchor points within the 880x360 stage.
const START = { x: 430, y: 330 };
const LEFT = { x: 210, y: 150 };   // over the plain image
const RIGHT = { x: 660, y: 150 };  // over the cav-img

const lerp = (a: number, b: number, t: number) => a + (b - a) * Math.max(0, Math.min(1, t));
function place(el: HTMLElement, x: number, y: number) { el.style.left = `${x}px`; el.style.top = `${y}px`; }

function reset() {
  menu.style.display = 'none';
  menu.classList.remove('blocked');
  chip.style.display = 'none';
  tagL.className = 'tag neutral'; tagL.textContent = 'a normal image';
  tagR.className = 'tag neutral'; tagR.textContent = 'rendered to canvas';
}

// Pure function of n: pose the whole scene. No timers.
window.__frame = (n: number) => {
  reset();
  if (n < 6) {
    // glide from start toward the left image
    place(cursor, lerp(START.x, LEFT.x, n / 6), lerp(START.y, LEFT.y, n / 6));
  } else if (n < 12) {
    // right-click the plain image → menu opens, bad tag
    place(cursor, LEFT.x, LEFT.y);
    menu.style.display = 'block';
    place(menu, LEFT.x + 6, LEFT.y + 6);
    tagL.className = 'tag bad';
    tagL.textContent = '❌ saveable · draggable · URL in inspector';
  } else if (n < 18) {
    // glide to the cav-img
    const t = (n - 12) / 6;
    place(cursor, lerp(LEFT.x, RIGHT.x, t), lerp(LEFT.y, RIGHT.y, t));
  } else {
    // right-click the cav-img → blocked
    place(cursor, RIGHT.x, RIGHT.y);
    chip.style.display = 'block';
    place(chip, RIGHT.x - 30, RIGHT.y - 10);
    tagR.className = 'tag good';
    tagR.textContent = '✅ no <img> · no src · save & drag blocked';
  }
};

const prot = document.getElementById('prot')!;
const finish = () => { window.__frame!(0); window.__ready = true; };
prot.addEventListener('cav-load', finish, { once: true });
prot.addEventListener('cav-error', finish, { once: true }); // still pose the scene if load fails
// fallback so __ready is never stuck
setTimeout(() => { if (!window.__ready) finish(); }, 4000);
