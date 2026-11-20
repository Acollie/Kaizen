// Call-flow diagram of the review graph: a layered, directed layout where calls flow
// from callers to callees. With nothing selected it shows the whole PR, one flow per
// connected group of calls, riskiest group first. Selecting a function re-lays out
// around it: its callers (up to two levels) flow into it and it flows into its
// callees. In the narrow side panel the flow runs top to bottom; expanded (F) it runs
// left to right.

import { h, clear, riskColor, topSeverity, severityCounts } from './dom.js';

const FOCUS_DEPTH = 2;
const NODE_HEIGHT = 40;
const NODE_MIN_WIDTH = 96;
const NODE_MAX_WIDTH = 210;
const CHAR_WIDTH = 7.1;
const RANK_GAP = { TB: 64, LR: 90 };
const SIBLING_GAP = { TB: 14, LR: 14 };
const COMPONENT_GAP = 40;
const MOVE_MS = 420;
const SEVERITY_VARS = { critical: '--sev-critical', warning: '--sev-warning', suggestion: '--sev-suggestion', praise: '--sev-praise' };

const NOOP_GRAPH = { focus() {}, fit() {}, highlightCallers() {}, clear() {}, refreshBadges() {}, toggleLayered() {}, setExpanded() {} };

export function createGraph({ store, canvasEl, legendEl, tooltipEl, hintEl, onNodeClick }) {
  renderLegend(legendEl);
  const d3 = window.d3;
  if (!d3) {
    canvasEl.append(h('div', { class: 'graph-fallback', text: 'The flow diagram needs D3 from cdn.jsdelivr.net, which could not be loaded. Everything else works offline.' }));
    return NOOP_GRAPH;
  }

  const model = buildModel(store.context.graph);
  let selectedId = null;
  let expanded = false;
  let lastLayout = null;

  const svg = d3.select(canvasEl).append('svg').attr('role', 'img').attr('aria-label', 'Call flow of the changed functions');
  appendMarkers(svg.append('defs'));
  const viewport = svg.append('g');
  const groupLayer = viewport.append('g').attr('class', 'flow-groups');
  const edgeLayer = viewport.append('g').attr('class', 'flow-edges');
  const nodeLayer = viewport.append('g').attr('class', 'flow-nodes');
  const zoom = d3.zoom().scaleExtent([0.15, 2.5]).on('zoom', (event) => viewport.attr('transform', event.transform));
  svg.call(zoom).on('dblclick.zoom', null);

  new ResizeObserver(() => render({ animate: false })).observe(canvasEl);
  store.on('finding', () => refreshBadges());

  function direction() {
    return expanded || canvasEl.clientWidth > canvasEl.clientHeight * 1.3 ? 'LR' : 'TB';
  }

  function render({ animate = true } = {}) {
    if (!canvasEl.clientWidth) return;
    const visibleIds = selectedId ? model.focusSet(selectedId, FOCUS_DEPTH) : new Set(model.nodes.keys());
    const rankDirection = direction();
    const layout = layoutFlow(model, visibleIds, { focusId: selectedId, direction: rankDirection });
    lastLayout = layout;
    drawGroups(layout, animate);
    drawEdges(layout, animate);
    drawNodes(layout, animate);
    updateHint(layout);
    fit({ animate });
  }

  function drawGroups(layout, animate) {
    const groups = groupLayer.selectAll('g.flow-group').data(layout.groups, (group) => group.key);
    groups.exit().remove();
    const entered = groups.enter().append('g').attr('class', 'flow-group');
    entered.append('rect').attr('rx', 10);
    entered.append('text').attr('class', 'flow-group-label');
    const merged = entered.merge(groups);
    const transition = (selection) => (animate ? selection.transition().duration(MOVE_MS) : selection);
    transition(merged.select('rect'))
      .attr('x', (group) => group.x).attr('y', (group) => group.y)
      .attr('width', (group) => group.width).attr('height', (group) => group.height);
    transition(merged.select('text')).attr('x', (group) => group.x + 10).attr('y', (group) => group.y + 15);
    merged.select('text').text((group) => group.label);
  }

  function drawEdges(layout, animate) {
    const edges = edgeLayer.selectAll('g.flow-edge').data(layout.edges, (edge) => `${edge.from}->${edge.to}`);
    edges.exit().remove();
    const entered = edges.enter().append('g').attr('class', 'flow-edge');
    entered.append('path').attr('marker-end', 'url(#flow-arrow)');
    entered.append('text').attr('class', 'flow-edge-weight');
    const merged = entered.merge(edges);
    merged.classed('on-path', (edge) => Boolean(selectedId) && (edge.from === selectedId || edge.to === selectedId))
      .classed('back-edge', (edge) => edge.reversed);
    const paths = merged.select('path').attr('stroke-width', (edge) => Math.min(4, 1.3 + Math.log2(edge.weight || 1)));
    (animate ? paths.transition().duration(MOVE_MS) : paths).attr('d', (edge) => edgePath(edge, layout.direction));
    merged.select('text')
      .text((edge) => (edge.weight > 1 ? `×${edge.weight}` : ''))
      .attr('x', (edge) => (edge.points.sx + edge.points.tx) / 2)
      .attr('y', (edge) => (edge.points.sy + edge.points.ty) / 2 - 3);
  }

  function drawNodes(layout, animate) {
    const nodes = nodeLayer.selectAll('g.flow-node').data(layout.nodes, (node) => node.id);
    nodes.exit().transition().duration(animate ? 200 : 0).style('opacity', 0).remove();
    const entered = nodes.enter().append('g').attr('class', 'flow-node').style('opacity', 0)
      .attr('transform', (node) => `translate(${node.x},${node.y})`)
      .on('click', (event, node) => { event.stopPropagation(); onNodeClick(node.id); })
      .on('mouseenter', (event, node) => showTooltip(event, node))
      .on('mousemove', (event) => moveTooltip(event))
      .on('mouseleave', () => { tooltipEl.hidden = true; });
    entered.append('rect').attr('class', 'flow-box').attr('rx', 7).attr('height', NODE_HEIGHT);
    entered.append('rect').attr('class', 'flow-stripe').attr('rx', 3).attr('width', 5).attr('height', NODE_HEIGHT - 10).attr('x', 5).attr('y', 5);
    entered.append('text').attr('class', 'flow-name').attr('x', 16).attr('y', 17);
    entered.append('text').attr('class', 'flow-sub').attr('x', 16).attr('y', 31);
    const badge = entered.append('g').attr('class', 'flow-badge');
    badge.append('circle').attr('r', 9);
    badge.append('text').attr('text-anchor', 'middle').attr('dy', '0.35em');

    const merged = entered.merge(nodes);
    merged.attr('data-kind', (node) => node.kind)
      .classed('selected', (node) => node.id === selectedId)
      .classed('focus', (node) => node.focus)
      .classed('changed', (node) => node.kind === 'changed');
    merged.select('rect.flow-box').attr('width', (node) => node.width);
    merged.select('rect.flow-stripe').attr('fill', (node) => (node.kind === 'changed' ? riskColor(node.risk_score) : 'var(--node-muted-stroke)'));
    merged.select('text.flow-name').text((node) => `${node.focus ? '★ ' : ''}${node.label}`);
    merged.select('text.flow-sub').text((node) => node.subLabel);
    (animate ? merged.transition().duration(MOVE_MS) : merged)
      .style('opacity', 1)
      .attr('transform', (node) => `translate(${node.x},${node.y})`);
    refreshBadges();
  }

  function refreshBadges() {
    nodeLayer.selectAll('g.flow-node').each(function updateBadge(node) {
      const findings = findingsForNode(store, node);
      const badge = d3.select(this).select('g.flow-badge');
      badge.attr('transform', `translate(${node.width - 2},2)`).style('display', findings.length ? null : 'none');
      if (!findings.length) return;
      const severity = topSeverity(findings);
      badge.select('circle').attr('fill', `var(${SEVERITY_VARS[severity] || '--sev-suggestion'})`);
      badge.select('text').text(findings.length);
      d3.select(this).attr('data-severity', severity);
    });
  }

  function updateHint(layout) {
    if (!hintEl) return;
    if (selectedId) {
      const node = model.nodes.get(selectedId);
      const callers = layout.nodes.filter((entry) => entry.rank < 0).length;
      const callees = layout.nodes.filter((entry) => entry.rank > 0).length;
      hintEl.textContent = `${node ? node.name : ''}: ${callers} caller${callers === 1 ? '' : 's'} → ${callees} callee${callees === 1 ? '' : 's'}`;
    } else {
      hintEl.textContent = `Whole PR · ${layout.groups.length} call flow${layout.groups.length === 1 ? '' : 's'} · click a box to follow it`;
    }
  }

  function fit({ animate = true } = {}) {
    if (!lastLayout || !lastLayout.nodes.length) return;
    const width = canvasEl.clientWidth || 400;
    const height = canvasEl.clientHeight || 360;
    const bounds = lastLayout.bounds;
    const padding = 24;
    const scale = Math.min(1.25, (width - padding * 2) / Math.max(1, bounds.width), (height - padding * 2) / Math.max(1, bounds.height));
    const translateX = (width - bounds.width * scale) / 2 - bounds.x * scale;
    const translateY = (height - bounds.height * scale) / 2 - bounds.y * scale;
    const transform = d3.zoomIdentity.translate(translateX, translateY).scale(Math.max(0.15, scale));
    (animate ? svg.transition().duration(MOVE_MS) : svg).call(zoom.transform, transform);
  }

  function showTooltip(event, node) {
    const source = store.functions.get(node.id);
    const findings = findingsForNode(store, node);
    const lines = [
      h('div', { class: 'tt-title', text: node.name }),
      h('div', { class: 'tt-sub', text: node.file ? `${node.file}:${node.line}` : 'external' }),
    ];
    if (node.kind === 'changed') lines.push(h('div', { text: `risk ${Math.round(node.risk_score)} · complexity ${node.complexity} · fan-in ${node.fan_in}` }));
    if (source && source.risk_reasons && source.risk_reasons.length) lines.push(h('div', { class: 'tt-reason', text: source.risk_reasons[0] }));
    if (findings.length) {
      lines.push(h('div', { class: 'tt-findings', text: severityCounts(findings).map(([severity, count]) => `${count} ${severity}`).join(' · ') }));
    }
    clear(tooltipEl).append(...lines);
    tooltipEl.hidden = false;
    moveTooltip(event);
  }

  function moveTooltip(event) {
    const bounds = canvasEl.getBoundingClientRect();
    const left = Math.min(event.clientX - bounds.left + 14, bounds.width - 240);
    tooltipEl.style.left = `${Math.max(4, left)}px`;
    tooltipEl.style.top = `${event.clientY - bounds.top + 14}px`;
  }

  svg.on('click', () => { if (selectedId) clearSelection(); });

  function clearSelection() {
    selectedId = null;
    render();
  }

  return {
    focus(id) {
      if (!model.nodes.has(id) || id === selectedId) return;
      selectedId = id;
      render();
    },
    clear: clearSelection,
    fit: () => fit(),
    refreshBadges,
    highlightCallers(id) {
      if (model.nodes.has(id)) {
        selectedId = id;
        render();
      }
      nodeLayer.selectAll('g.flow-node').classed('pulse', (node) => node.rank < 0);
      window.setTimeout(() => nodeLayer.selectAll('g.flow-node').classed('pulse', false), 1600);
    },
    /** L: toggle between the selected function's flow and the whole PR */
    toggleLayered() {
      if (selectedId) {
        model.parkedSelection = selectedId;
        selectedId = null;
      } else if (model.parkedSelection) {
        selectedId = model.parkedSelection;
      }
      render();
    },
    /** Called when the diagram is shown full width (F) or returned to the side panel */
    setExpanded(isExpanded) {
      expanded = isExpanded;
      window.requestAnimationFrame(() => render({ animate: false }));
    },
  };
}

