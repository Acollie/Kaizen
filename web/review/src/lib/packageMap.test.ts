import { describe, expect, it } from 'vitest';
import type { FileDiff, Finding, FunctionRisk, GraphNode, ReviewGraph } from '../types';
import { makeFunction } from '../state/aiCoverage.fixtures';
import {
  buildPackageMap, groupPackages, isTestFile, lookReasons, overviewSummary, summaryParts, unviewedRiskyPackages, whereToLook,
  type CoverageSource, type PackageMapInput,
} from './packageMap';

function file(path: string, additions = 10, deletions = 0, overrides: Partial<FileDiff> = {}): FileDiff {
  return {
    path, old_path: path, status: 'modified', language: path.endsWith('.go') ? 'Go' : '', additions, deletions,
    binary: false, hunks: [], line_count: 100, risk_score: 0, risk_reasons: [], ...overrides,
  };
}

function node(id: string, filePath: string, kind: GraphNode['kind'] = 'changed'): GraphNode {
  return { id, name: id, file: filePath, line: 1, kind, risk_score: 0, focus: false, fan_in: 0, complexity: 1 };
}

function finding(id: string, filePath: string, severity: Finding['severity']): Finding {
  return { id, severity, category: 'correctness', file: filePath, line: 1, title: id, body: '', confidence: 0.8 };
}

const NO_COVERAGE: CoverageSource = { viewedFunctions: {}, viewedFiles: {}, aiFunctions: new Set(), aiFiles: new Set(), aiKnown: true };

function input(overrides: Partial<PackageMapInput>): PackageMapInput {
  return { files: [], functions: [], graph: { nodes: [], edges: [] }, findings: [], ...NO_COVERAGE, ...overrides };
}

/** A PR shaped like a real one: Go packages, a deep TS tree, testdata and root files */
const REALISTIC_PATHS = [
  'README.md', 'Makefile', 'go.mod',
  '.github/workflows/ci.yml',
  'cmd/kaizen/main.go', 'cmd/kaizen/review.go', 'cmd/kaizen/review_test.go',
  'pkg/github/client.go', 'pkg/github/post.go', 'pkg/github/post_test.go',
  'pkg/review/context.go', 'pkg/review/types.go', 'pkg/review/risk_test.go',
  'pkg/review/engine/claude.go', 'pkg/review/engine/claude_test.go', 'pkg/review/engine/testdata/stream.jsonl',
  'pkg/review/tools/tools.go',
  'pkg/reviewui/server.go', 'pkg/reviewui/testdata/worktree/cmd/kaizen/main.go', 'pkg/reviewui/testdata/worktree/pkg/churn/a.go',
  'pkg/models/models.go',
  'web/review/package.json', 'web/review/src/App.tsx', 'web/review/src/types.ts',
  ...['code', 'flow', 'layout', 'post', 'review', 'sidebar', 'header', 'help', 'shared', 'function', 'activity']
    .flatMap((area) => [`web/review/src/components/${area}/A.tsx`, `web/review/src/components/${area}/a.test.ts`]),
  'web/review/src/state/store.ts', 'web/review/src/lib/risk.ts', 'web/review/src/keys/keymap.ts', 'web/review/src/hooks/useKeymap.ts',
  'web/review/src/theme/tokens.css',
];

describe('isTestFile', () => {
  it('recognises Go, TS and Python tests and fixture directories', () => {
    expect(isTestFile('pkg/review/risk_test.go')).toBe(true);
    expect(isTestFile('pkg/review/engine/testdata/stream.jsonl')).toBe(true);
    expect(isTestFile('web/review/src/lib/risk.test.ts')).toBe(true);
    expect(isTestFile('web/review/src/App.spec.tsx')).toBe(true);
    expect(isTestFile('tools/test_parse.py')).toBe(true);
    expect(isTestFile('pkg/review/context.go')).toBe(false);
    expect(isTestFile('pkg/testing/helpers.go')).toBe(false);
    expect(isTestFile('web/review/src/lib/latest.ts')).toBe(false);
  });
});

