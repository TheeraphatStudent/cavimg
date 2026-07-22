'use client';
import { useEffect, useRef, useState } from 'react';

declare global {
  interface Window { __perf?: { plainMs?: number; cavMs?: number; heap?: number }; }
}

export default function CavDemo() {
  const ref = useRef<HTMLDivElement>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;
    import('cavimg').then((m) => {
      m.defineCavImg();
      if (!cancelled) setReady(true);
    });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (!ready || !ref.current) return;
    const t0 = performance.now();
    window.__perf = {};
    const plain = ref.current.querySelector<HTMLImageElement>('img[data-test="plain"]')!;
    const setPlain = () => { window.__perf!.plainMs = performance.now() - t0; };
    if (plain.complete) setPlain();
    else plain.addEventListener('load', setPlain, { once: true });
    const cav = ref.current.querySelector('cav-img')!;
    cav.addEventListener('cav-load', () => {
      window.__perf!.cavMs = performance.now() - t0;
      const mem = (performance as unknown as { memory?: { usedJSHeapSize: number } }).memory;
      if (mem) window.__perf!.heap = mem.usedJSHeapSize;
    }, { once: true });
  }, [ready]);

  return (
    <div ref={ref}>
      <h1>cavimg — before / after</h1>
      {ready && (
        <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap' }}>
          <figure>
            <figcaption>Plain &lt;img&gt; (copyable)</figcaption>
            <img data-test="plain" src="/fixture.png?v=plain" alt="plain" style={{ display: 'block', width: 360 }} />
          </figure>
          <figure>
            <figcaption>&lt;cav-img&gt; (canvas, protected)</figcaption>
            {/* @ts-expect-error custom element */}
            <cav-img data-test="protected" src="/fixture.png?v=cav" alt="protected" style={{ display: 'block', width: 360 }} />
          </figure>
        </div>
      )}
    </div>
  );
}
