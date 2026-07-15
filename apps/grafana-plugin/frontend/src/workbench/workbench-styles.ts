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
  container-name: mtb-workbench;
  container-type: inline-size;
  min-width: 0;
  padding: 16px;
  color: var(--mtb-text);
  font-family: var(--mtb-font);
}
.mtb-workbench *, .mtb-workbench *::before, .mtb-workbench *::after { box-sizing: border-box; }
.mtb-workbench-header { display: grid; grid-template-columns: minmax(210px, auto) minmax(0, 1fr) auto; align-items: center; gap: 20px; margin-bottom: 16px; min-width: 0; }
.mtb-workbench-heading { min-width: 0; }
.mtb-workbench-eyebrow { margin: 0 0 2px; color: var(--mtb-text-secondary); font-size: 11px; font-weight: 600; letter-spacing: .06em; text-transform: uppercase; }
.mtb-workbench-title { margin: 0; font-size: 24px; line-height: 1.2; }
.mtb-workbench-status { color: var(--mtb-text-secondary); font-size: 12px; white-space: nowrap; }
.mtb-workbench-status.tone-success { color: var(--mtb-success); }
.mtb-workbench-status.tone-warning { color: var(--mtb-warning); }
.mtb-workbench-status.tone-error { color: var(--mtb-error); }
.mtb-workbench-status.tone-info { color: var(--mtb-info); }
.mtb-product-nav { display: flex; align-items: center; gap: 4px; min-width: 0; overflow-x: auto; padding: 4px; border: 1px solid var(--mtb-border); border-radius: 8px; background: var(--mtb-surface); }
.mtb-product-nav-item { flex: 0 0 auto; padding: 8px 12px; border: 1px solid transparent; border-radius: 6px; background: transparent; color: var(--mtb-text-secondary); cursor: pointer; font: inherit; font-weight: 500; }
.mtb-product-nav-item:hover:not(:disabled) { color: var(--mtb-text); background: var(--mtb-surface-raised); }
.mtb-product-nav-item.is-current { border-color: var(--mtb-accent); color: var(--mtb-text); background: color-mix(in srgb, var(--mtb-accent) 12%, transparent); }
.mtb-product-nav-item:disabled { cursor: not-allowed; opacity: .48; }
.mtb-product-nav-item:focus-visible, .mtb-context-toggle:focus-visible { outline: 2px solid var(--mtb-accent); outline-offset: 2px; }
.mtb-workbench-layout { display: grid; grid-template-areas: 'chat' 'context' 'canvas'; grid-template-columns: minmax(0, 1fr); align-items: stretch; gap: 16px; min-width: 0; }
.mtb-workbench-chat-slot { grid-area: chat; min-width: 0; min-height: 0; }
.mtb-workbench-context-slot, .mtb-workbench-canvas-slot { min-width: 0; min-height: 0; }
.mtb-workbench-context-slot { grid-area: context; }
.mtb-workbench-canvas-slot { grid-area: canvas; display: flex; }
.mtb-chat-pane { display: flex; flex-direction: column; height: 100%; min-width: 0; min-height: 620px; overflow: hidden; border: 1px solid var(--mtb-border); border-radius: 8px; background: var(--mtb-surface); }
.mtb-chat-header { flex: 0 0 auto; display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; padding: 16px 16px 12px; border-bottom: 1px solid var(--mtb-border); }
.mtb-pane-kicker { display: block; margin-bottom: 2px; color: var(--mtb-text-secondary); font-size: 10px; font-weight: 600; letter-spacing: .08em; text-transform: uppercase; }
.mtb-chat-header h2 { margin: 0; font-size: 18px; line-height: 1.25; }
.mtb-chat-header p { margin: 3px 0 0; overflow-wrap: anywhere; color: var(--mtb-text-secondary); font-size: 12px; }
.mtb-session-menu { flex: 0 0 auto; border-bottom: 1px solid var(--mtb-border); }
.mtb-session-menu-actions { display: flex; align-items: center; gap: 8px; padding: 10px 16px; }
.mtb-session-menu-toggle { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 8px; min-width: 0; flex: 1 1 auto; padding: 7px 9px; border: 1px solid var(--mtb-border); border-radius: 6px; background: var(--mtb-surface-raised); color: var(--mtb-text); cursor: pointer; font: inherit; text-align: left; }
.mtb-session-menu-toggle:focus-visible, .mtb-session-item:focus-visible, .mtb-example-prompts button:focus-visible { outline: 2px solid var(--mtb-accent); outline-offset: 2px; }
.mtb-session-menu-current { min-width: 0; overflow: hidden; color: var(--mtb-text-secondary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.mtb-session-list { display: grid; gap: 6px; max-height: 210px; overflow-y: auto; overflow-x: hidden; padding: 0 16px 12px; scrollbar-gutter: stable; }
.mtb-session-item { display: flex; flex-direction: column; gap: 3px; width: 100%; min-width: 0; padding: 8px 10px; border: 1px solid transparent; border-radius: 6px; background: transparent; color: inherit; text-align: left; cursor: pointer; }
.mtb-session-item:hover:not(:disabled) { background: var(--mtb-surface-raised); }
.mtb-session-item.is-selected { border-color: var(--mtb-border-medium); background: color-mix(in srgb, var(--mtb-accent) 8%, var(--mtb-surface)); }
.mtb-session-item:disabled { cursor: not-allowed; opacity: .65; }
.mtb-session-title { display: -webkit-box; min-width: 0; overflow: hidden; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow-wrap: anywhere; font-weight: 500; }
.mtb-session-time { overflow: hidden; color: var(--mtb-text-secondary); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.mtb-chat-timeline { flex: 1 1 auto; min-height: 0; overflow-y: auto; overflow-x: hidden; padding: 16px; scrollbar-gutter: stable; }
.mtb-chat-timeline > * + * { margin-top: 12px; }
.mtb-chat-empty { padding: 16px; border: 1px dashed var(--mtb-border-medium); border-radius: 8px; background: color-mix(in srgb, var(--mtb-surface-raised) 65%, transparent); }
.mtb-chat-empty p { margin: 6px 0 12px; color: var(--mtb-text-secondary); }
.mtb-example-prompts { display: grid; gap: 7px; }
.mtb-example-prompts button { padding: 8px 10px; border: 1px solid var(--mtb-border); border-radius: 6px; background: var(--mtb-surface); color: var(--mtb-text); cursor: pointer; font: inherit; text-align: left; }
.mtb-example-prompts button:hover { border-color: var(--mtb-accent); }
.mtb-message { margin: 0; padding: 10px 12px; border: 1px solid var(--mtb-border); border-radius: 8px; overflow-wrap: anywhere; white-space: pre-wrap; user-select: text; }
.mtb-message.is-user { margin-left: 18px; border-color: color-mix(in srgb, var(--mtb-accent) 45%, var(--mtb-border)); background: color-mix(in srgb, var(--mtb-accent) 9%, var(--mtb-surface)); }
.mtb-message.is-assistant { margin-right: 10px; background: var(--mtb-surface-raised); }
.mtb-chat-composer { flex: 0 0 auto; padding: 12px 16px 16px; border-top: 1px solid var(--mtb-border); background: var(--mtb-surface); }
.mtb-chat-composer > * + * { margin-top: 8px; }
.mtb-composer-actions { display: flex; align-items: center; gap: 8px; }
.mtb-inline-error, .mtb-inline-notice, .mtb-task-status, .mtb-muted { margin: 0; font-size: 12px; }
.mtb-inline-error { color: var(--mtb-error); }
.mtb-inline-notice { color: var(--mtb-info); }
.mtb-task-status, .mtb-muted { color: var(--mtb-text-secondary); }
.mtb-context-pane { height: 100%; min-width: 0; overflow: hidden; border: 1px solid var(--mtb-border); border-radius: 8px; background: var(--mtb-surface); }
.mtb-context-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 16px; }
.mtb-context-kicker { display: block; margin-bottom: 2px; color: var(--mtb-text-secondary); font-size: 10px; font-weight: 600; letter-spacing: .08em; text-transform: uppercase; }
.mtb-context-heading h2 { margin: 0; font-size: 18px; line-height: 1.25; }
.mtb-context-toggle { display: none; align-items: center; justify-content: center; width: 32px; height: 32px; border: 1px solid var(--mtb-border); border-radius: 6px; background: var(--mtb-surface-raised); color: var(--mtb-text); cursor: pointer; font: inherit; }
.mtb-context-details { padding: 0 16px 16px; }
.mtb-context-details h3 { margin: 0 0 4px; color: var(--mtb-text-secondary); font-size: 12px; font-weight: 600; text-transform: uppercase; }
.mtb-context-session-title { margin: 0; overflow-wrap: anywhere; font-size: 15px; font-weight: 600; }
.mtb-context-list { display: grid; gap: 0; margin: 16px 0; }
.mtb-context-list > div { display: grid; grid-template-columns: minmax(88px, .8fr) minmax(0, 1.2fr); gap: 8px; padding: 9px 0; border-top: 1px solid var(--mtb-border); }
.mtb-context-list dt { color: var(--mtb-text-secondary); }
.mtb-context-list dd { min-width: 0; margin: 0; overflow-wrap: anywhere; text-align: right; }
.mtb-context-badge { display: inline-flex; padding: 2px 8px; border: 1px solid var(--mtb-border-medium); border-radius: 999px; font-size: 12px; }
.mtb-context-badge.tone-success { color: var(--mtb-success); }
.mtb-context-badge.tone-warning { color: var(--mtb-warning); }
.mtb-context-badge.tone-error { color: var(--mtb-error); }
.mtb-context-badge.tone-info { color: var(--mtb-info); }
.mtb-context-note { padding: 12px; border: 1px solid var(--mtb-border); border-radius: 6px; background: var(--mtb-surface-raised); color: var(--mtb-text-secondary); }
.mtb-context-note strong { color: var(--mtb-text); }
.mtb-context-note p { margin: 4px 0 0; }
.mtb-visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); clip-path: inset(50%); white-space: nowrap; }
@container mtb-workbench (min-width: 1024px) {
  .mtb-workbench-layout { grid-template-areas: 'canvas context chat'; grid-template-columns: minmax(0, 1fr) 40px minmax(320px, 360px); height: calc(100dvh - 128px); min-height: 480px; }
  .mtb-workbench-chat-slot { overflow: hidden; }
  .mtb-chat-pane { min-height: 0; }
  .mtb-workbench-context-slot.is-expanded { width: 260px; }
  .mtb-workbench-layout:has(.mtb-workbench-context-slot.is-expanded) { grid-template-columns: minmax(0, 1fr) 260px minmax(320px, 360px); }
  .mtb-context-pane:not(.is-expanded) .mtb-context-heading, .mtb-context-pane:not(.is-expanded) .mtb-context-details { display: none; }
  .mtb-context-pane:not(.is-expanded) .mtb-context-header { justify-content: center; padding: 8px 4px; }
  .mtb-context-toggle { display: inline-flex; }
}
@container mtb-workbench (min-width: 1366px) {
  .mtb-workbench-layout, .mtb-workbench-layout:has(.mtb-workbench-context-slot.is-expanded) { grid-template-columns: minmax(560px, 1fr) minmax(240px, 260px) minmax(340px, 360px); }
  .mtb-context-pane:not(.is-expanded) .mtb-context-heading, .mtb-context-pane:not(.is-expanded) .mtb-context-details { display: block; }
  .mtb-context-pane:not(.is-expanded) .mtb-context-header { justify-content: space-between; padding: 16px; }
  .mtb-context-toggle { display: none; }
}
@container mtb-workbench (max-width: 900px) {
  .mtb-workbench-header { grid-template-columns: 1fr; align-items: start; gap: 12px; }
  .mtb-workbench-status { white-space: normal; }
}
`;
}