describe('groupPackages', () => {
  it('keeps one package per directory when within budget, folding testdata into its package', () => {
    const units = groupPackages(['pkg/a/x.go', 'pkg/a/testdata/in.txt', 'pkg/a/testdata/deep/y.go', 'pkg/b/y.go', 'main.go']);
    expect(units.map((unit) => unit.key)).toEqual(['', 'pkg/a', 'pkg/b']);
    expect(units.find((unit) => unit.key === 'pkg/a')?.paths).toHaveLength(3);
  });

  it('collapses deep TS directories first and keeps Go packages apart', () => {
    const units = groupPackages(REALISTIC_PATHS, () => 1, 14);
    const keys = units.map((unit) => unit.key);
    expect(keys.length).toBeLessThanOrEqual(14);
    expect(keys.length).toBeGreaterThanOrEqual(8);
    for (const goPackage of ['cmd/kaizen', 'pkg/github', 'pkg/review', 'pkg/review/engine', 'pkg/review/tools', 'pkg/reviewui']) {
      expect(keys).toContain(goPackage);
    }
    expect(keys).not.toContain('web/review/src/components/flow');
    expect(keys.some((key) => key.startsWith('web/review'))).toBe(true);
  });

  it('merges into the smallest common ancestor needed, not the root', () => {
    const paths = Array.from({ length: 30 }, (_, index) => `web/src/components/c${index}/A.tsx`).concat(['pkg/a/a.go', 'pkg/b/b.go']);
    const keys = groupPackages(paths, () => 1, 20).map((unit) => unit.key);
    expect(keys).toEqual(['pkg/a', 'pkg/b', 'web/src/components']);
  });

  it('merges Go packages only when nothing else is left to merge', () => {
    const paths = Array.from({ length: 25 }, (_, index) => `pkg/p${index}/a.go`);
    const keys = groupPackages(paths, () => 1, 20).map((unit) => unit.key);
    expect(keys).toEqual(['pkg']);
  });
});

