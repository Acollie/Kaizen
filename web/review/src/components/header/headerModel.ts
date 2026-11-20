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

/**
 * What the review used, labelled honestly. The Claude Code CLI engine runs on the
 * user's Claude plan: its cost_usd is an API-equivalent estimate of usage, not a
 * charge. Only the API engine's figure is money billed to an API key.
 */
export function usageLabel(result: Pick<ReviewResult, 'engine' | 'cost_usd'>): string {
  const cost = formatCost(result.cost_usd);
  if (!cost) return '';
  if (result.engine === 'anthropic-api') return `${cost} API`;
  return `≈${cost} usage`;
}

/** Tooltip text explaining usageLabel */
export function usageExplanation(result: Pick<ReviewResult, 'engine' | 'cost_usd'>): string {
  if (!formatCost(result.cost_usd)) return '';
  if (result.engine === 'anthropic-api') return 'Billed to your Anthropic API key';
  return 'API-equivalent estimate of model usage; counts against your Claude plan, not billed per token';
}

/** "≈$2.07 usage · claude-cli · opus" with empty parts dropped */
export function engineLabel(result: Pick<ReviewResult, 'engine' | 'model' | 'cost_usd'>): string {
  return [usageLabel(result), result.engine, result.model].filter(Boolean).join(' · ');
}

export function engineView(status: EngineStatus, result: ReviewResult | null): EngineView {
  switch (status.state) {
    case 'error':
      return { tone: 'error', label: status.message || 'Review failed', agent: '', title: status.message || 'Review failed' };
    case 'done': {
      if (!result) return { tone: 'done', label: status.message || 'Finished', agent: '', title: status.message || 'Finished' };
      const label = ['Reviewed', usageLabel(result), result.engine].filter(Boolean).join(' · ');
      const title = ['Review complete', engineLabel(result), usageExplanation(result)].filter(Boolean).join(' · ');
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
