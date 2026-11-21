// The package map: packages laid out left to right by calls, with pan/zoom (d3-zoom
// owns only the viewport transform, as in the flow panel), a Fit button and hover cards.

import { useCallback, useEffect, useId, useMemo, useRef, useState, type MouseEvent } from 'react';
import { useCommands } from '../../state/commands';
import { useReview } from '../../state/StoreProvider';
import { findingsHidden } from '../../state/store';
import type { PackageMap } from '../../lib/packageMap';
import { AI_LOOKED_HINT, YOU_REVIEWED_HINT } from '../../lib/coverageCopy';
import { ArrowMarkers, FlowEdges } from '../flow/FlowEdges';
import { useElementSize, useFlowZoom } from '../flow/useFlowZoom';
import { layoutOverview } from './overviewLayout';
import { PackageNode, sizeScale } from './PackageNode';
import { PackageTooltip, placePackageTooltip } from './PackageTooltip';
import '../flow/FlowPanel.css';

interface OverviewMapProps {
  map: PackageMap;
  selectedKey: string | null;
  onSelect(key: string | null): void;
}

interface Hover { key: string; left: number; top: number }

export function OverviewMap({ map, selectedKey, onSelect }: OverviewMapProps) {
  const idPrefix = `ov${useId().replace(/[^a-zA-Z0-9]/g, '')}`;
  const canvasRef = useRef<HTMLDivElement>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const viewportRef = useRef<SVGGElement>(null);
  const size = useElementSize(canvasRef);
  // The map sits in a scrolling view: the wheel scrolls, Ctrl/Cmd + wheel or pinch zooms
  const fit = useFlowZoom(svgRef, viewportRef, { wheel: 'modified' });
  const layout = useMemo(() => layoutOverview(map.packages, map.edges), [map]);
  const canvasHeight = canvasHeightFor(layout.bounds, size.width);
  const scale = useMemo(() => sizeScale(map.packages), [map.packages]);
  const activeIds = useMemo(() => new Set(selectedKey !== null ? [selectedKey] : []), [selectedKey]);
  const [hover, setHover] = useState<Hover | null>(null);

  // Fit on first paint, on resize and when the set of packages changes; not when coverage updates
  const shapeKey = `${layout.nodes.map((node) => `${node.id}@${Math.round(node.x)},${Math.round(node.y)}`).join('|')}`;
  const fitNow = useCallback((animate: boolean) => fit(layout.bounds, animate), [fit, layout.bounds]);
  const fitRef = useRef(fitNow);
  fitRef.current = fitNow;
  useEffect(() => {
    if (size.width > 0 && size.height > 0) fitRef.current(false);
  }, [shapeKey, size.width, size.height]);

  useCommands({ fitGraph: () => fitNow(true) });

  const onHover = useCallback((key: string | null, event?: MouseEvent) => {
    const svg = svgRef.current;
    if (key === null || !event || !svg) {
      setHover(null);
      return;
    }
    const bounds = svg.getBoundingClientRect();
    setHover({ key, ...placePackageTooltip(event.clientX - bounds.left, event.clientY - bounds.top, bounds.width, bounds.height) });
  }, []);
  const hovered = hover ? map.packages.find((pkg) => pkg.key === hover.key) : undefined;

  return (
    <div className="ov-map">
      <div className="ov-map-bar">
        <span className="label">Packages</span>
        <span className="ov-map-hint">Arrows: calls between packages in the PR · click a package for its files and functions</span>
        <span className="ov-gap" />
        <button type="button" className="btn" onClick={() => fitNow(true)} title="Fit to view (f)">Fit <kbd>f</kbd></button>
      </div>
      <MapLegend />
      <div className="ov-canvas" ref={canvasRef} style={{ height: canvasHeight }}>
        <svg ref={svgRef} className="fl-svg ov-svg" role="img" aria-label="Packages this PR touches and the calls between them"
          onClick={() => onSelect(null)}>
          <ArrowMarkers idPrefix={idPrefix} />
          <g ref={viewportRef}>
            {layout.gridLabel && (
              <text className="fl-group" x={layout.gridLabel.x} y={layout.gridLabel.y}>
                <tspan className="fl-group-detail">{layout.gridLabel.text}</tspan>
              </text>
            )}
            <FlowEdges edges={layout.edges} activeIds={activeIds} hoverId={hover?.key ?? null} idPrefix={idPrefix} />
            <g>
              {layout.nodes.map((node) => (
                <PackageNode key={node.id} node={node} selected={node.id === selectedKey} scale={scale}
                  onSelect={(key) => onSelect(key === selectedKey ? null : key)} onHover={onHover} />
              ))}
            </g>
          </g>
        </svg>
        {hover && hovered && <PackageTooltip pkg={hovered} left={hover.left} top={hover.top} />}
      </div>
    </div>
  );
}

const MIN_READABLE_SCALE = 0.62;
const CANVAS_PADDING = 40;

/**
 * Size the canvas so the map fits its width at a readable scale (never below
 * MIN_READABLE_SCALE, never above 1); the surrounding view scrolls for the rest.
 */
export function canvasHeightFor(bounds: { width: number; height: number }, width: number): number {
  if (width <= 0) return 320;
  const scale = Math.max(MIN_READABLE_SCALE, Math.min(1, (width - CANVAS_PADDING) / Math.max(1, bounds.width)));
  return Math.max(240, Math.ceil(bounds.height * scale + CANVAS_PADDING));
}

function MapLegend() {
  const aiHidden = useReview(findingsHidden);
  return (
    <div className="fl-legend ov-legend" aria-label="Legend">
      <span className="fl-legend-item">
        <svg width="10" height="12" aria-hidden="true"><rect x="3" y="1" width="3" height="10" rx="1.5" fill="var(--risk-notable)" /></svg>
        stripe = max risk
      </span>
      <span className="fl-legend-item">
        <svg width="22" height="6" aria-hidden="true"><rect y="1.5" width="14" height="3" rx="1.5" fill="var(--diff-add-mark)" /><rect x="14" y="1.5" width="8" height="3" rx="1.5" fill="var(--diff-del-mark)" /></svg>
        lines changed
      </span>
      <span className="fl-legend-item" title={YOU_REVIEWED_HINT}>
        <svg width="22" height="6" aria-hidden="true"><rect y="1.5" width="22" height="3" rx="1.5" fill="var(--line-strong)" /><rect y="1.5" width="12" height="3" rx="1.5" fill="var(--accent)" /></svg>
        you reviewed (verified)
      </span>
      {!aiHidden && <span className="fl-legend-item" title={AI_LOOKED_HINT}>
        <svg width="22" height="6" aria-hidden="true"><rect y="1.5" width="22" height="3" rx="1.5" fill="var(--line-strong)" /><rect y="1.5" width="8" height="3" rx="1.5" fill="var(--text-2)" /></svg>
        AI looked at (not verified)
      </span>}
      <span className="fl-legend-item"><span className="ov-legend-dim">pkg</span> mostly tests</span>
    </div>
  );
}
