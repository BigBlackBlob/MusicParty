---
version: "alpha"
name: "MusicParty Desktop"
description: "A dark, restrained, album-art-driven desktop control console for a shared multiplayer music room."
colors:
  primary: "#D3C2F3"
  background: "#121212"
  foreground: "#F0F0F0"
  muted: "#8A8A8A"
  border: "#303033"
  surface:
    background: "#121212"
    panel: "#1A1A1A"
    stage: "#222222"
    raised: "#2A2A2A"
    elevated: "#333333"
    overlay: "#0B0B0C"
  text:
    primary: "#F0F0F0"
    secondary: "#B8B8B8"
    tertiary: "#8A8A8A"
    inverse: "#F9F9FA"
    disabled: "#5F5F64"
  accent:
    primary: "#D3C2F3"
    hover: "#E0D4F7"
    muted: "#3A3148"
    subtle: "#241F2B"
    focus: "#B99DFF"
  semantic:
    success: "#22C55E"
    warning: "#EAB308"
    danger: "#EF4444"
    info: "#A8B7FF"
  platform:
    netease: "#D84E4E"
    bilibili: "#6DBCEB"
    navidrome: "#D3C2F3"
  border:
    default: "#303033"
    subtle: "#242426"
    strong: "#48484D"
    accent: "#8E7EAA"
  chart:
    like: "#D3C2F3"
    queue: "#A8B7FF"
    listener: "#22C55E"
    error: "#EF4444"
typography:
  fontFamily:
    sans: "Geist, HarmonyOS Sans SC, Noto Sans SC, PingFang SC, Microsoft YaHei UI, system-ui, sans-serif"
    mono: "JetBrains Mono, HarmonyOS Sans SC, Noto Sans SC, ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
  fontSize:
    display: "36px"
    title: "24px"
    section: "16px"
    body: "14px"
    compact: "13px"
    caption: "12px"
    micro: "10px"
  lineHeight:
    display: "1.18"
    title: "1.25"
    section: "1.35"
    body: "1.5"
    compact: "1.4"
    caption: "1.45"
    micro: "1.2"
  fontWeight:
    regular: "400"
    medium: "500"
    semibold: "600"
    bold: "700"
    black: "900"
  letterSpacing:
    normal: "0"
    label: "0.08em"
    micro: "0.14em"
spacing:
  base: "8px"
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
  xxl: "48px"
  panelPadding: "16px"
  stagePadding: "40px"
  toolbarGap: "8px"
  listGap: "6px"
  controlGap: "12px"
rounded:
  xs: "4px"
  sm: "6px"
  md: "8px"
  lg: "12px"
  xl: "16px"
  full: "9999px"
shadows:
  panel: "0 16px 36px rgba(0, 0, 0, 0.22)"
  floating: "0 24px 60px rgba(0, 0, 0, 0.36)"
  cover: "0 32px 90px rgba(0, 0, 0, 0.44)"
  toast: "0 20px 40px rgba(0, 0, 0, 0.28)"
  focus: "0 0 0 3px rgba(211, 194, 243, 0.22)"
elevation:
  base: "0"
  panel: "1"
  dock: "2"
  overlay: "3"
  modal: "4"
motion:
  duration:
    press: "100ms"
    exit: "140ms"
    enter: "220ms"
    reveal: "360ms"
    ambient: "700ms"
  easing:
    out: "cubic-bezier(0.16, 1, 0.3, 1)"
    in: "cubic-bezier(0.4, 0, 1, 1)"
    standard: "cubic-bezier(0.2, 0, 0, 1)"
layout:
  desktop:
    viewportHeight: "100dvh"
    topBarHeight: "64px"
    leftRailWidth: "260px"
    rightPanelWidth: "320px"
    bottomDockHeight: "96px"
    maxStageWidth: "1500px"
    coverSizeMin: "360px"
    coverSizeMax: "600px"
    queueItemHeight: "56px"
    compactQueueItemHeight: "44px"
    iconButtonSize: "36px"
    primaryButtonHeight: "36px"