// ---------- Model ----------

/** Adjacency over the review graph, plus traversal helpers */
export function buildModel(graph) {
  const nodes = new Map(graph.nodes.map((node) => [node.id, node]));
  const edges = graph.edges.filter((edge) => nodes.has(edge.from) && nodes.has(edge.to) && edge.from !== edge.to);
  const callers = new Map([...nodes.keys()].map((id) => [id, []]));
  const callees = new Map([...nodes.keys()].map((id) => [id, []]));
  for (const edge of edges) {
    callees.get(edge.from).push(edge.to);
    callers.get(edge.to).push(edge.from);
  }

  /** Signed distance from the focus: negative for callers, positive for callees */
  function distances(focusId, depth) {
    const result = new Map([[focusId, 0]]);
    const walk = (adjacency, sign) => {
      let frontier = [focusId];
      for (let level = 1; level <= depth; level++) {
        const next = [];
        for (const id of frontier) {
          for (const neighbour of adjacency.get(id) || []) {
            if (result.has(neighbour)) continue;
            result.set(neighbour, sign * level);
            next.push(neighbour);
          }
        }
        frontier = next;
      }
    };
    walk(callers, -1);
    walk(callees, 1);
    return result;
  }

  return {
    nodes,
    edges,
    callers,
    callees,
    distances,
    parkedSelection: null,
    focusSet: (focusId, depth) => new Set(distances(focusId, depth).keys()),
  };
}

