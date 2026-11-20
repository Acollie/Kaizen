// D3 force graph of the review graph: changed functions coloured by risk, their
// callers/callees muted, externals grey. Selecting a node centres it and dims
// everything outside its one-hop neighbourhood.

import { h, clear, riskColor, topSeverity, truncate } from './dom.js';

const SEVERITY_COLORS = { critical: '--sev-critical', warning: '--sev-warning', suggestion: '--sev-suggestion', praise: '--sev-praise' };

export function createGraph({ store, canvasEl, legendEl, tooltipEl, onNodeClick }) {
  renderLegend(legendEl);
  const d3 = window.d3;
  if (!d3) {
    canvasEl.append(h('div', { class: 'graph-fallback', text: 'The graph needs D3 from cdn.jsdelivr.net, which could not be loaded. Everything else works offline.' }));
    return { focus() {}, fit() {}, highlightCallers() {}, clear() {}, refreshBadges() {} };
  }

  const nodes = store.context.graph.nodes.map((node) => ({ ...node }));
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const links = store.context.graph.edges
    .filter((edge) => byId.has(edge.from) && byId.has(edge.to))
    .map((edge) => ({ source: edge.from, target: edge.to, weight: edge.weight || 1 }));
  const neighbours = new Map(nodes.map((node) => [node.id, new Set([node.id])]));
  for (const link of links) {
    neighbours.get(link.source).add(link.target);
    neighbours.get(link.target).add(link.source);
  }
  const maxFanIn = Math.max(1, ...nodes.map((node) => node.fan_in || 0));
  const radius = (node) => 6 + Math.sqrt((node.fan_in || 0) / maxFanIn) * 14 + (node.kind === 'changed' ? 2 : 0);

  const svg = d3.select(canvasEl).append('svg').attr('role', 'img').attr('aria-label', 'Call graph of changed functions');
  const defs = svg.append('defs');
  defs.append('marker').attr('id', 'arrow').attr('viewBox', '0 -4 8 8').attr('refX', 8).attr('refY', 0)
    .attr('markerWidth', 7).attr('markerHeight', 7).attr('orient', 'auto')
    .append('path').attr('d', 'M0,-4L8,0L0,4').attr('class', 'arrow-head');
  const root = svg.append('g');
  const zoom = d3.zoom().scaleExtent([0.2, 4]).on('zoom', (event) => root.attr('transform', event.transform));
  svg.call(zoom).on('dblclick.zoom', null);

  const { width, height } = size();
  const simulation = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(links).id((node) => node.id).distance((link) => (link.source.kind === 'changed' && link.target.kind === 'changed' ? 70 : 90)))
    .force('charge', d3.forceManyBody().strength(-260))
    .force('center', d3.forceCenter(width / 2, height / 2))
    .force('x', d3.forceX(width / 2).strength(0.04))
    .force('y', d3.forceY(height / 2).strength(0.06))
    .force('collision', d3.forceCollide().radius((node) => radius(node) + 6))
    .stop();
  simulation.tick(300); // settle before first paint so the layout does not jump

  const linkSel = root.append('g').selectAll('line').data(links).join('line')
    .attr('class', 'graph-link').attr('marker-end', 'url(#arrow)')
    .attr('stroke-width', (link) => Math.min(3, 1 + Math.log2(link.weight)));

  const nodeSel = root.append('g').selectAll('g').data(nodes).join('g')
    .attr('class', (node) => `graph-node kind-${node.kind}${node.focus ? ' focus' : ''}`)
    .call(d3.drag().on('start', dragStart).on('drag', dragMove).on('end', dragEnd));

  nodeSel.filter((node) => node.focus).append('circle').attr('class', 'focus-ring').attr('r', (node) => radius(node) + 4);
  nodeSel.append('circle').attr('class', 'body').attr('r', radius).each(function paint(node) {
    this.style.fill = nodeFill(node);
  });
  nodeSel.append('text').attr('class', 'label').attr('text-anchor', 'middle')
    .attr('dy', (node) => radius(node) + 13).text((node) => truncate(node.name, 28));
  const badgeSel = nodeSel.append('g').attr('class', 'badge-group')
    .attr('transform', (node) => `translate(${radius(node) * 0.75},${-radius(node) * 0.75})`);
  badgeSel.append('circle').attr('r', 7);
  badgeSel.append('text').attr('text-anchor', 'middle').attr('dy', 3);

  nodeSel.on('click', (event, node) => { event.stopPropagation(); onNodeClick(node.id); })
    .on('mouseenter', (event, node) => showTooltip(event, node))
    .on('mousemove', (event) => moveTooltip(event))
    .on('mouseleave', () => { tooltipEl.hidden = true; });

  simulation.on('tick', paintPositions);
  paintPositions();
  refreshBadges();
  updateLabels(null);
  store.on('finding', refreshBadges);

  new ResizeObserver(() => {
    const next = size();
    svg.attr('viewBox', `0 0 ${next.width} ${next.height}`);
  }).observe(canvasEl);
  svg.attr('viewBox', `0 0 ${width} ${height}`);
  window.requestAnimationFrame(() => fit(false));

  let selectedId = null;

  function size() {
    return { width: canvasEl.clientWidth || 400, height: canvasEl.clientHeight || 360 };
  }

  function nodeFill(node) {
    if (node.kind === 'changed') return riskColor(node.risk_score);
    if (node.kind === 'external') return 'var(--node-external)';
    return 'var(--node-muted)';
  }

  function paintPositions() {
    linkSel.each(function place(link) {
      const deltaX = link.target.x - link.source.x;
      const deltaY = link.target.y - link.source.y;
      const distance = Math.hypot(deltaX, deltaY) || 1;
      const inset = radius(link.target) + 3;
      this.setAttribute('x1', link.source.x);
      this.setAttribute('y1', link.source.y);
      this.setAttribute('x2', link.target.x - (deltaX / distance) * inset);
      this.setAttribute('y2', link.target.y - (deltaY / distance) * inset);
    });
    nodeSel.attr('transform', (node) => `translate(${node.x},${node.y})`);
  }

  function refreshBadges() {
    badgeSel.each(function paint(node) {
      const findings = findingsForNode(node);
      this.style.display = findings.length ? '' : 'none';
      const severity = topSeverity(findings);
      if (severity) this.firstChild.style.fill = `var(${SEVERITY_COLORS[severity]})`;
      this.lastChild.textContent = findings.length > 9 ? '9+' : String(findings.length);
    });
  }

  function findingsForNode(node) {
    const fn = store.functions.get(node.id);
    return fn ? store.findingsForFunction(fn) : [];
  }

  function updateLabels(focusId) {
    const neighbourhood = focusId ? neighbours.get(focusId) : null;
    nodeSel.classed('show-label', (node) => node.focus || (neighbourhood && neighbourhood.has(node.id)))
      .classed('nbr', (node) => !neighbourhood || neighbourhood.has(node.id))
      .classed('selected', (node) => node.id === focusId);
    linkSel.classed('hl', (link) => Boolean(focusId) && (link.source.id === focusId || link.target.id === focusId))
      .classed('caller-hl', false);
    svg.classed('graph-dim', Boolean(neighbourhood));
  }

  function focus(id, { center = true } = {}) {
    if (!byId.has(id)) return;
    selectedId = id;
    updateLabels(id);
    if (center) {
      const node = byId.get(id);
      svg.transition().duration(450).call(zoom.translateTo, node.x, node.y);
    }
  }

  function highlightCallers(id) {
    if (!byId.has(id)) return;
    focus(id);
    const callers = new Set(links.filter((link) => link.target.id === id).map((link) => link.source.id));
    nodeSel.classed('nbr', (node) => node.id === id || callers.has(node.id))
      .classed('show-label', (node) => node.id === id || callers.has(node.id));
    linkSel.classed('hl', false).classed('caller-hl', (link) => link.target.id === id);
  }

  function clearFocus() {
    selectedId = null;
    updateLabels(null);
  }

  function fit(animate = true) {
    if (!nodes.length) return;
    const { width: viewWidth, height: viewHeight } = size();
    const xs = nodes.map((node) => node.x);
    const ys = nodes.map((node) => node.y);
    const minX = Math.min(...xs) - 40;
    const maxX = Math.max(...xs) + 40;
    const minY = Math.min(...ys) - 40;
    const maxY = Math.max(...ys) + 50;
    const scale = Math.min(2, 0.95 / Math.max((maxX - minX) / viewWidth, (maxY - minY) / viewHeight));
    const transform = d3.zoomIdentity.translate(viewWidth / 2, viewHeight / 2).scale(scale)
      .translate(-(minX + maxX) / 2, -(minY + maxY) / 2);
    (animate ? svg.transition().duration(400) : svg).call(zoom.transform, transform);
  }

  function showTooltip(event, node) {
    const findings = findingsForNode(node);
    const rows = [
      node.file ? `${node.file}${node.line ? `:${node.line}` : ''}` : 'external',
      node.kind === 'changed' ? `risk ${Math.round(node.risk_score)} · complexity ${node.complexity} · fan-in ${node.fan_in}` : `${node.kind} · fan-in ${node.fan_in}`,
      findings.length ? `${findings.length} finding${findings.length === 1 ? '' : 's'}` : null,
    ].filter(Boolean);
    tooltipEl.replaceChildren(h('div', { class: 'tt-title', text: node.name }), rows.map((text) => h('div', { class: 'tt-row', text })));
    tooltipEl.hidden = false;
    moveTooltip(event);
  }

  function moveTooltip(event) {
    const bounds = canvasEl.parentElement.getBoundingClientRect();
    const left = Math.min(event.clientX - bounds.left + 14, bounds.width - 290);
    tooltipEl.style.left = `${Math.max(4, left)}px`;
    tooltipEl.style.top = `${event.clientY - bounds.top + 14}px`;
  }

  function dragStart(event, node) {
    if (!event.active) simulation.alphaTarget(0.2).restart();
    node.fx = node.x;
    node.fy = node.y;
  }
  function dragMove(event, node) {
    node.fx = event.x;
    node.fy = event.y;
  }
  function dragEnd(event, node) {
    if (!event.active) simulation.alphaTarget(0);
    node.fx = null;
    node.fy = null;
  }

  svg.on('click', () => clearFocus());

  return { focus, fit, highlightCallers, clear: clearFocus, refreshBadges, selected: () => selectedId };
}

function renderLegend(legendEl) {
  clear(legendEl);
  const item = (swatchClass, text) => h('span', { class: 'legend-item' }, h('span', { class: `legend-swatch ${swatchClass}` }), text);
  legendEl.append(
    item('gradient', 'Changed (risk low → high)'),
    item('muted', 'Caller / callee'),
    item('external', 'External'),
    item('ring', 'Focus'),
    item('badge-swatch', 'Findings'),
  );
}
