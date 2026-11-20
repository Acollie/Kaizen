import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { createReviewStore } from '../../state/store';
import { StoreProvider } from '../../state/StoreProvider';
import * as api from '../../lib/api';
import { makeContext, makeFinding } from './testFixtures';
import { PostModal } from './PostModal';

function setup() {
  const store = createReviewStore(makeContext());
  act(() => {
    store.getState().applyEvent({ seq: 1, type: 'finding', data: makeFinding({ id: 'inline', line: 2, title: 'On the diff' }) } as never);
    store.getState().applyEvent({ seq: 2, type: 'finding', data: makeFinding({ id: 'body', line: 0, title: 'File level' }) } as never);
  });
  render(<StoreProvider store={store}><PostModal /></StoreProvider>);
  act(() => store.getState().setPostOpen(true));
  return store;
}

afterEach(() => vi.restoreAllMocks());

describe('PostModal', () => {
  it('prefills a generated summary and splits inline from body findings', () => {
    setup();
    const summary = screen.getByLabelText('Summary', { selector: 'textarea' }) as HTMLTextAreaElement;
    expect(summary.value).toContain('Kaizen review: 2 should fix.');
    expect(screen.getByText('Inline comments')).toBeTruthy();
    expect(screen.getByText('Review body')).toBeTruthy();
    expect(screen.getByText('2 selected · 1 inline · 1 in body')).toBeTruthy();
    expect(screen.getByText('This posts a COMMENT review to GitHub as you.')).toBeTruthy();
  });

  it('posts only the included findings and shows the result link', async () => {
    const store = setup();
    const post = vi.spyOn(api, 'postReview').mockResolvedValue({ url: 'https://github.com/o/r/pull/7#review', inline_comments: 1, body_findings: 0 });
    fireEvent.click(screen.getByLabelText('Include File level'));
    expect(store.getState().included.body).toBe(false);
    fireEvent.click(screen.getByRole('button', { name: 'Post review', hidden: true }));
    await waitFor(() => expect(screen.getByText('Review posted')).toBeTruthy());
    expect(post).toHaveBeenCalledTimes(1);
    expect(post.mock.calls[0]?.[0].findings.map((finding) => finding.id)).toEqual(['inline']);
    const link = screen.getByRole('link', { hidden: true });
    expect(link.getAttribute('href')).toBe('https://github.com/o/r/pull/7#review');
  });

  it('shows the error and offers a retry', async () => {
    setup();
    vi.spyOn(api, 'postReview').mockRejectedValue(new Error('gh: not authenticated'));
    fireEvent.click(screen.getByRole('button', { name: 'Post review', hidden: true }));
    await waitFor(() => expect(screen.getByRole('alert', { hidden: true }).textContent).toContain('gh: not authenticated'));
    expect(screen.getByRole('button', { name: 'Retry', hidden: true })).toBeTruthy();
  });

  it('does not link unsafe URLs', async () => {
    setup();
    vi.spyOn(api, 'postReview').mockResolvedValue({ url: 'javascript:alert(1)', inline_comments: 0, body_findings: 0 });
    fireEvent.click(screen.getByRole('button', { name: 'Post review', hidden: true }));
    await waitFor(() => expect(screen.getByText('Review posted')).toBeTruthy());
    expect(screen.queryByRole('link', { hidden: true })).toBeNull();
  });
});
