export {};
declare global {
  interface Window {
    __perf?: { plainMs: number; cavMs: number; heap?: number };
  }
}