// ---------- Layout ----------

/**
 * Layered (Sugiyama-style) layout. Ranks come from signed distance to the focus when
 * one is selected, otherwise from longest paths after removing cycles. Order within a
 * rank is refined by barycentre sweeps to reduce crossings. Each weakly connected group
 * is laid out separately and the groups are stacked, riskiest first.
 */
export function layoutFlow(model, visibleIds, { focusId = null, direction = 'TB' } = {}) {
  const nodes = [...visibleIds].map((id) => model.nodes.get(id)).filter(Boolean);
  const edges = model.edges.filter((edge) => visibleIds.has(edge.from) && visibleIds.has(edge.to));
  const components = focusId ? [nodes] : connectedComponents(nodes, edges);

  const placedNodes = [];
  const placedGroups = [];
  let offset = 0;
  for (const component of components) {
    const ids = new Set(component.map((node) => node.id));
    const componentEdges = edges.filter((edge) => ids.has(edge.from) && ids.has(edge.to));
    const ranks = focusId ? focusRanks(model, focusId, ids) : longestPathRanks(component, componentEdges);
    const placed = placeComponent(component, componentEdges, ranks, direction);
    const shift = direction === 'TB' ? { x: offset, y: 0 } : { x: 0, y: offset };
    for (const node of placed.nodes) {
      node.x += shift.x;
      node.y += shift.y;
      placedNodes.push(node);
    }
    if (!focusId && components.length > 1) placedGroups.push(groupBox(placed.nodes, component));
    offset += (direction === 'TB' ? placed.width : placed.height) + COMPONENT_GAP;
  }

  const byId = new Map(placedNodes.map((node) => [node.id, node]));
  const placedEdges = edges.map((edge) => {
    const source = byId.get(edge.from);
    const target = byId.get(edge.to);
    return { ...edge, reversed: target.rank <= source.rank, points: ports(source, target, direction) };
  });
  return { nodes: placedNodes, edges: placedEdges, groups: placedGroups, direction, bounds: boundsOf(placedNodes) };
}

