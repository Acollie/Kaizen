// Pure view-model helpers for the header (and the review headline, which shares the
// engine wording so the two never disagree).

import type { EngineStatus } from '../../state/store';
import type { ReviewResult } from '../../types';

export type EngineTone = 'busy' | 'done' | 'error';

export interface EngineView {
  tone: EngineTone;
  /** Short text shown inline (truncated by CSS) */
  label: string;
  /** Agent doing the work while busy ("" otherwise) */
  agent: string;
  /** Full message for the tooltip */
  title: string;
}

/** "$2.07"; "" when unknown. Sub-cent costs read "<$0.01". */
export function formatCost(cost: number | undefined): string {
  if (cost === undefined || cost === null || !Number.isFinite(cost) || cost < 0) return '';
  if (cost > 0 && cost < 0.01) return '<$0.01';
  return `$${cost.toFixed(2)}`;
}

/** "claude-cli · claude-opus" with empty parts dropped */
export function engineLabel(result: Pick<ReviewResult, 'engine' | 'model' | 'cost_usd'>): string {
  return [formatCost(result.cost_usd), result.engine, result.model].filter(Boolean).join(' · ');
}

export function engineView(status: EngineStatus, result: ReviewResult | null): EngineView {
  switch (status.state) {
    case 'error':
      return { tone: 'error', label: status.message || 'Review failed', agent: '', title: status.message || 'Review failed' };
    case 'done': {
      if (!result) return { tone: 'done', label: status.message || 'Finished', agent: '', title: status.message || 'Finished' };
      const label = ['Reviewed', formatCost(result.cost_usd), result.engine].filter(Boolean).join(' · ');
      const title = ['Review complete', engineLabel(result)].filter(Boolean).join(' · ');
      return { tone: 'done', label, agent: '', title };
    }
    case 'connecting':
      return { tone: 'busy', label: status.message || 'Connecting', agent: '', title: status.message || 'Connecting' };
    default: {
      const message = status.message || 'Reviewing';
      const agent = status.agent || '';
      return { tone: 'busy', label: message, agent, title: agent ? `${agent}: ${message}` : message };
    }
  }
}

/** Meta line parts: author, branches, diff size, file count */
export function prMeta(pr: { author: string; head_ref: string; base_ref: string; additions: number; deletions: number; file_count: number }) {
  return {
    author: pr.author,
    branches: pr.head_ref && pr.base_ref ? `${pr.head_ref} → ${pr.base_ref}` : pr.head_ref || pr.base_ref,
    additions: `+${pr.additions}`,
    deletions: `−${pr.deletions}`,
    files: `${pr.file_count} ${pr.file_count === 1 ? 'file' : 'files'}`,
  };
}
