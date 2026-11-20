// Registers the bundled UI and code fonts (Latin subsets only, to keep the binary
// small). Loaded as same-origin files, which the page's CSP (font-src 'self') allows.
import interLatin from '@fontsource-variable/inter/files/inter-latin-wght-normal.woff2?url';
import monoRegular from '@fontsource/jetbrains-mono/files/jetbrains-mono-latin-400-normal.woff2?url';
import monoSemibold from '@fontsource/jetbrains-mono/files/jetbrains-mono-latin-600-normal.woff2?url';

const FACES: [string, string, FontFaceDescriptors][] = [
  ['Inter Variable', interLatin, { weight: '100 900', style: 'normal', display: 'swap' }],
  ['JetBrains Mono', monoRegular, { weight: '400', style: 'normal', display: 'swap' }],
  ['JetBrains Mono', monoSemibold, { weight: '600', style: 'normal', display: 'swap' }],
];

export function registerFonts(): void {
  if (typeof FontFace === 'undefined') return;
  for (const [family, url, descriptors] of FACES) {
    const face = new FontFace(family, `url(${url}) format('woff2')`, descriptors);
    document.fonts.add(face);
    face.load().catch(() => undefined);
  }
}
