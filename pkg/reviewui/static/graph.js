// D3 force graph of the review graph: changed functions coloured by risk, their
// callers/callees muted, externals grey. Selecting a node zooms to fit it and its
// one-hop neighbourhood; in layered mode (default, toggle with L) callers line up
// on the left and callees on the right of the selection.

import { h, clear, riskColor, topSeverity, truncate } from './dom.js';

const SEVERITY_COLORS = { critical: '--sev-critical', warning: '--sev-warning', suggestion: '--sev-suggestion', praise: '--sev-praise' };
const LAYER_GAP_X = 190;
const LAYER_GAP_Y = 44;
const MOVE_MS = 450;

const NOOP_GRAPH = { focus() {}, fit() {}, highlightCallers() {}, clear() {}, refreshBadges() {}, toggleLayered() {} };

export function createGraph({ store, canvasEl, legendEl, tooltipEl, hintEl, onNodeClick }) {
  renderLegend(legendEl);
  const d3 = window.d3;
  if (!d3) {
    canvasEl.append(h('div', { class: 'graph-fallback', text: 'The graph needs D3 from cdn.jsdelivr.net, which could not be loaded. Everything else works offline.' }));
    return NOOP_GRAPH;
  }

  const nodes = store.context.graph.nodes.map((node) => ({ ...node }));
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const links = store.context.graph.edges
    .filter((edge) => byId.has(edge.from) && byId.has(edge.to) && edge.from !== edge.to)
    .map((edge) => ({ source: edge.from, target: edge.to, weight: edge.weight || 1 }));
  const callersOf = new Map(nodes.map((node) => [node.id, new Set()]));
  const calleesOf = new Map(nodes.map((node) => [node.id, new Set()]));
  for (const link of links) {
    calleesOf.get(link.source).add(link.target);
    callersOf.get(link.target).add(link.source);
  }
  const neighbourhoodOf = (id) => new Set([id, ...callersOf.get(id), ...calleesOf.get(id)]);
  const maxFanIn = Math.max(1, ...nodes.map((node) => node.fan_in || 0));
  const radius = (node) => 7 + Math.sqrt((node.fan_in || 0) / maxFanIn) * 12 + (node.kind === 'changed' ? 2 : 0);

  let selectedId = null;
  let layered = true;

  const svg = d3.select(canvasEl).append('svg').attr('role', 'img').attr('aria-label', 'Call graph of changed functions');
  appendMarkers(svg.append('defs'));
  const root = svg.append('g');
  const zoom = d3.zoom().scaleExtent([0.2, 3]).on('zoom', (event) => root.attr('transform', event.transform));
  svg.call(zoom).on('dblclick.zoom', null);

  const initial = size();
  const simulation = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(links).id((node) => node.id).distance(70).strength(0.6))
    .force('charge', d3.forceManyBody().strength(-170))
    .force('x', d3.forceX(initial.width / 2).strength(0.07))
    .force('y', d3.forceY(initial.height / 2).strength(0.09))
    .force('collision', d3.forceCollide().radius((node) => radius(node) + 10))
    .stop();
  simulation.tick(300); // settle before first paint so the layout does not jump

  const linkSel = root.append('g').selectAll('line').data(links).join('line')
    .attr('class', 'graph-link').attr('marker-end', 'url(#arrow)')
    .attr('stroke-width', (link) => Math.min(3, 1.2 + Math.log2(link.weight)));

  const nodeSel = root.append('g').selectAll('g').data(nodes).join('g')
    .attr('class', (node) => `graph-node kind-${node.kind}${node.focus ? ' focus' : ''}`)
    .call(d3.drag().on('start', dragStart).on('drag', dragMove).on('end', dragEnd));
  nodeSel.filter((node) => node.focus).append('circle').attr('class', 'focus-ring').attr('r', (node) => radius(node) + 4);
  nodeSel.append('circle').attr('class', 'body').attr('r', radius).each(function paint(node) {
    this.style.fill = nodeFill(node);
  });
  nodeSel.append('text').attr('class', 'label').attr('text-anchor', 'middle')
    .attr('dy', (node) => radius(node) + 14).text((node) => truncate(node.name, 30));
  const badgeSel = nodeSel.append('g').attr('class', 'badge-group')
    .attr('transform', (node) => `translate(${radius(node) * 0.75},${-radius(node) * 0.75})`);
  badgeSel.append('circle').attr('r', 7);
  badgeSel.append('text').attr('text-anchor', 'middle').attr('dy', 3);

  nodeSel.on('click', (event, node) => { event.stopPropagation(); onNodeClick(node.id); })
    .on('mouseenter', (event, node) => showTooltip(event, node))
    .on('mousemove', (event) => moveTooltip(event))
    .on('mouseleave', () => { tooltipEl.hidden = true; });
  svg.on('click', () => clearFocus());

  simulation.on('tick', paintPositions);
  paintPositions();
  refreshBadges();
  applyClasses(null);
  updateHint();
  store.on('finding', refreshBadges);

  new ResizeObserver(() => {
    const next = size();
    svg.attr('viewBox', `0 0 ${next.width} ${next.height}`);
  }).observe(canvasEl);
  svg.attr('viewBox', `0 0 ${initial.width} ${initial.height}`);
  window.requestAnimationFrame(() => fitTo(nodes, false));

  // ---------- Painting ----------

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
      const startInset = radius(link.source) + 1;
      const endInset = radius(link.target) + 4;
      this.setAttribute('x1', link.source.x + (deltaX / distance) * startInset);
      this.setAttribute('y1', link.source.y + (deltaY / distance) * startInset);
      this.setAttribute('x2', link.target.x - (deltaX / distance) * endInset);
      this.setAttribute('y2', link.target.y - (deltaY / distance) * endInset);
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

  function applyClasses(focusId, { callersOnly = false } = {}) {
    let visible = null;
    if (focusId) visible = callersOnly ? new Set([focusId, ...callersOf.get(focusId)]) : neighbourhoodOf(focusId);
    nodeSel.classed('show-label', (node) => node.focus || Boolean(visible && visible.has(node.id)))
      .classed('nbr', (node) => !visible || visible.has(node.id))
      .classed('selected', (node) => node.id === focusId);
    const touches = (link) => {
      if (!focusId) return false;
      if (callersOnly) return link.target.id === focusId;
      return link.source.id === focusId || link.target.id === focusId;
    };
    linkSel.classed('hl', (link) => !callersOnly && touches(link))
      .classed('caller-hl', (link) => callersOnly && link.target.id === focusId)
      .attr('marker-end', (link) => (touches(link) ? 'url(#arrow-hl)' : 'url(#arrow)'));
    svg.classed('graph-dim', Boolean(visible));
  }

  // ---------- Layout ----------

  /** Target positions: selection fixed, callers in a column to its left, callees to its right */
  function layeredTargets(id) {
    const center = byId.get(id);
    const targets = new Map([[id, { x: center.x, y: center.y }]]);
    const callers = [...callersOf.get(id)];
    const callees = [...calleesOf.get(id)].filter((callee) => !callersOf.get(id).has(callee));
    const column = (ids, x) => ids.sort((left, right) => byId.get(left).y - byId.get(right).y).forEach((nodeId, index) => {
      targets.set(nodeId, { x, y: center.y + (index - (ids.length - 1) / 2) * LAYER_GAP_Y });
    });
    column(callers, center.x - LAYER_GAP_X);
    column(callees, center.x + LAYER_GAP_X);
    return targets;
  }

  /** Pin targets, let the rest of the graph make room, and animate to the new layout */
  function relayout(targets) {
    const before = new Map(nodes.map((node) => [node.id, { x: node.x, y: node.y }]));
    for (const node of nodes) {
      const target = targets.get(node.id);
      node.fx = target ? target.x : null;
      node.fy = target ? target.y : null;
    }
    simulation.alpha(0.5);
    simulation.tick(160);
    const after = new Map(nodes.map((node) => [node.id, { x: node.fx ?? node.x, y: node.fy ?? node.y }]));
    animatePositions(before, after);
  }

  function animatePositions(before, after) {
    const ease = d3.easeCubicInOut;
    const timer = d3.timer((elapsed) => {
      const progress = ease(Math.min(1, elapsed / MOVE_MS));
      for (const node of nodes) {
        const from = before.get(node.id);
        const to = after.get(node.id);
        node.x = from.x + (to.x - from.x) * progress;
        node.y = from.y + (to.y - from.y) * progress;
      }
      paintPositions();
      if (progress >= 1) timer.stop();
    });
  }

  function releasePins() {
    for (const node of nodes) {
      node.fx = null;
      node.fy = null;
    }
  }

  function fitTo(subset, animate = true) {
    if (!subset.length) return;
    const { width, height } = size();
    const xs = subset.map((node) => node.fx ?? node.x);
    const ys = subset.map((node) => node.fy ?? node.y);
    // labels sit centred under nodes, so leave room for about 15 characters either side
    const padX = 100;
    const padY = 44;
    const minX = Math.min(...xs) - padX;
    const maxX = Math.max(...xs) + padX;
    const minY = Math.min(...ys) - padY;
    const maxY = Math.max(...ys) + padY + 16;
    const scale = Math.max(0.3, Math.min(1.6, Math.min(width / (maxX - minX), height / (maxY - minY))));
    const transform = d3.zoomIdentity.translate(width / 2, height / 2).scale(scale)
      .translate(-(minX + maxX) / 2, -(minY + maxY) / 2);
    (animate ? svg.transition().duration(MOVE_MS + 150) : svg).call(zoom.transform, transform);
  }

  // ---------- Public API ----------

  function focus(id) {
    if (!byId.has(id)) return;
    selectedId = id;
    applyClasses(id);
    if (layered) relayout(layeredTargets(id));
    fitTo([...neighbourhoodOf(id)].map((nodeId) => byId.get(nodeId)));
    updateHint();
  }

  function highlightCallers(id) {
    if (!byId.has(id)) return;
    focus(id);
    applyClasses(id, { callersOnly: true });
    fitTo([id, ...callersOf.get(id)].map((nodeId) => byId.get(nodeId)));
  }

  function clearFocus() {
    selectedId = null;
    applyClasses(null);
    updateHint();
  }

  function toggleLayered() {
    layered = !layered;
    if (!layered) {
      releasePins();
      simulation.alpha(0.4).restart();
      window.setTimeout(() => simulation.stop(), 1500);
    }
    if (selectedId) focus(selectedId);
    updateHint();
    return layered;
  }

  function updateHint() {
    if (!hintEl) return;
    const selected = selectedId && byId.get(selectedId);
    hintEl.textContent = selected
      ? `${callersOf.get(selectedId).size} callers ← ${truncate(selected.name, 24)} → ${calleesOf.get(selectedId).size} callees`
      : 'Select a function to see its callers and callees';
    hintEl.dataset.layered = String(layered);
    const toggle = document.getElementById('graph-layered');
    if (toggle) toggle.setAttribute('aria-pressed', String(layered));
  }

  function showTooltip(event, node) {
    const findings = findingsForNode(node);
    const rows = [
      node.file ? `${node.file}${node.line ? `:${node.line}` : ''}` : 'external',
      node.kind === 'changed' ? `risk ${Math.round(node.risk_score)} · complexity ${node.complexity} · fan-in ${node.fan_in}` : `${node.kind} · fan-in ${node.fan_in}`,
      `${callersOf.get(node.id).size} callers · ${calleesOf.get(node.id).size} callees in view`,
      findings.length ? `${findings.length} finding${findings.length === 1 ? '' : 's'}` : null,
    ].filter(Boolean);
    tooltipEl.replaceChildren(h('div', { class: 'tt-title', text: node.name }), ...rows.map((text) => h('div', { class: 'tt-row', text })));
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
    if (!layered || !selectedId) {
      node.fx = null;
      node.fy = null;
    }
    window.setTimeout(() => simulation.stop(), 1200);
  }

  return {
    focus,
    fit: () => fitTo(nodes),
    highlightCallers,
    clear: clearFocus,
    refreshBadges,
    toggleLayered,
    selected: () => selectedId,
  };
}

function appendMarkers(defs) {
  for (const [id, className] of [['arrow', 'arrow-head'], ['arrow-hl', 'arrow-head hl']]) {
    defs.append('marker').attr('id', id).attr('viewBox', '0 -5 10 10').attr('refX', 9).attr('refY', 0)
      .attr('markerUnits', 'userSpaceOnUse').attr('markerWidth', 9).attr('markerHeight', 9).attr('orient', 'auto')
      .append('path').attr('d', 'M0,-5L10,0L0,5Z').attr('class', className);
  }
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
    h('span', { class: 'legend-item' }, h('span', { class: 'legend-arrow', text: '→' }), 'calls'),
  );
}
