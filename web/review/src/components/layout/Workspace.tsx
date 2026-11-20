// Three-pane workspace: sidebar | code | context, with draggable hairline dividers,
// collapse/restore rails, the h/l pane focus line and the full-width flow mode.
// Below 768px the sidebar starts collapsed; below 640px the panes stack vertically.

import { useEffect, useRef, type CSSProperties, type ReactNode } from 'react';
import { useFindings, useReview, useReviewShallow } from '../../state/StoreProvider';
import type { PaneName } from '../../state/store';
import { reviewBadge } from './tabBadges';
import { clampWidth, DEFAULT_WIDTHS, effectiveWidths, MIN_WIDTH, type Side } from './layoutMath';
import { useElementWidth, useMediaQuery, usePaneWidths } from './usePaneWidths';
import { Divider, RestoreRail } from './Divider';
import './Workspace.css';

const NARROW_QUERY = '(max-width: 768px)';

interface WorkspaceProps {
  sidebar: ReactNode;
  code: ReactNode;
  context: ReactNode;
}

export function Workspace({ sidebar, code, context }: WorkspaceProps) {
  const ui = useReviewShallow((state) => ({
    collapsed: state.ui.collapsed, flowExpanded: state.ui.flowExpanded, focusedPane: state.ui.focusedPane,
  }));
  const setCollapsed = useReview((state) => state.setCollapsed);
  const railBadge = reviewBadge(useFindings());
  const ref = useRef<HTMLElement>(null);
  const workspaceWidth = useElementWidth(ref);
  const widths = usePaneWidths();
  const effective = effectiveWidths(widths.preferred, workspaceWidth, ui.collapsed);
  useNarrowDefaults(setCollapsed);

  const resize = (side: Side) => (desired: number) => {
    const other = side === 'sidebar' ? effective.context : effective.sidebar;
    widths.setWidth(side, workspaceWidth > 0 ? clampWidth(side, desired, workspaceWidth, other) : Math.max(MIN_WIDTH[side], desired));
  };
  const reset = (side: Side) => () => {
    widths.setWidth(side, DEFAULT_WIDTHS[side]);
    widths.commit();
  };

  const className = [
    'workspace',
    ui.collapsed.sidebar ? 'sidebar-collapsed' : '',
    ui.collapsed.context ? 'context-collapsed' : '',
    ui.flowExpanded ? 'flow-expanded' : '',
  ].filter(Boolean).join(' ');
  const style = { '--ws-sidebar-w': `${effective.sidebar}px`, '--ws-context-w': `${effective.context}px` } as CSSProperties;

  return (
    <main ref={ref} className={className} style={style}>
      {ui.collapsed.sidebar
        ? <RestoreRail side="sidebar" label="Changes" onRestore={() => setCollapsed('sidebar', false)} />
        : <Divider side="sidebar" width={effective.sidebar} onResize={resize('sidebar')} onCommit={widths.commit}
            onCollapse={() => setCollapsed('sidebar', true)} onReset={reset('sidebar')} />}
      {ui.collapsed.context
        ? <RestoreRail side="context" label="Context" badge={railBadge} onRestore={() => setCollapsed('context', false)} />
        : <Divider side="context" width={effective.context} onResize={resize('context')} onCommit={widths.commit}
            onCollapse={() => setCollapsed('context', true)} onReset={reset('context')} />}
      <Pane name="sidebar" label="Changes" focused={ui.focusedPane === 'sidebar'} hidden={ui.collapsed.sidebar}>{sidebar}</Pane>
      <Pane name="code" label="Code" focused={ui.focusedPane === 'code'} hidden={false}>{code}</Pane>
      <Pane name="context" label="Context" focused={ui.focusedPane === 'context'} hidden={ui.collapsed.context}>{context}</Pane>
    </main>
  );
}

interface PaneProps {
  name: PaneName;
  label: string;
  focused: boolean;
  hidden: boolean;
  children: ReactNode;
}

/** A pane: clicking it focuses it; keyboard focus (h/l) moves DOM focus into it */
function Pane({ name, label, focused, hidden, children }: PaneProps) {
  const setFocusedPane = useReview((state) => state.setFocusedPane);
  const ref = useRef<HTMLElement>(null);
  const wasFocused = useRef(focused);

  useEffect(() => {
    const element = ref.current;
    if (focused && !wasFocused.current && element && !element.contains(document.activeElement)) {
      element.focus({ preventScroll: true });
    }
    wasFocused.current = focused;
  }, [focused]);

  return (
    <section
      ref={ref} tabIndex={-1} aria-label={label} hidden={hidden}
      className={`pane pane-${name}${focused ? ' focused' : ''}`}
      onPointerDownCapture={() => { if (!focused) setFocusedPane(name); }}
    >
      {children}
    </section>
  );
}

/** Collapse the sidebar when the window becomes narrow (on load or on resize) */
function useNarrowDefaults(setCollapsed: (pane: Side, collapsed: boolean) => void) {
  const narrow = useMediaQuery(NARROW_QUERY);
  useEffect(() => {
    if (narrow) setCollapsed('sidebar', true);
  }, [narrow, setCollapsed]);
}