function focusRanks(model, focusId, ids) {
  const distance = model.distances(focusId, FOCUS_DEPTH);
  return new Map([...ids].map((id) => [id, distance.get(id) ?? 0]));
}

/** Longest-path layering on the graph with back edges (found by DFS) ignored */
function longestPathRanks(nodes, edges) {
  const outgoing = new Map(nodes.map((node) => [node.id, []]));
  for (const edge of edges) outgoing.get(edge.from).push(edge.to);

  const backEdges = new Set();
  const state = new Map();
  const visit = (id) => {
    state.set(id, 'active');
    for (const next of outgoing.get(id)) {
      if (state.get(next) === 'active') backEdges.add(`${id}->${next}`);
      else if (!state.has(next)) visit(next);
    }
    state.set(id, 'done');
  };
  // Start from nodes nobody calls, riskiest first, so cycles break in a natural place
  const incoming = new Map(nodes.map((node) => [node.id, 0]));
  for (const edge of edges) incoming.set(edge.to, incoming.get(edge.to) + 1);
  const starts = [...nodes].sort((left, right) => incoming.get(left.id) - incoming.get(right.id) || (right.risk_score || 0) - (left.risk_score || 0));
  for (const node of starts) if (!state.has(node.id)) visit(node.id);

  const forward = edges.filter((edge) => !backEdges.has(`${edge.from}->${edge.to}`));
  const ranks = new Map(nodes.map((node) => [node.id, 0]));
  for (let pass = 0; pass < nodes.length; pass++) {
    let changed = false;
    for (const edge of forward) {
      const candidate = ranks.get(edge.from) + 1;
      if (candidate > ranks.get(edge.to)) {
        ranks.set(edge.to, candidate);
        changed = true;
      }
    }
    if (!changed) break;
  }
  return ranks;
}

