// Small fixtures shared by the shell's tests.

import type { FileDiff, Finding, ReviewContext } from '../../types';

export function makeFinding(overrides: Partial<Finding> & { id: string }): Finding {
  return {
    severity: 'warning', category: 'correctness', file: 'pkg/a.go', line: 2,
    title: `Finding ${overrides.id}`, body: 'Body', ...overrides,
  } as Finding;
}

/** pkg/a.go with head lines 1-3 in the diff (line 2 added) */
export function makeFile(path = 'pkg/a.go'): FileDiff {
  return {
    path, old_path: path, status: 'modified', language: 'Go', additions: 1, deletions: 0, binary: false,
    line_count: 40, risk_score: 0.5,
    hunks: [{
      old_start: 1, old_count: 2, new_start: 1, new_count: 3, header: '@@ -1,2 +1,3 @@',
      lines: [
        { kind: 'context', old_line: 1, new_line: 1, text: 'package a' },
        { kind: 'added', old_line: 0, new_line: 2, text: 'var x = 1' },
        { kind: 'context', old_line: 2, new_line: 3, text: '' },
      ],
    }],
  };
}

export function makeContext(): ReviewContext {
  return {
    pr: {
      number: 7, title: 'Test PR', body: '', author: 'dev', url: 'https://github.com/o/r/pull/7', owner: 'o', repo: 'r',
      base_ref: 'main', head_ref: 'feat', base_sha: 'a', head_sha: 'b', merge_base: 'a',
      additions: 1, deletions: 0, file_count: 1,
    },
    files: [makeFile()],
    functions: [],
    graph: { nodes: [], edges: [] },
    concerns: [],
    worktree_path: '/tmp/wt',
    generated_at: '2026-01-01T00:00:00Z',
  };
}
