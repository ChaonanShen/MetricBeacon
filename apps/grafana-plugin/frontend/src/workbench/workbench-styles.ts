import type { GrafanaTheme2 } from '@grafana/data';

export function workbenchThemeCSS(theme: GrafanaTheme2): string {
  const accent = theme.colors.mode === 'dark' ? '#ff780a' : '#b54708';
  return `
.mtb-workbench {
  --mtb-bg: ${theme.colors.background.canvas};
  --mtb-surface: ${theme.colors.background.primary};
  --mtb-surface-raised: ${theme.colors.background.secondary};
  --mtb-text: ${theme.colors.text.primary};
  --mtb-text-secondary: ${theme.colors.text.secondary};
  --mtb-border: ${theme.colors.border.weak};
  --mtb-border-medium: ${theme.colors.border.medium};
  --mtb-accent: ${accent};
  --mtb-success: ${theme.colors.success.text};
  --mtb-warning: ${theme.colors.warning.text};
  --mtb-error: ${theme.colors.error.text};
  --mtb-info: ${theme.colors.info.text};
  --mtb-font: ${theme.typography.fontFamily};
  box-sizing: border-box;
  min-width: 0;
  padding: 16px;
  color: var(--mtb-text);
  font-family: var(--mtb-font);
}
.mtb-workbench *, .mtb-workbench *::before, .mtb-workbench *::after { box-sizing: border-box; }
.mtb-workbench-header { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; margin-bottom: 16px; min-width: 0; }
.mtb-workbench-eyebrow { margin: 0 0 2px; color: var(--mtb-text-secondary); font-size: 11px; font-weight: 600; letter-spacing: .06em; text-transform: uppercase; }
.mtb-workbench-title { margin: 0; font-size: 24px; line-height: 1.2; }
.mtb-workbench-status { color: var(--mtb-text-secondary); font-size: 12px; white-space: nowrap; }
.mtb-workbench-layout { display: flex; align-items: stretch; gap: 16px; min-width: 0; }
.mtb-workbench-session-slot { flex: 0 1 260px; width: 260px; min-width: 0; }
.mtb-workbench-conversation-slot { flex: 1 1 0; width: auto; min-width: 320px; max-width: 420px; }
.mtb-workbench-canvas-slot { display: flex; flex: 3 1 0; min-width: 0; }
@media (min-width: 1200px) { .mtb-workbench-layout { height: calc(100dvh - 112px); } }
@media (max-width: 1199px) {
  .mtb-workbench-layout { flex-direction: column; }
  .mtb-workbench-session-slot { width: 100%; height: 280px; flex-basis: auto; }
  .mtb-workbench-conversation-slot { width: 100%; max-width: none; min-width: 0; }
  .mtb-workbench-canvas-slot { width: 100%; }
}
`;
}