/** Orders nodes within ranks and assigns coordinates for one component */
function placeComponent(nodes, edges, ranks, direction) {
  const layers = new Map();
  for (const node of nodes) {
    const rank = ranks.get(node.id);
    if (!layers.has(rank)) layers.set(rank, []);
    layers.get(rank).push({ ...node, rank, width: nodeWidth(node), label: nodeLabel(node), subLabel: nodeSubLabel(node) });
  }
  const rankKeys = [...layers.keys()].sort((left, right) => left - right);
  for (const rank of rankKeys) {
    layers.get(rank).sort((left, right) => kindOrder(left) - kindOrder(right) || (right.risk_score || 0) - (left.risk_score || 0) || left.name.localeCompare(right.name));
  }
  reduceCrossings(layers, rankKeys, edges);

  const placed = [];
  const isVertical = direction === 'TB';
  const rankExtents = [];
  for (const rank of rankKeys) {
    const layer = layers.get(rank);
    const extent = layer.reduce((total, node) => total + (isVertical ? node.width : NODE_HEIGHT), 0) + SIBLING_GAP[direction] * (layer.length - 1);
    rankExtents.push(extent);
  }
  const widest = Math.max(...rankExtents);
  const maxWidthByRank = rankKeys.map((rank) => Math.max(...layers.get(rank).map((node) => node.width)));

  let majorCursor = 0;
  rankKeys.forEach((rank, rankIndex) => {
    const layer = layers.get(rank);
    let minorCursor = (widest - rankExtents[rankIndex]) / 2;
    for (const node of layer) {
      if (isVertical) {
        node.x = minorCursor;
        node.y = majorCursor;
        minorCursor += node.width + SIBLING_GAP.TB;
      } else {
        node.x = majorCursor + (maxWidthByRank[rankIndex] - node.width) / 2;
        node.y = minorCursor;
        minorCursor += NODE_HEIGHT + SIBLING_GAP.LR;
      }
      placed.push(node);
    }
    majorCursor += (isVertical ? NODE_HEIGHT : maxWidthByRank[rankIndex]) + RANK_GAP[direction];
  });

  const majorExtent = majorCursor - RANK_GAP[direction];
  return { nodes: placed, width: isVertical ? widest : majorExtent, height: isVertical ? majorExtent : widest };
}

/** Barycentre heuristic: alternately sweep down and up, ordering each rank by neighbour positions */
function reduceCrossings(layers, rankKeys, edges) {
  const neighbours = (id, wantCallers) => edges
    .filter((edge) => (wantCallers ? edge.to === id : edge.from === id))
    .map((edge) => (wantCallers ? edge.from : edge.to));
  const position = new Map();
  const index = () => {
    for (const rank of rankKeys) layers.get(rank).forEach((node, order) => position.set(node.id, order));
  };
  index();
  for (let sweep = 0; sweep < 6; sweep++) {
    const downward = sweep % 2 === 0;
    const order = downward ? rankKeys.slice(1) : rankKeys.slice(0, -1).reverse();
    for (const rank of order) {
      const layer = layers.get(rank);
      const weight = new Map(layer.map((node, current) => {
        const related = neighbours(node.id, downward).filter((id) => position.has(id));
        const centre = related.length ? related.reduce((sum, id) => sum + position.get(id), 0) / related.length : current;
        return [node.id, centre];
      }));
      layer.sort((left, right) => weight.get(left.id) - weight.get(right.id));
      index();
    }
  }
}

function connectedComponents(nodes, edges) {
  const parent = new Map(nodes.map((node) => [node.id, node.id]));
  const find = (id) => (parent.get(id) === id ? id : (parent.set(id, find(parent.get(id))), parent.get(id)));
  for (const edge of edges) parent.set(find(edge.from), find(edge.to));
  const groups = new Map();
  for (const node of nodes) {
    const root = find(node.id);
    if (!groups.has(root)) groups.set(root, []);
    groups.get(root).push(node);
  }
  const riskOf = (group) => Math.max(...group.map((node) => node.risk_score || 0));
  const changedCount = (group) => group.filter((node) => node.kind === 'changed').length;
  return [...groups.values()].sort((left, right) => riskOf(right) - riskOf(left) || changedCount(right) - changedCount(left));
}

function groupBox(placedNodes, component) {
  const bounds = boundsOf(placedNodes);
  const lead = [...component].sort((left, right) => (right.risk_score || 0) - (left.risk_score || 0))[0];
  const file = lead.file ? lead.file.split('/').pop() : '';
  return {
    key: lead.id,
    label: file,
    x: bounds.x - 10,
    y: bounds.y - 22,
    width: bounds.width + 20,
    height: bounds.height + 32,
  };
}

