import { describe, expect, it } from 'vitest';
import type { PackageInfo } from '../../lib/packageMap';
import { layoutOverview } from './overviewLayout';
import { titleParts } from './PackageNode';

function pkg(key: string, maxRisk = 10): PackageInfo {
  const empty = { seen: 0, total: 0 };
  return {
    key, label: key, files: [], functions: [], added: 1, removed: 0, maxRisk, totalRisk: maxRisk, focusCount: 0, findings: [],
    severity: { critical: 0, warning: 0, suggestion: 0, praise: 0 }, testFiles: 0, testShare: 0, dim: false,
    you: { functions: empty, files: empty, share: 0 }, ai: { functions: empty, files: empty, share: null },
  };
}

function overlaps(left: { x: number; y: number; width: number; height: number }, right: typeof left): boolean {
  return left.x < right.x + right.width && right.x < left.x + left.width && left.y < right.y + right.height && right.y < left.y + left.height;
}

describe('layoutOverview', () => {
  const packages = ['cmd/kaizen', 'pkg/review', 'pkg/review/engine', 'pkg/check', 'pkg/github', 'docs', 'scripts', 'web/review/src'].map((key) => pkg(key));
  const edges = [
    { from: 'cmd/kaizen', to: 'pkg/review', weight: 10 },
    { from: 'cmd/kaizen', to: 'pkg/github', weight: 2 },
    { from: 'pkg/review', to: 'pkg/check', weight: 2 },
    { from: 'pkg/review/engine', to: 'pkg/review', weight: 2 },
  ];

  it('lays callers left of callees and puts unconnected packages in a labelled grid below', () => {
    const layout = layoutOverview(packages, edges);
    const at = (key: string) => layout.nodes.find((node) => node.id === key)!;
    expect(at('cmd/kaizen').x).toBeLessThan(at('pkg/review').x);
    expect(at('pkg/review').x).toBeLessThan(at('pkg/check').x);
    const layeredBottom = Math.max(...layout.nodes.filter((node) => node.connected).map((node) => node.y + node.height));
    for (const key of ['docs', 'scripts', 'web/review/src']) {
      expect(at(key).connected).toBe(false);
      expect(at(key).y).toBeGreaterThan(layeredBottom);
    }
    expect(layout.gridLabel?.text).toContain('3');
    expect(layout.edges).toHaveLength(4);
  });

  it('never overlaps two packages', () => {
    const layout = layoutOverview(packages, edges);
    for (const left of layout.nodes) {
      for (const right of layout.nodes) if (left !== right) expect(overlaps(left, right)).toBe(false);
    }
  });

  it('handles a PR with no calls between packages', () => {
    const layout = layoutOverview(packages.slice(0, 3), []);
    expect(layout.gridLabel).toBeNull();
    expect(layout.nodes).toHaveLength(3);
    expect(layout.edges).toEqual([]);
  });
});

describe('titleParts', () => {
  it('keeps short paths and drops leading directories from long ones', () => {
    expect(titleParts('pkg/review/engine', 30)).toEqual({ dir: 'pkg/review/', base: 'engine' });
    expect(titleParts('(root)', 30)).toEqual({ dir: '', base: '(root)' });
    expect(titleParts('web/review/src/components/overview', 22)).toEqual({ dir: '…/components/', base: 'overview' });
  });
});
