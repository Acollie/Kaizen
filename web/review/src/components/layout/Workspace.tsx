// Three-pane workspace: sidebar | code | context, with draggable dividers and collapse.
// OWNER: shell agent (dividers, collapse buttons, responsive stacking, flow-expanded mode).

import type { ReactNode } from 'react';
import { useReview } from '../../state/StoreProvider';
import './Workspace.css';

export function Workspace({ sidebar, code, context }: { sidebar: ReactNode; code: ReactNode; context: ReactNode }) {
  const ui = useReview((state) => state.ui);
  const className = [
    'workspace',
    ui.collapsed.sidebar ? 'sidebar-collapsed' : '',
    ui.collapsed.context ? 'context-collapsed' : '',
    ui.flowExpanded ? 'flow-expanded' : '',
  ].filter(Boolean).join(' ');
  return (
    <main className={className}>
      <section className={`pane pane-sidebar${ui.focusedPane === 'sidebar' ? ' focused' : ''}`} aria-label="Changes">{sidebar}</section>
      <section className={`pane pane-code${ui.focusedPane === 'code' ? ' focused' : ''}`} aria-label="Code">{code}</section>
      <section className={`pane pane-context${ui.focusedPane === 'context' ? ' focused' : ''}`} aria-label="Context">{context}</section>
    </main>
  );
}
