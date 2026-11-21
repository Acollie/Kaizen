// Function tab for a selected flow node that is not a changed function (an unchanged
// caller or callee, or an external function).

import { useReview, useReviewStore } from '../../state/StoreProvider';
import type { GraphNode } from '../../types';
import { qualifiedName, splitPath } from '../../lib/format';
import { riskColor } from '../../lib/risk';
import { connectedFunctions } from './functionData';
import { Section } from './FunctionSections';

const KIND_LABEL: Record<GraphNode['kind'], string> = {
  changed: 'Changed function',
  caller: 'Unchanged caller',
  callee: 'Unchanged callee',
  external: 'External function',
};

export function NodeInfo({ node }: { node: GraphNode }) {
  const store = useReviewStore();
  const context = useReview((state) => state.context);
  const related = connectedFunctions(context, node.id);
  return (
    <div className="fn-panel">
      <header className="fn-header">
        <div className="fn-title-row"><h2 className="fn-name">{node.name}</h2></div>
        <div className="fn-meta">
          <span>{KIND_LABEL[node.kind]}</span>
          {node.file && <span className="fn-loc" title={node.file}>{splitPath(node.file).base}:{node.line}</span>}
        </div>
      </header>
      <dl className="fn-metrics">
        <div className="fn-metric"><dt>Fan-in</dt><dd>{node.fan_in}</dd></div>
        {node.complexity > 0 && <div className="fn-metric"><dt>Cyclomatic</dt><dd>{node.complexity}</dd></div>}
      </dl>
      {node.file && (
        <div className="fn-actions">
          <button type="button" className="btn btn-outline" onClick={() => store.getState().openLocation(node.file, node.line)}>
            Open definition
          </button>
        </div>
      )}
      <Section title="Connected changed functions" count={related.length}>
        <ul className="fn-list">
          {related.map((fn) => (
            <li key={fn.id}>
              <button type="button" className="fn-row fn-row-changed" onClick={() => store.getState().selectFunction(fn.id)}>
                <span className="fn-row-name">{qualifiedName(fn)}</span>
                <span className="fn-row-loc" style={{ color: riskColor(fn.risk_score) }}>risk {Math.round(fn.risk_score)}</span>
              </button>
            </li>
          ))}
        </ul>
      </Section>
    </div>
  );
}
