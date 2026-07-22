import { Component, CUSTOM_ELEMENTS_SCHEMA, AfterViewInit, ElementRef } from '@angular/core';
import { defineCavImg } from 'cavimg';

declare global {
  interface Window { __perf?: { plainMs?: number; cavMs?: number; heap?: number }; }
}

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [],
  schemas: [CUSTOM_ELEMENTS_SCHEMA],
  templateUrl: './app.html',
  styleUrl: './app.css',
})
export class App implements AfterViewInit {
  constructor(private host: ElementRef<HTMLElement>) { defineCavImg(); }

  ngAfterViewInit(): void {
    const t0 = performance.now();
    window.__perf = {};
    const root = this.host.nativeElement;
    const plain = root.querySelector('img[data-test="plain"]') as HTMLImageElement;
    const setPlain = () => { window.__perf!.plainMs = performance.now() - t0; };
    if (plain.complete) setPlain(); else plain.addEventListener('load', setPlain, { once: true });
    root.querySelector('cav-img')!.addEventListener('cav-load', () => {
      window.__perf!.cavMs = performance.now() - t0;
      const mem = (performance as unknown as { memory?: { usedJSHeapSize: number } }).memory;
      if (mem) window.__perf!.heap = mem.usedJSHeapSize;
    }, { once: true });
  }
}
