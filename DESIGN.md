# Design

## Color

Strategy: **Restrained** — tinted neutrals + one accent ≤10%. Product dashboard default.

Mood: "工程师的精密仪器面板 — 冷静、专注、高对比度信息层次，像 Linear 和 Vercel 的克制感"

```css
:root {
  /* Primary — olive-teal, professional and grounded */
  --color-primary: oklch(0.550 0.100 115.0);
  --color-primary-hover: oklch(0.500 0.110 115.0);
  --color-primary-active: oklch(0.460 0.100 115.0);
  --color-primary-light: oklch(0.920 0.030 115.0);

  /* Background — pure white */
  --color-bg: oklch(1.000 0.000 0);

  /* Surface — very subtle cool gray for cards/panels */
  --color-surface: oklch(0.975 0.000 0);
  --color-surface-hover: oklch(0.960 0.000 0);
  --color-surface-border: oklch(0.900 0.000 0);

  /* Ink — near-black with slight cool cast */
  --color-ink: oklch(0.150 0.005 260.0);
  --color-ink-secondary: oklch(0.450 0.005 260.0);

  /* Muted — secondary text */
  --color-muted: oklch(0.550 0.005 260.0);

  /* Accent — warm amber for badges, status, links */
  --color-accent: oklch(0.650 0.150 55.0);
  --color-accent-light: oklch(0.920 0.040 55.0);

  /* Semantic */
  --color-success: oklch(0.600 0.140 145.0);
  --color-warning: oklch(0.700 0.140 80.0);
  --color-error: oklch(0.550 0.180 25.0);
  --color-info: oklch(0.600 0.100 240.0);

  /* Sidebar */
  --color-sidebar-bg: oklch(0.160 0.010 260.0);
  --color-sidebar-text: oklch(0.850 0.000 0);
  --color-sidebar-text-muted: oklch(0.600 0.000 0);
  --color-sidebar-active: oklch(0.550 0.100 115.0);
}
```

## Typography

Single family: **Inter** with system-ui fallback. Fixed rem scale (ratio 1.2).

```css
:root {
  --font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', 'SF Mono', monospace;

  /* Scale (1.2 ratio) */
  --text-xs: 0.694rem;    /* 11px */
  --text-sm: 0.833rem;    /* 13px */
  --text-base: 1rem;      /* 16px */
  --text-lg: 1.2rem;      /* 19px */
  --text-xl: 1.44rem;     /* 23px */
  --text-2xl: 1.728rem;   /* 28px */
  --text-3xl: 2.074rem;   /* 33px */

  /* Weights */
  --font-normal: 400;
  --font-medium: 500;
  --font-semibold: 600;
  --font-bold: 700;

  /* Line heights */
  --leading-tight: 1.25;
  --leading-normal: 1.5;
  --leading-relaxed: 1.625;
}
```

## Spacing

8px base grid.

```css
:root {
  --space-1: 0.25rem;   /* 4px */
  --space-2: 0.5rem;    /* 8px */
  --space-3: 0.75rem;   /* 12px */
  --space-4: 1rem;      /* 16px */
  --space-5: 1.25rem;   /* 20px */
  --space-6: 1.5rem;    /* 24px */
  --space-8: 2rem;      /* 32px */
  --space-10: 2.5rem;   /* 40px */
  --space-12: 3rem;     /* 48px */
  --space-16: 4rem;     /* 64px */
}
```

## Radius

```css
:root {
  --radius-sm: 4px;
  --radius-md: 6px;
  --radius-lg: 8px;
  --radius-xl: 12px;
}
```

## Shadows

```css
:root {
  --shadow-sm: 0 1px 2px oklch(0.000 0.000 0 / 0.05);
  --shadow-md: 0 2px 4px oklch(0.000 0.000 0 / 0.06), 0 1px 2px oklch(0.000 0.000 0 / 0.04);
  --shadow-lg: 0 4px 12px oklch(0.000 0.000 0 / 0.08), 0 2px 4px oklch(0.000 0.000 0 / 0.04);
}
```

## Motion

Product-appropriate: fast, state-driven, no choreography.

```css
:root {
  --duration-fast: 120ms;
  --duration-normal: 200ms;
  --duration-slow: 300ms;
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);  /* expo out */
  --ease-in-out: cubic-bezier(0.65, 0, 0.35, 1);
}

@media (prefers-reduced-motion: reduce) {
  :root {
    --duration-fast: 0ms;
    --duration-normal: 0ms;
    --duration-slow: 0ms;
  }
}
```

## Layout

- Sidebar: 240px expanded, 64px collapsed
- Content max-width: none (fills available space)
- Content padding: var(--space-6)
- Breakpoint for sidebar collapse: 1280px
- Table density: compact rows (40px height), comfortable (48px)

## Components

Using Ant Design 5.x with custom theme token overrides to match the design system. Key overrides:

- `colorPrimary`: mapped to --color-primary
- `borderRadius`: 6px
- `fontFamily`: Inter stack
- `colorBgContainer`: --color-surface
- `colorText`: --color-ink
- `colorTextSecondary`: --color-muted

All interactive components must cover: default, hover, focus, active, disabled, loading states.
Skeleton loading for async content. Empty states with guidance text.
