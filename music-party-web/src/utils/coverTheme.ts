type Rgb = { r: number; g: number; b: number }

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value))
const hex = ({ r, g, b }: Rgb) => `#${[r, g, b].map(channel => Math.round(channel).toString(16).padStart(2, '0')).join('')}`
const rgba = ({ r, g, b }: Rgb, alpha: number) => `rgba(${Math.round(r)}, ${Math.round(g)}, ${Math.round(b)}, ${alpha})`
const mix = (color: Rgb, target: Rgb, amount: number): Rgb => ({
  r: color.r + (target.r - color.r) * amount,
  g: color.g + (target.g - color.g) * amount,
  b: color.b + (target.b - color.b) * amount,
})

function parse(value: unknown): Rgb | null {
  if (typeof value !== 'string' || !/^#[\da-f]{6}$/i.test(value)) return null
  return { r: parseInt(value.slice(1, 3), 16), g: parseInt(value.slice(3, 5), 16), b: parseInt(value.slice(5, 7), 16) }
}

function luminance(color: Rgb): number {
  const linear = (channel: number) => {
    const value = channel / 255
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  }
  return 0.2126 * linear(color.r) + 0.7152 * linear(color.g) + 0.0722 * linear(color.b)
}

function contrast(a: Rgb, b: Rgb): number {
  const first = luminance(a), second = luminance(b)
  return (Math.max(first, second) + 0.05) / (Math.min(first, second) + 0.05)
}

function toHsl({ r, g, b }: Rgb) {
  const red = r / 255, green = g / 255, blue = b / 255
  const max = Math.max(red, green, blue), min = Math.min(red, green, blue)
  const delta = max - min, lightness = (max + min) / 2
  if (delta === 0) return { hue: 0, saturation: 0, lightness }
  const hue = max === red ? ((green - blue) / delta + 6) % 6 : max === green ? (blue - red) / delta + 2 : (red - green) / delta + 4
  return { hue: hue * 60, saturation: delta / (1 - Math.abs(2 * lightness - 1)), lightness }
}

function fromHsl(hue: number, saturation: number, lightness: number): Rgb {
  const channel = (offset: number) => {
    const k = (offset + hue / 30) % 12
    const amplitude = saturation * Math.min(lightness, 1 - lightness)
    return 255 * (lightness - amplitude * Math.max(-1, Math.min(k - 3, 9 - k, 1)))
  }
  return { r: channel(0), g: channel(8), b: channel(4) }
}

function tokens(color: Rgb, hover: Rgb, stops: string[], dark: boolean) {
  return {
    accent: hex(color),
    accentHover: hex(hover),
    accentMuted: rgba(color, dark ? 0.18 : 0.14),
    accentSubtle: rgba(color, dark ? 0.10 : 0.07),
    borderAccent: rgba(color, 0.5),
    stageTint: stops[0]!,
    stageGradient: `linear-gradient(180deg, ${stops[0]} 0%, ${stops[1]} 38%, ${stops[2]} 72%, ${stops[3]} 100%)`,
  }
}

export const brandTheme = {
  dark: tokens({ r: 113, g: 189, b: 153 }, { r: 133, g: 198, b: 167 }, ['#1f4434', '#153024', '#0d1712', '#0a0b0b'], true),
  light: tokens({ r: 56, g: 116, b: 85 }, { r: 50, g: 104, b: 77 }, ['#e1ece5', '#ebf0eb', '#f3f4ef', '#f7f6f3'], false),
}

/** Derive stage and controls together from the original extracted color, including after a theme switch. */
export function deriveCoverTheme(source: unknown, dark: boolean) {
  const fallback = dark ? brandTheme.dark : brandTheme.light
  const parsed = parse(source)
  if (!parsed) return fallback
  const { hue, saturation, lightness } = toHsl(parsed)
  // Black, gray and near-white artwork has no useful hue. Pale chromatic art keeps a bounded tint.
  if (lightness < 0.06 || lightness > 0.97 || saturation < 0.10) return fallback

  const stageSaturation = clamp(saturation + 0.12, 0.42, 0.64)
  const stageTop = dark ? fromHsl(hue, stageSaturation, 0.19) : fromHsl(hue, clamp(saturation, 0.18, 0.32), 0.91)
  const stops = dark
    ? [hex(stageTop), hex(fromHsl(hue, stageSaturation * 0.92, 0.125)), hex(fromHsl(hue, stageSaturation * 0.6, 0.065)), '#0a0b0b']
    : [hex(stageTop), hex(mix(stageTop, { r: 247, g: 246, b: 243 }, 0.45)), '#f3f4ef', '#f7f6f3']
  const background = parse(stops[0])!
  const target = dark ? { r: 255, g: 255, b: 255 } : { r: 0, g: 0, b: 0 }
  let color = dark
    ? fromHsl(hue, clamp(saturation * 0.86, 0.30, 0.62), Math.max(lightness, 0.56))
    : parsed
  // Check rounded output against both the stage top and button foreground/surface end.
  const otherBackground = dark ? { r: 7, g: 20, b: 14 } : { r: 247, g: 246, b: 243 }
  for (let step = 0; step < 100; step++) {
    const rounded = parse(hex(color))!
    if (contrast(rounded, background) >= 4.5 && contrast(rounded, otherBackground) >= 4.5 && (dark || contrast(rounded, { r: 255, g: 255, b: 255 }) >= 4.5)) break
    color = mix(color, target, 0.025)
  }
  return tokens(color, mix(color, target, dark ? 0.14 : 0.10), stops, dark)
}
