import { describe, expect, test } from 'vitest';
import type { ReviewResult } from '../../types';
import { engineLabel, engineView, formatCost, prMeta } from './headerModel';

const result: ReviewResult = { summary: '', verdict: 'comment', findings: [], engine: 'claude-cli', model: 'opus', cost_usd: 2.0672 };

describe('formatCost', () => {
  test('formats dollars', () => {
    expect(formatCost(2.0672)).toBe('$2.07');
    expect(formatCost(0)).toBe('$0.00');
    expect(formatCost(0.004)).toBe('<$0.01');
    expect(formatCost(undefined)).toBe('');
    expect(formatCost(Number.NaN)).toBe('');
  });
});

describe('engineView', () => {
  test('running shows the agent and its action', () => {
    const view = engineView({ state: 'running', message: 'get_function(config.Validate)', agent: 'hotspot-reviewer' }, null);
    expect(view).toEqual({ tone: 'busy', label: 'get_function(config.Validate)', agent: 'hotspot-reviewer', title: 'hotspot-reviewer: get_function(config.Validate)' });
  });
  test('thinking counts as busy', () => {
    expect(engineView({ state: 'thinking', message: 'thinking', agent: 'lead' }, null).tone).toBe('busy');
  });
  test('done with a result shows cost and engine', () => {
    const view = engineView({ state: 'done', message: 'Review complete', agent: '' }, result);
    expect(view.label).toBe('Reviewed · $2.07 · claude-cli');
    expect(view.title).toBe('Review complete · $2.07 · claude-cli · opus');
  });
  test('done without a result uses the status message', () => {
    expect(engineView({ state: 'done', message: 'Finished', agent: '' }, null).label).toBe('Finished');
  });
  test('error is red with the full message in the tooltip', () => {
    const view = engineView({ state: 'error', message: 'claude: not found in PATH', agent: '' }, null);
    expect(view.tone).toBe('error');
    expect(view.title).toBe('claude: not found in PATH');
  });
});

describe('engineLabel and prMeta', () => {
  test('engineLabel drops missing parts', () => {
    expect(engineLabel({ engine: 'api', model: undefined, cost_usd: undefined })).toBe('api');
  });
  test('prMeta formats branches and sizes', () => {
    const meta = prMeta({ author: 'acollie', head_ref: 'fix', base_ref: 'main', additions: 12, deletions: 3, file_count: 1 });
    expect(meta).toEqual({ author: 'acollie', branches: 'fix → main', additions: '+12', deletions: '−3', files: '1 file' });
  });
});
