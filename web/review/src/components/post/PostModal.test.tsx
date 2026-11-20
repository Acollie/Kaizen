import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { createReviewStore } from '../../state/store';
import { StoreProvider } from '../../state/StoreProvider';
import * as api from '../../lib/api';
import { makeContext, makeFinding } from './testFixtures';
import { PostModal } from './PostModal';

function setup({ accept = [] as string[] } = {}) {
  const store = createReviewStore(makeContext());
  act(() => {
    store.getState().applyEvent({ seq: 1, type: 'finding', data: makeFinding({ id: 'inline', line: 2, title: 'On the diff' }) } as never);
    store.getState().applyEvent({ seq: 2, type: 'finding', data: makeFinding({ id: 'body', line: 0, title: 'File level' }) } as never);
    for (const id of accept) store.getState().acceptFinding(id);
  });
  render(<StoreProvider store={store}><PostModal /></StoreProvider>);
  act(() => store.getState().setPostOpen(true));
  return store;
}

const button = (name: string | RegExp) => screen.getByRole('button', { name, hidden: true });

afterEach(() => vi.restoreAllMocks());

describe('PostModal', () => {
  it('posts nothing by default and explains why', () => {
    setup();
    expect(screen.getByRole('note', { hidden: true }).textContent).toContain("You haven't accepted any findings");
    expect(button('Post summary only')).toBeTruthy();
    expect(screen.getByText('2 must-fix or should-fix findings not triaged yet.')).toBeTruthy();
    expect(screen.queryByText(/AI-written text as yourself/)).toBeNull();
  });

  it('lists accepted findings and warns that AI text is posted as you', () => {
    setup({ accept: ['inline'] });
    expect(screen.getByText('Accepted findings')).toBeTruthy();
    expect(screen.getByText('On the diff')).toBeTruthy();
    expect(screen.queryByText('File level')).toBeNull();
    expect(screen.getByText('0 yours · 1 accepted · 1 inline · 0 in body')).toBeTruthy();
    expect(screen.getByText(/AI-written text as yourself/)).toBeTruthy();
    const summary = screen.getByLabelText('Summary', { selector: 'textarea' }) as HTMLTextAreaElement;
    expect(summary.value).toContain('Kaizen review: 1 should fix.');
  });

  it('posts your comments first, then the accepted findings with notes', async () => {
    const store = setup({ accept: ['inline'] });
    const post = vi.spyOn(api, 'postReview').mockResolvedValue({ url: 'https://github.com/o/r/pull/7#review', inline_comments: 2, body_findings: 0 });
    act(() => {
      store.getState().setTriageNote('inline', 'Reproduced it locally.');
      store.getState().saveComment({ file: 'pkg/a.go', line: 2, body: 'Why a var?' });
    });
    expect(screen.getByText('Your comments')).toBeTruthy();
    fireEvent.click(button('Post 1 comment and 1 accepted finding'));
    await waitFor(() => expect(screen.getByText('Review posted')).toBeTruthy());
    const sent = post.mock.calls[0]?.[0].findings ?? [];
    expect(sent.map((finding) => finding.source)).toEqual(['you', undefined]);
    expect(sent[0]?.id.startsWith('c-')).toBe(true);
    expect(sent[1]?.body).toContain('**Reviewer note:** Reproduced it locally.');
    expect(screen.getByRole('link', { hidden: true }).getAttribute('href')).toBe('https://github.com/o/r/pull/7#review');
  });

  it('un-accepting in the dialog removes the finding', () => {
    const store = setup({ accept: ['inline'] });
    fireEvent.click(button('Un-accept On the diff'));
    expect(store.getState().triage.inline).toBeUndefined();
  });

  it('shows the error and offers a retry', async () => {
    setup({ accept: ['inline'] });
    vi.spyOn(api, 'postReview').mockRejectedValue(new Error('gh: not authenticated'));
    fireEvent.click(button('Post 1 accepted finding'));
    await waitFor(() => expect(screen.getByRole('alert', { hidden: true }).textContent).toContain('gh: not authenticated'));
    expect(button('Retry')).toBeTruthy();
  });

  it('does not link unsafe URLs', async () => {
    setup({ accept: ['inline'] });
    vi.spyOn(api, 'postReview').mockResolvedValue({ url: 'javascript:alert(1)', inline_comments: 0, body_findings: 0 });
    fireEvent.click(button('Post 1 accepted finding'));
    await waitFor(() => expect(screen.getByText('Review posted')).toBeTruthy());
    expect(screen.queryByRole('link', { hidden: true })).toBeNull();
  });
});
