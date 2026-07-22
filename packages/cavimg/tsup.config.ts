import { defineConfig } from 'tsup';

export default defineConfig({
  entry: { index: 'src/index.ts' },
  format: ['esm', 'iife'],
  globalName: 'Cavimg',
  dts: true,
  clean: true,
  minify: true,
  sourcemap: true,
  target: 'es2022',
});