components:
  appShell:
    backgroundColor: "{colors.surface.background}"
    textColor: "{colors.text.primary}"
    layout: "desktop four-zone shell with top bar, left rail, center stage, right work panel, and player dock"
  topBar:
    height: "{layout.desktop.topBarHeight}"
    backgroundColor: "{colors.surface.panel}"
    borderColor: "{colors.border.default}"
    paddingInline: "24px"
  leftRail:
    width: "{layout.desktop.leftRailWidth}"
    backgroundColor: "{colors.surface.panel}"
    borderColor: "{colors.border.default}"
  rightPanel:
    width: "{layout.desktop.rightPanelWidth}"
    backgroundColor: "{colors.surface.panel}"
    borderColor: "{colors.border.default}"
  centerStage:
    backgroundColor: "{colors.surface.stage}"
    padding: "{spacing.stagePadding}"
    maxWidth: "{layout.desktop.maxStageWidth}"
  panel:
    backgroundColor: "{colors.surface.panel}"
    borderColor: "{colors.border.default}"
    rounded: "{rounded.md}"
    shadow: "{shadows.panel}"
  playerDock:
    height: "{layout.desktop.bottomDockHeight}"
    backgroundColor: "{colors.surface.panel}"
    borderColor: "{colors.border.default}"
  coverArtwork:
    rounded: "{rounded.lg}"
    shadow: "{shadows.cover}"
    backgroundColor: "{colors.surface.elevated}"
  iconButton:
    size: "{layout.desktop.iconButtonSize}"
    rounded: "{rounded.sm}"
    backgroundColor: "transparent"
    textColor: "{colors.text.secondary}"
  iconButton-hover:
    size: "{layout.desktop.iconButtonSize}"
    rounded: "{rounded.sm}"
    backgroundColor: "{colors.surface.raised}"
    textColor: "{colors.text.primary}"
  primaryButton:
    height: "{layout.desktop.primaryButtonHeight}"
    rounded: "{rounded.sm}"
    backgroundColor: "{colors.accent.primary}"
    textColor: "{colors.text.inverse}"
  primaryButton-hover:
    height: "{layout.desktop.primaryButtonHeight}"
    rounded: "{rounded.sm}"
    backgroundColor: "{colors.accent.hover}"
    textColor: "{colors.text.inverse}"
  trackListItem:
    height: "{layout.desktop.queueItemHeight}"
    rounded: "{rounded.md}"
    backgroundColor: "transparent"
    textColor: "{colors.text.primary}"
  trackListItem-hover:
    height: "{layout.desktop.queueItemHeight}"
    rounded: "{rounded.md}"
    backgroundColor: "{colors.surface.stage}"
    textColor: "{colors.text.primary}"
  trackListItem-active:
    height: "{layout.desktop.queueItemHeight}"
    rounded: "{rounded.md}"
    backgroundColor: "{colors.accent.subtle}"
    textColor: "{colors.accent.primary}"
  segmentedControl:
    height: "36px"
    rounded: "{rounded.sm}"
    backgroundColor: "{colors.surface.raised}"
    textColor: "{colors.text.secondary}"
  segmentedControl-active:
    height: "36px"
    rounded: "{rounded.sm}"
    backgroundColor: "{colors.surface.elevated}"
    textColor: "{colors.text.primary}"
  input:
    height: "40px"
    rounded: "{rounded.sm}"
    backgroundColor: "{colors.surface.raised}"
    textColor: "{colors.text.primary}"
  toast:
    backgroundColor: "{colors.surface.raised}"
    textColor: "{colors.text.primary}"
    rounded: "{rounded.lg}"
    shadow: "{shadows.toast}"
---

# MusicParty Desktop Design System

This document defines the desktop web visual system for MusicParty. It is intentionally scoped to the desktop experience only. The desktop product should feel like a shared music room control console: calm enough to stay open for hours, precise enough to manage a live queue, and atmospheric enough that the current song feels emotionally present.

## Design Intent

MusicParty is a multiplayer music room, not a personal streaming app. The interface should support three simultaneous desktop behaviors: watching the current track, managing the queue, and quickly searching or chatting without losing context. The design should feel like a professional audio control surface softened by album-art atmosphere.

The existing visual identity is dark, minimal, immersive, and cover-driven. Preserve that direction. The redesign should refine it into a more stable desktop system with stronger layout discipline, clearer hierarchy, and less incidental UI noise.

## Physical Scene

The primary scene is a group of friends in a late-night remote room. The browser window is open on a laptop, desktop monitor, or shared screen. Ambient light is low, users are chatting or doing other things, and MusicParty sits in the background as a synchronized room object. The UI should not shout for attention; it should quietly expose control when needed.

## Color Strategy

Use a restrained dark palette with album-driven accents. The base surface is near-black but not pure black. Panels are separated with subtle luminance changes and low-contrast borders. The accent color is a soft lavender fallback, but the live product may derive accent from album artwork. Accent should be used sparingly: active queue item, primary action, progress fill, focus state, and current playback feedback.

Do not flood the whole desktop shell with the accent color. Do not use generic blue-purple SaaS gradients. Do not make every panel translucent. Atmospheric cover color belongs primarily in the center stage around the current artwork.

## Desktop Layout

The desktop shell should use a stable five-region structure:

