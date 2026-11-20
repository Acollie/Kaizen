import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import type { FileDiff } from '../../types';
import { withheldReason } from './withheld';
import { WithheldNotice } from './WithheldNotice';

function generatedFile(reasons: string[]): FileDiff {
  return {
    path: 'web/review/package-lock.json', old_path: 'web/review/package-lock.json', status: 'added', language: '',
    additions: 4000, deletions: 0, binary: false, generated: true, hunks: [], line_count: 4000, risk_score: 0,
    risk_reasons: reasons,
  } as FileDiff;
}

const LOCKFILE = 'lockfile: diff not shown or sent to the AI; check dependency changes in the manifest';
const MINIFIED = 'minified content (nearly all added lines are very long): diff not shown or sent to the AI';

describe('withheldReason', () => {
  it('extracts the cause and the advice', () => {
    expect(withheldReason(generatedFile([LOCKFILE]))).toEqual({ reason: 'lockfile', hint: 'check dependency changes in the manifest' });
    expect(withheldReason(generatedFile(['CI workflow', MINIFIED]))).toEqual({
      reason: 'minified content (nearly all added lines are very long)', hint: '',
    });
  });

  it('falls back to a generic reason', () => {
    expect(withheldReason(generatedFile([]))).toEqual({ reason: 'generated file', hint: '' });
  });
});

describe('WithheldNotice', () => {
  it('says the diff was withheld and offers to show the file', () => {
    const onShow = vi.fn();
    render(<WithheldNotice diff={generatedFile([LOCKFILE])} shown={false} loading={false} onShow={onShow} />);
    expect(screen.getByText('Diff withheld: lockfile. Not shown to the AI.')).toBeTruthy();
    expect(screen.getByText('Check dependency changes in the manifest.')).toBeTruthy();
    expect(screen.queryByText(/No textual changes/)).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Show file anyway' }));
    expect(onShow).toHaveBeenCalledOnce();
  });

  it('keeps the notice once the head version is shown', () => {
    render(<WithheldNotice diff={generatedFile([MINIFIED])} shown loading={false} onShow={() => {}} />);
    expect(screen.getByText(/Diff withheld: minified content/)).toBeTruthy();
    expect(screen.getByText(/Showing the head version, read-only/)).toBeTruthy();
    expect(screen.queryByRole('button')).toBeNull();
  });
});
