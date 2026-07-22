import 'cavimg';

const t0 = performance.now();
window.__perf = {} as Window['__perf'];

const app = document.querySelector<HTMLDivElement>('#app')!;
app.innerHTML = `
  <h1>cavimg — before / after</h1>
  <div style="display:flex;gap:24px;flex-wrap:wrap">
    <figure>
      <figcaption>Plain &lt;img&gt; (copyable)</figcaption>
      <img data-test="plain" src="/fixture.png?v=plain" alt="plain" style="display:block;width:360px" />
    </figure>
    <figure>
      <figcaption>&lt;cav-img&gt; (canvas, protected)</figcaption>
      <cav-img data-test="protected" src="/fixture.png?v=cav" alt="protected" style="display:block;width:360px"></cav-img>
    </figure>
  </div>`;

const plain = app.querySelector<HTMLImageElement>('img[data-test="plain"]')!;
const done = () => {
  if (plain.complete) window.__perf!.plainMs = performance.now() - t0;
};
plain.addEventListener('load', () => { window.__perf!.plainMs = performance.now() - t0; });
done();

app.querySelector('cav-img')!.addEventListener('cav-load', () => {
  window.__perf!.cavMs = performance.now() - t0;
  const mem = (performance as unknown as { memory?: { usedJSHeapSize: number } }).memory;
  if (mem) window.__perf!.heap = mem.usedJSHeapSize;
});
