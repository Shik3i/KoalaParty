import { writable } from 'svelte/store';

export type AmbilightMode = 'dynamic' | 'dynamic-thumbnail' | 'static' | 'thumbnail' | 'off';

export interface AmbilightConfig {
  mode: AmbilightMode;
  color: string;
  intensity: number; // 0.2 to 1.0
  blur: number; // 24 to 80 px
}

export const AMBILIGHT_MODES: Array<{ value: AmbilightMode; labelKey: string }> = [
  { value: 'dynamic', labelKey: 'ambilight.modeDynamic' },
  { value: 'dynamic-thumbnail', labelKey: 'ambilight.modeAdaptive' },
  { value: 'static', labelKey: 'ambilight.modeStatic' },
  { value: 'thumbnail', labelKey: 'ambilight.modeThumbnail' },
  { value: 'off', labelKey: 'ambilight.modeOff' },
];

export const AMBILIGHT_SWATCHES = [
  { label: 'Cyan Glow', value: '#00e5ff' },
  { label: 'Cyber Purple', value: '#a855f7' },
  { label: 'Sunset Amber', value: '#ff7a00' },
  { label: 'Emerald Mint', value: '#10b981' },
  { label: 'Neon Pink', value: '#f43f5e' },
  { label: 'Warm Gold', value: '#eab308' },
  { label: 'Arctic Frost', value: '#e0f2fe' },
];

export const DEFAULT_AMBILIGHT: AmbilightConfig = {
  mode: 'dynamic',
  color: '#00e5ff',
  intensity: 0.75,
  blur: 52,
};

const storageKey = 'koalaparty.ambilight';

export function initialAmbilight(): AmbilightConfig {
  if (typeof localStorage === 'undefined') return { ...DEFAULT_AMBILIGHT };
  try {
    const raw = localStorage.getItem(storageKey);
    if (!raw) return { ...DEFAULT_AMBILIGHT };
    const parsed = JSON.parse(raw) as Partial<AmbilightConfig>;
    const validModes: AmbilightMode[] = ['dynamic', 'dynamic-thumbnail', 'static', 'thumbnail', 'off'];
    const mode = validModes.includes(parsed.mode as AmbilightMode)
      ? (parsed.mode as AmbilightMode)
      : DEFAULT_AMBILIGHT.mode;
    const color =
      typeof parsed.color === 'string' && /^#[0-9a-fA-F]{3,8}$/.test(parsed.color)
        ? parsed.color
        : DEFAULT_AMBILIGHT.color;
    const intensity =
      typeof parsed.intensity === 'number' && parsed.intensity >= 0.1 && parsed.intensity <= 1
        ? parsed.intensity
        : DEFAULT_AMBILIGHT.intensity;
    const blur =
      typeof parsed.blur === 'number' && parsed.blur >= 16 && parsed.blur <= 100 ? parsed.blur : DEFAULT_AMBILIGHT.blur;
    return { mode, color, intensity, blur };
  } catch {
    return { ...DEFAULT_AMBILIGHT };
  }
}

export const ambilight = writable<AmbilightConfig>(initialAmbilight());

export function updateAmbilight(patch: Partial<AmbilightConfig>) {
  ambilight.update((current) => {
    const next: AmbilightConfig = { ...current, ...patch };
    if (typeof localStorage !== 'undefined') {
      try {
        localStorage.setItem(storageKey, JSON.stringify(next));
      } catch {
        /* storage unavailable */
      }
    }
    return next;
  });
}

export function resetAmbilight() {
  updateAmbilight(DEFAULT_AMBILIGHT);
}

/**
 * Deterministically generates a harmonious 4-color palette from any string seed (e.g. videoId or title).
 * Used when image pixel data cannot be read due to cross-origin canvas security (CORS).
 */
export function generatePaletteFromSeed(seed: string): [string, string, string, string] {
  let hash = 0;
  for (let i = 0; i < seed.length; i++) {
    hash = (hash << 5) - hash + seed.charCodeAt(i);
    hash |= 0;
  }
  const baseHue = Math.abs(hash) % 360;
  const h1 = baseHue;
  const h2 = (baseHue + 45) % 360;
  const h3 = (baseHue + 160) % 360;
  const h4 = (baseHue + 210) % 360;

  return [`hsl(${h1}, 82%, 58%)`, `hsl(${h2}, 88%, 62%)`, `hsl(${h3}, 80%, 54%)`, `hsl(${h4}, 85%, 60%)`];
}

const rgbToHex = (r: number, g: number, b: number) =>
  '#' + [r, g, b].map((x) => x.toString(16).padStart(2, '0')).join('');

/**
 * Attempts to extract corner colors from a thumbnail. If CORS prevents canvas read,
 * smoothly falls back to a deterministic harmonious palette.
 */
export async function extractPalette(url: string, seed = 'default'): Promise<[string, string, string, string]> {
  if (typeof window === 'undefined' || !url) return generatePaletteFromSeed(seed);

  return new Promise((resolve) => {
    const img = new Image();
    img.crossOrigin = 'anonymous';

    const fallback = () => resolve(generatePaletteFromSeed(seed || url));

    img.onload = () => {
      try {
        const canvas = document.createElement('canvas');
        canvas.width = 4;
        canvas.height = 4;
        const ctx = canvas.getContext('2d', { willReadFrequently: true });
        if (!ctx) return fallback();

        ctx.drawImage(img, 0, 0, 4, 4);
        const data = ctx.getImageData(0, 0, 4, 4).data;

        // Top-left (0,0), Top-right (3,0), Bottom-left (0,3), Bottom-right (3,3)
        const c1 = rgbToHex(data[0], data[1], data[2]);
        const c2 = rgbToHex(data[12], data[13], data[14]);
        const c3 = rgbToHex(data[48], data[49], data[50]);
        const c4 = rgbToHex(data[60], data[61], data[62]);

        resolve([c1, c2, c3, c4]);
      } catch {
        fallback();
      }
    };

    img.onerror = fallback;
    img.src = url;
  });
}