function boundsOf(nodes) {
  if (!nodes.length) return { x: 0, y: 0, width: 1, height: 1 };
  const minX = Math.min(...nodes.map((node) => node.x));
  const minY = Math.min(...nodes.map((node) => node.y)) - 22;
  const maxX = Math.max(...nodes.map((node) => node.x + node.width));
  const maxY = Math.max(...nodes.map((node) => node.y + NODE_HEIGHT));
  return { x: minX - 10, y: minY, width: maxX - minX + 20, height: maxY - minY + 10 };
}

/** Edge endpoints: bottom-centre to top-centre (TB) or right-middle to left-middle (LR) */
function ports(source, target, direction) {
  if (direction === 'TB') {
    return { sx: source.x + source.width / 2, sy: source.y + NODE_HEIGHT, tx: target.x + target.width / 2, ty: target.y };
  }
  return { sx: source.x + source.width, sy: source.y + NODE_HEIGHT / 2, tx: target.x, ty: target.y + NODE_HEIGHT / 2 };
}

/** Smooth curve along the flow direction; edges that go against the flow loop around the side */
export function edgePath(edge, direction) {
  const { sx, sy, tx, ty } = edge.points;
  if (direction === 'TB') {
    if (ty > sy) {
      const bend = Math.max(24, (ty - sy) / 2);
      return `M${sx},${sy} C${sx},${sy + bend} ${tx},${ty - bend} ${tx},${ty - 2}`;
    }
    const loop = 48;
    return `M${sx},${sy} C${sx + loop},${sy + loop} ${tx + loop},${ty - loop} ${tx},${ty - 2}`;
  }
  if (tx > sx) {
    const bend = Math.max(30, (tx - sx) / 2);
    return `M${sx},${sy} C${sx + bend},${sy} ${tx - bend},${ty} ${tx - 2},${ty}`;
  }
  const loop = 56;
  return `M${sx},${sy} C${sx + loop},${sy + loop} ${tx - loop},${ty + loop} ${tx - 2},${ty}`;
}

// ---------- Node text ----------

function kindOrder(node) {
  return { changed: 0, caller: 1, callee: 1, external: 2 }[node.kind] ?? 1;
}

function nodeLabel(node) {
  const maxChars = Math.floor((NODE_MAX_WIDTH - 40) / CHAR_WIDTH);
  const name = node.name || node.id;
  return name.length > maxChars ? `${name.slice(0, maxChars - 1)}…` : name;
}

function nodeSubLabel(node) {
  if (!node.file) return 'external';
  const base = node.file.split('/').pop();
  const text = `${base}:${node.line}`;
  const maxChars = Math.floor((NODE_MAX_WIDTH - 24) / 6.2);
  return text.length > maxChars ? `…${text.slice(text.length - maxChars + 1)}` : text;
}

function nodeWidth(node) {
  const nameChars = (node.name || node.id).length + (node.focus ? 2 : 0);
  const subChars = node.file ? node.file.split('/').pop().length + 5 : 8;
  const wanted = Math.max(nameChars * CHAR_WIDTH, subChars * 6.2) + 34;
  return Math.round(Math.max(NODE_MIN_WIDTH, Math.min(NODE_MAX_WIDTH, wanted)));
}

function findingsForNode(store, node) {
  const fn = store.functions.get(node.id);
  return fn ? store.findingsForFunction(fn) : [];
}

// ---------- Static SVG parts ----------

function appendMarkers(defs) {
  defs.append('marker')
    .attr('id', 'flow-arrow').attr('viewBox', '0 0 10 10').attr('refX', 9).attr('refY', 5)
    .attr('markerWidth', 7).attr('markerHeight', 7).attr('orient', 'auto-start-reverse')
    .append('path').attr('d', 'M0,0 L10,5 L0,10 z').attr('class', 'flow-arrowhead');
}

function renderLegend(legendEl) {
  if (!legendEl) return;
  const swatch = (className, text) => h('span', { class: 'legend-item' }, h('span', { class: `legend-swatch ${className}` }), text);
  clear(legendEl).append(
    swatch('changed', 'Changed (stripe = risk)'),
    swatch('context', 'Caller / callee'),
    swatch('external', 'External'),
    h('span', { class: 'legend-item' }, h('span', { class: 'legend-arrow', text: '→' }), 'calls'),
    swatch('badge', 'AI findings'),
  );
}
