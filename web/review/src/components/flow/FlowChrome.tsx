// Flow panel chrome: the toolbar (scope, fit, expand), the hint line and the legend.

interface FlowToolbarProps {
  scope: 'selection' | 'all';
  expanded: boolean;
  onToggleScope(): void;
  onFit(): void;
  onToggleExpanded(): void;
}

export function FlowToolbar({ scope, expanded, onToggleScope, onFit, onToggleExpanded }: FlowToolbarProps) {
  return (
    <div className="fl-toolbar">
      <button type="button" className="fl-scope" onClick={onToggleScope} title="Switch scope (L)" aria-label="Switch between selection and whole PR">
        <span className={scope === 'selection' ? 'fl-scope-on' : ''}>Selection</span>
        <span className="fl-scope-sep">·</span>
        <span className={scope === 'all' ? 'fl-scope-on' : ''}>Whole PR</span>
      </button>
      <span className="fl-toolbar-gap" />
      <button type="button" className="btn" onClick={onFit} title="Fit to view (f)">Fit <kbd>f</kbd></button>
      <button type="button" className="btn" onClick={onToggleExpanded} title={expanded ? 'Back to the side panel (F)' : 'Full width (F)'}>
        {expanded ? 'Collapse' : 'Expand'} <kbd>F</kbd>
      </button>
    </div>
  );
}

export function FlowLegend() {
  return (
    <div className="fl-legend" aria-label="Legend">
      <span className="fl-legend-item">
        <svg width="18" height="12" aria-hidden="true">
          <rect className="fl-legend-box" x="0.5" y="0.5" width="17" height="11" rx="3" />
          <rect x="2.5" y="3" width="2" height="6" rx="1" fill="var(--risk-notable)" />
        </svg>
        Changed · stripe = risk
      </span>
      <span className="fl-legend-item">
        <svg width="18" height="12" aria-hidden="true"><rect className="fl-legend-box fl-legend-context" x="0.5" y="0.5" width="17" height="11" rx="3" /></svg>
        Caller / callee
      </span>
      <span className="fl-legend-item">
        <svg width="18" height="12" aria-hidden="true"><rect className="fl-legend-box fl-legend-external" x="0.5" y="0.5" width="17" height="11" rx="3" /></svg>
        External
      </span>
      <span className="fl-legend-item">
        <svg width="22" height="12" aria-hidden="true"><path className="fl-legend-line" d="M1,6 H15" /><path className="fl-arrowhead" d="M15,2 L21,6 L15,10 z" /></svg>
        Calls
      </span>
      <span className="fl-legend-item">
        <svg width="22" height="12" aria-hidden="true"><path className="fl-legend-line fl-legend-dashed" d="M1,6 H15" /><path className="fl-arrowhead" d="M15,2 L21,6 L15,10 z" /></svg>
        Cycle
      </span>
      <span className="fl-legend-item">
        <svg width="12" height="12" aria-hidden="true"><circle cx="6" cy="6" r="5" fill="var(--sev-warning)" /></svg>
        AI findings
      </span>
    </div>
  );
}
