import { describe, expect, it } from 'vitest';

import { deriveSessionTitle, flattenSessionPages } from './session-list';

const session = (id: string) => ({ id }) as never;

describe('session list helpers', () => {
  it('flattens keyset pages and keeps the first occurrence of a moved Session', () => {
    expect(flattenSessionPages([
      { items: [session('new'), session('moved')], nextPageToken: 'next' } as never,
      { items: [session('moved'), session('old')], nextPageToken: null } as never,
    ]).map(({ id }) => id)).toEqual(['new', 'moved', 'old']);
  });

  it('derives a compact title without splitting Unicode code points', () => {
    expect(deriveSessionTitle('  查看 CPU\n\t和内存  ')).toBe('查看 CPU 和内存');
    expect(deriveSessionTitle('😀'.repeat(50))).toBe('😀'.repeat(50));
    expect(deriveSessionTitle('😀'.repeat(51))).toBe(`${'😀'.repeat(49)}…`);
  });
});
