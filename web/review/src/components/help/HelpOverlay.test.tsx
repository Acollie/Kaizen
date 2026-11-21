import { describe, expect, it } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { createReviewStore } from '../../state/store';
import { StoreProvider } from '../../state/StoreProvider';
import { helpGroups } from '../../keys/keymap';
import { makeContext } from '../post/testFixtures';
import { HelpOverlay } from './HelpOverlay';
import { splitKeyLabel } from './keyLabels';

describe('splitKeyLabel', () => {
  it('splits sequences into keys and keeps named keys whole', () => {
    expect(splitKeyLabel('gT')).toEqual(['g', 'T']);
    expect(splitKeyLabel(']c')).toEqual([']', 'c']);
    expect(splitKeyLabel('Ctrl-d')).toEqual(['Ctrl-d']);
    expect(splitKeyLabel('Space')).toEqual(['Space']);
    expect(splitKeyLabel('Esc')).toEqual(['Esc']);
  });
});

describe('HelpOverlay', () => {
  it('lists every group and row when open and closes on the close button', () => {
    const store = createReviewStore(makeContext());
    render(<StoreProvider store={store}><HelpOverlay /></StoreProvider>);
    act(() => store.getState().setHelpOpen(true));
    for (const group of helpGroups()) {
      expect(screen.getByText(group.group)).toBeTruthy();
      for (const row of group.rows) expect(screen.getAllByText(row.description).length).toBeGreaterThan(0);
    }
    act(() => screen.getByRole('button', { name: 'Close', hidden: true }).click());
    expect(store.getState().ui.helpOpen).toBe(false);
  });
});
