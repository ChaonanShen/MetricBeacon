import type { Session, SessionPage } from '../api/resource';

export function flattenSessionPages(pages: SessionPage[]): Session[] {
  const seen = new Set<string>();
  const sessions: Session[] = [];
  for (const page of pages) {
    for (const session of page.items) {
      if (seen.has(session.id)) continue;
      seen.add(session.id);
      sessions.push(session);
    }
  }
  return sessions;
}

export function deriveSessionTitle(message: string): string {
  const normalized = message.trim().replace(/\s+/gu, ' ');
  const characters = Array.from(normalized);
  return characters.length <= 50 ? normalized : `${characters.slice(0, 49).join('')}…`;
}
