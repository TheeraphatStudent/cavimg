# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`cavimg` is a browser image-protection library. Its purpose (per `package.json` and `README.md`) is to render a source image onto an HTML `<canvas>` so the result is undraggable and hard to copy/download — countering right-click save, drag-out, and casual download. Author: th33raphat. License: MIT. Repo: https://github.com/TheeraphatStudent/cavimg

## Current state — greenfield

The repository is essentially empty. As of the initial commit it contains only `package.json` and `README.md`.

- **No source code exists yet.** `package.json` declares `"main": "index.ts"`, but that file has not been created. TypeScript is the intended language (`.ts` entry), but there is no `tsconfig.json`, build tooling, bundler, or dependencies installed.
- **No test setup.** The `test` script is the default npm placeholder (`echo "Error: no test specified" && exit 1`) — running `npm test` will fail by design until a real test runner is added.
- **No lint/format/build tooling** is configured.

Because there is no implementation, there is no architecture to preserve yet. When building out the library, you are establishing these conventions, not following existing ones — set up the toolchain (TypeScript config, build/bundle for browser distribution, tests) as part of the work.

## Design intent (from metadata, not yet implemented)

The `keywords` in `package.json` describe the intended behavior to build toward: `image-to-canvas`, `canvas`, `image-protection`, `copy-protection`, `anti-copy`, `prevent-download`, `disable-right-click`, `no-drag`, `undraggable`, `image-security`, `browser`. The core mechanism is drawing an image into a canvas element and suppressing the browser affordances (drag, context-menu save, direct URL access) that normally allow copying it.