describe('buildPackageMap', () => {
  const files = [
    file('pkg/github/post.go', 80, 4, { risk_score: 30 }),
    file('pkg/github/post_test.go', 40),
    file('pkg/review/context.go', 20, 2),
    file('pkg/review/engine/claude.go', 50),
    file('pkg/review/engine/testdata/stream.jsonl', 300),
    file('pkg/review/engine/claude_test.go', 100),
  ];
  const functions: FunctionRisk[] = [
    makeFunction('gh.Post', 'pkg/github/post.go', 1, 40, { risk_score: 48, focus: true, risk_reasons: ['security-sensitive: changed lines use "http.Client"'] }),
    makeFunction('gh.encode', 'pkg/github/post.go', 41, 60, { risk_score: 8 }),
    makeFunction('review.Build', 'pkg/review/context.go', 1, 30, { risk_score: 20 }),
    makeFunction('engine.Run', 'pkg/review/engine/claude.go', 1, 50, { risk_score: 14 }),
  ];
  const graph: ReviewGraph = {
    nodes: [
      node('review.Build', 'pkg/review/context.go'), node('engine.Run', 'pkg/review/engine/claude.go'),
      node('gh.Post', 'pkg/github/post.go'), node('gh.encode', 'pkg/github/post.go'),
      node('main.run', 'cmd/kaizen/main.go', 'caller'), node('fmt.Sprintf', '', 'external'),
      node('unknown.Fn', 'vendor/x/y.go', 'callee'),
    ],
    edges: [
      { from: 'review.Build', to: 'engine.Run', weight: 2 },
      { from: 'review.Build', to: 'engine.Run', weight: 1 },
      { from: 'engine.Run', to: 'gh.Post', weight: 1 },
      { from: 'gh.Post', to: 'gh.encode', weight: 4 },
      { from: 'gh.Post', to: 'fmt.Sprintf', weight: 1 },
      { from: 'main.run', to: 'review.Build', weight: 1 },
      { from: 'engine.Run', to: 'unknown.Fn', weight: 1 },
    ],
  };
  const findings = [
    finding('w1', 'pkg/github/post.go', 'warning'), finding('w2', 'pkg/github/post.go', 'warning'),
    finding('s1', 'pkg/review/context.go', 'suggestion'),
  ];

  it('aggregates files, functions, lines, risk, focus and findings per package', () => {
    const map = buildPackageMap(input({ files, functions, graph, findings }));
    const github = map.packages.find((pkg) => pkg.key === 'pkg/github')!;
    expect(github.files).toHaveLength(2);
    expect(github.functions.map((fn) => fn.id)).toEqual(['gh.Post', 'gh.encode']);
    expect([github.added, github.removed]).toEqual([120, 4]);
    expect(github.maxRisk).toBe(48);
    expect(github.totalRisk).toBe(56);
    expect(github.focusCount).toBe(1);
    expect(github.severity).toEqual({ critical: 0, warning: 2, suggestion: 0, praise: 0 });
  });

  it('measures test share and dims mostly-test packages', () => {
    const map = buildPackageMap(input({ files, functions }));
    const engine = map.packages.find((pkg) => pkg.key === 'pkg/review/engine')!;
    expect(engine.testFiles).toBe(2);
    expect(engine.testShare).toBeCloseTo(400 / 450);
    expect(engine.dim).toBe(true);
    expect(map.packages.find((pkg) => pkg.key === 'pkg/review')!.dim).toBe(false);
  });

  it('never dims a mostly-test package that has focus code or urgent findings', () => {
    const focused = functions.map((fn) => (fn.id === 'engine.Run' ? { ...fn, focus: true } : fn));
    expect(buildPackageMap(input({ files, functions: focused })).packages.find((pkg) => pkg.key === 'pkg/review/engine')!.dim).toBe(false);
    const flagged = buildPackageMap(input({ files, functions, findings: [finding('c', 'pkg/review/engine/claude.go', 'critical')] }));
    expect(flagged.packages.find((pkg) => pkg.key === 'pkg/review/engine')!.dim).toBe(false);
  });

  it('aggregates call edges by package, summing weights, skipping self, external and unknown nodes', () => {
    const map = buildPackageMap(input({ files, functions, graph }));
    expect(map.edges).toEqual([
      { from: 'pkg/review', to: 'pkg/review/engine', weight: 3 },
      { from: 'pkg/review/engine', to: 'pkg/github', weight: 1 },
    ]);
  });

  it('maps unchanged callers into a package that covers their directory', () => {
    const many = [...files, ...Array.from({ length: 24 }, (_, index) => file(`cmd/tool${index}/main.go`))];
    const map = buildPackageMap(input({ files: many, functions, graph: { ...graph, nodes: [...graph.nodes!, node('tool.main', 'cmd/other/main.go', 'caller')], edges: [...graph.edges!, { from: 'tool.main', to: 'gh.Post', weight: 1 }] } }));
    expect(map.packages.some((pkg) => pkg.key === 'cmd')).toBe(true);
    expect(map.edges).toContainEqual({ from: 'cmd', to: 'pkg/github', weight: 1 });
  });

  it('reports what you viewed and what the AI examined; unknown AI coverage is null, not 0', () => {
    const coverage = { viewedFunctions: { 'gh.Post': true as const }, viewedFiles: {}, aiFunctions: new Set(['gh.Post', 'gh.encode']), aiFiles: new Set<string>() };
    const known = buildPackageMap(input({ files, functions, ...coverage, aiKnown: true }));
    const github = known.packages.find((pkg) => pkg.key === 'pkg/github')!;
    expect(github.you.share).toBe(0.5);
    expect(github.ai.share).toBe(1);
    expect(known.packages.find((pkg) => pkg.key === 'pkg/review')!.ai.share).toBe(0);

    const unknown = buildPackageMap(input({ files, functions, ...coverage, aiKnown: false }));
    expect(unknown.packages.find((pkg) => pkg.key === 'pkg/github')!.ai.share).toBeNull();
  });

  it('uses files for coverage when a package has no functions', () => {
    const map = buildPackageMap(input({ files: [file('.github/workflows/ci.yml'), file('.github/workflows/release.yml')], viewedFiles: { '.github/workflows/ci.yml': true } }));
    expect(map.packages[0]!.you.share).toBe(0.5);
  });

  describe('summary and where to look', () => {
    it('writes the one-line summary: you reviewed, AI looked at; unknown AI coverage spelled out', () => {
      const map = buildPackageMap(input({ files, functions }));
      const known = overviewSummary(map, { ...NO_COVERAGE, viewedFunctions: { 'gh.Post': true }, aiFunctions: new Set(['engine.Run']), functions, aiComplete: true });
      expect(summaryParts(known)).toEqual([
        '3 packages', '6 files', '4 functions', "You've reviewed 100% of focus code", 'AI looked at 25% of changed functions',
      ]);
      const unknown = overviewSummary(map, { ...NO_COVERAGE, aiKnown: false, functions, aiComplete: true });
      expect(summaryParts(unknown).slice(3)).toEqual(["You've reviewed 0% of focus code", 'AI coverage unknown']);
    });

    it('never words AI coverage as examined, traced or verified', () => {
      const map = buildPackageMap(input({ files, functions, findings, aiFunctions: new Set(['gh.Post', 'gh.encode']) }));
      const text = [
        ...summaryParts(overviewSummary(map, { ...NO_COVERAGE, aiFunctions: new Set(['gh.Post']), functions, aiComplete: true })),
        ...whereToLook(map).flatMap((target) => target.reasons),
      ].join(' | ');
      expect(text).not.toMatch(/examined|traced|verified|checked/i);
      expect(text).toContain('AI looked at');
    });

    it('ranks risky, urgent, unviewed packages first with an honest reason', () => {
      const map = buildPackageMap(input({ files, functions, findings, aiFunctions: new Set(['review.Build']) }));
      const targets = whereToLook(map);
      expect(targets[0]!.pkg.key).toBe('pkg/github');
      expect(targets[0]!.reasons).toEqual([
        '★ 1 focus · Post risk 48: security-sensitive',
        '2 should-fix findings',
        "AI looked at 0 of 2 functions; you haven't opened it",
      ]);
      // The mostly-test engine package has no urgent findings, so it is not suggested
      expect(targets.map((target) => target.pkg.key)).not.toContain('pkg/review/engine');
    });

    it('ranks an AI-examined, human-unviewed high-risk package above a low-risk one', () => {
      // The AI looked at every function in pkg/github and none in pkg/review
      const map = buildPackageMap(input({ files, functions, findings, aiFunctions: new Set(['gh.Post', 'gh.encode']) }));
      const targets = whereToLook(map);
      expect(targets.map((target) => target.pkg.key)).toEqual(['pkg/github', 'pkg/review']);
      expect(targets[0]!.reasons.at(-1)).toBe("AI looked at 2 of 2 functions; you haven't opened it");
    });

    it('gives AI coverage no weight at all in the ranking', () => {
      const none = whereToLook(buildPackageMap(input({ files, functions, findings })));
      const all = whereToLook(buildPackageMap(input({ files, functions, findings, aiFunctions: new Set(functions.map((fn) => fn.id)) })));
      const unknown = whereToLook(buildPackageMap(input({ files, functions, findings, aiKnown: false })));
      const ranking = (targets: typeof none) => targets.map((target) => [target.pkg.key, target.score]);
      expect(ranking(all)).toEqual(ranking(none));
      expect(ranking(unknown)).toEqual(ranking(none));
    });

    it('leads with packages no human has viewed, even over a riskier partly viewed one', () => {
      const map = buildPackageMap(input({ files, functions, findings, viewedFunctions: { 'gh.Post': true } }));
      expect(whereToLook(map).map((target) => target.pkg.key)).toEqual(['pkg/review', 'pkg/github']);
    });

    it('drops a package once you have viewed it, whatever the AI did', () => {
      const viewedByYou = buildPackageMap(input({
        files, functions, findings,
        viewedFunctions: { 'gh.Post': true, 'gh.encode': true, 'review.Build': true },
      }));
      expect(whereToLook(viewedByYou)).toEqual([]);
      const lookedAtByAi = buildPackageMap(input({ files, functions, findings, aiFunctions: new Set(['gh.Post', 'gh.encode', 'review.Build']) }));
      expect(whereToLook(lookedAtByAi).map((target) => target.pkg.key)).toEqual(['pkg/github', 'pkg/review']);
    });

    it('says when AI coverage is unknown', () => {
      const map = buildPackageMap(input({ files, functions, findings, aiKnown: false }));
      expect(lookReasons(map.packages.find((pkg) => pkg.key === 'pkg/github')!)).toContain("AI coverage unknown; you haven't opened it");
    });

    it('lists risky packages you have not opened for the post check, whether or not the AI looked', () => {
      const map = buildPackageMap(input({ files, functions, findings, aiFunctions: new Set(['gh.Post']), viewedFunctions: { 'review.Build': true } }));
      expect(unviewedRiskyPackages(map).map((pkg) => pkg.key)).toEqual(['pkg/github']);
    });
  });
});