- A 64px top bar for product identity, connection state, user controls, theme controls, lite mode, and search entry.
- A 260px left rail for room members and lightweight room presence.
- A center stage for the current track, cover artwork, lyrics, and playback focus.
- A 320px right panel for queue-first work: queue, liked tracks, and search results can share this region through tabs or segmented navigation.
- A 96px bottom dock for persistent playback controls when controls are not fully embedded in the center stage.

The center stage should be the emotional focus. The left rail and right panel should feel like tools attached to the room, not competing cards. Avoid nested cards. Use full-height panels, clear panel headers, and consistent list geometry.

## Desktop Information Hierarchy

The current track always has the highest emotional priority. The queue has the highest operational priority. Search is a fast action layer. Chat and room members are social context.

Recommended priority order:

1. Current playing artwork, title, artist, platform, requester, and playback state.
2. Queue state and upcoming songs.
3. Search and add-to-queue actions.
4. Like state and liked song export.
5. Online users and chat.
6. Theme, lite mode, account, and settings.

## Components

### Top Bar

The top bar should be quiet and exact. Use uppercase product identity sparingly. Connection and room state should be visible but compact. Search should be an obvious action, but it should not dominate the bar.

### Center Stage

Use album artwork as the visual anchor. The cover may sit alone when no lyrics are available, or share the stage with lyrics when lyrics exist. The stage can use blurred album color in the background, but always under a dark overlay so text and controls remain readable.

The cover should have a small radius, a strong but soft shadow, and no excessive decorative frame. Interaction on the cover can support liking or reaction, but the affordance must not obscure the playback state.

### Right Work Panel

The right panel should be queue-first. Queue, liked tracks, and search should share the same list grammar: fixed item height, cover thumbnail, title, artist, metadata, and a fixed-width action zone. Switching between queue and liked tracks must not change panel width or cause the header to reflow.

Export liked tracks as a compact toolbar action or overflow item. Never use a large export button that changes the width relationship between queue and liked views.

### Left Rail

The left rail should show room members and presence. It should be compact, calm, and secondary. Online count can use a small success dot. Do not over-style user rows with heavy cards.

### Player Dock

The dock should provide persistent controls: play/pause, next, progress, volume, like, platform context, and playback error state. It should be visually attached to the shell, not a floating card. Use fixed heights and fixed control sizes.

### Lists

Track lists should be compact and stable. Use 56px items by default and 44px only for dense secondary lists. Hover may reveal actions but must reserve enough space so the row does not shift. Active rows use subtle accent background and accent border, not a loud side stripe.

### Buttons

Icon buttons should be 36px square with 6px radius. Primary buttons are compact, 36px high, and use the accent color. Avoid pill-shaped text buttons unless the control is a filter or segmented option. Use icons for playback, queue actions, export, settings, and theme controls.

## Typography

Use a modern sans-serif with good Chinese support. The desktop UI should feel compact and engineered. Use strong weight contrast for product identity and current song title, but keep most operational text between 12px and 14px.

Avoid negative letter spacing. Use uppercase micro labels only for short section labels such as ROOM MEMBERS, QUEUE, LIKED, or SEARCH. Body text should remain normal case for readability.

## Motion

Motion should be restrained and functional. Use short fade and translate transitions for panels, toasts, hover affordances, and search reveal. Album background changes may transition more slowly, around 700ms, so the stage feels ambient rather than jumpy.

Do not animate layout dimensions in queue or liked views. Do not use bounce or elastic motion. Respect reduced motion preferences.

## Error, Loading, And Empty States

Desktop errors should be visible where the action happened. Search errors belong inside the search panel and may also trigger a toast. Playback errors belong near the player dock or center stage. Platform errors should distinguish unavailable, unauthorized, and upstream failure states when possible.

Loading states should be quiet skeletons or inline spinners. Empty queue and empty liked views should be compact, not hero illustrations.

## Desktop-Specific UX Rules

The desktop redesign must solve layout instability:

- Queue and liked views must keep identical panel width.
- Toolbar actions must have fixed sizes.
- Track row actions must not appear in a way that changes row width.
- The search panel must not push the current playing stage out of frame.
- The left rail can collapse below medium desktop widths, but the center stage and right work panel must remain usable.
- The product should be comfortable at 1024px, 1440px, and 1920px widths.

## What To Avoid

Avoid a generic personal music player layout. Avoid marketing-style hero sections. Avoid identical card grids. Avoid nested cards. Avoid heavy glassmorphism. Avoid decorative gradient text. Avoid side-stripe accent borders. Avoid large rounded pill controls everywhere. Avoid mobile bottom navigation on desktop.

The final desktop design should feel like a precise, immersive shared music console: current track in the center, queue at hand, search one action away, and the room quietly alive around it.
