// Installs the vim-style keymap on the document. Store-level actions are handled here;
// everything else goes to commands registered by components (see state/commands.ts).
// While focus is in a text field or a modal is open, only Esc is handled.

import { useEffect, useRef, useState } from 'react';
import { createKeyParser, keyToken, type Action, type Binding, type ParseResult } from '../keys/keymap';
import { runRegisteredCommand } from '../state/commands';
import { useFindings, useReviewStore, useSidebarGroups } from '../state/StoreProvider';
import { orderedFindings, type TabName } from '../state/store';
import { sidebarFiles, sidebarFunctions } from '../lib/grouping';

export interface KeymapStatus {
  /** Pending count/sequence, e.g. "5" or "g" */
  pending: string;
  mode: 'NORMAL' | 'INPUT';
}

function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false;
  return Boolean(target.closest('input, textarea, select, [contenteditable="true"]'));
}

export function useKeymap(): KeymapStatus {
  const store = useReviewStore();
  const groups = useSidebarGroups();
  const findings = useFindings();
  const [status, setStatus] = useState<KeymapStatus>({ pending: '', mode: 'NORMAL' });

  // Handlers read the latest derived data through refs so the listener is installed once
  const latest = useRef({ groups, findings });
  latest.current = { groups, findings };

  useEffect(() => {
    const parser = createKeyParser();
    let timer: number | undefined;

    const step = <T>(list: T[], currentIndex: number, delta: number, wrap: boolean): T | undefined => {
      if (!list.length) return undefined;
      if (currentIndex === -1) return delta > 0 ? list[0] : list[list.length - 1];
      const next = wrap
        ? (((currentIndex + delta) % list.length) + list.length) % list.length
        : Math.max(0, Math.min(list.length - 1, currentIndex + delta));
      return list[next];
    };

    const storeHandlers: Partial<Record<Action, (count: number, binding: Binding) => void>> = {
      nextFunction: (count) => stepFunction(count),
      prevFunction: (count) => stepFunction(-count),
      nextFile: (count) => stepFile(count),
      prevFile: (count) => stepFile(-count),
      nextFinding: (count) => stepFinding(count),
      prevFinding: (count) => stepFinding(-count),
      toggleInclude: () => {
        const state = store.getState();
        const id = state.selection.findingId;
        if (id) state.setIncluded(id, !state.included[id]);
      },
      back: () => store.getState().goBack(),
      focusLeft: () => store.getState().moveFocus(-1),
      focusRight: () => store.getState().moveFocus(1),
      nextTab: (count) => store.getState().cycleTab(count),
      prevTab: (count) => store.getState().cycleTab(-count),
      tab: (_count, binding) => store.getState().setTab((binding.argument ?? 'review') as TabName),
      toggleWrap: () => store.getState().toggleWrap(),
      expandFlow: () => store.getState().toggleFlowExpanded(),
      toggleLayered: () => store.getState().toggleFlowScope(),
      post: () => store.getState().setPostOpen(true),
      help: () => store.getState().setHelpOpen(true),
    };

    function stepFunction(delta: number) {
      const list = sidebarFunctions(latest.current.groups);
      const state = store.getState();
      const currentIndex = list.findIndex((fn) => fn.id === state.selection.functionId);
      const target = step(list, currentIndex, delta, false);
      if (target) state.selectFunction(target.id);
    }

    function stepFile(delta: number) {
      const list = sidebarFiles(latest.current.groups);
      const state = store.getState();
      const currentIndex = list.findIndex((file) => file.path === state.selection.file);
      const target = step(list, currentIndex, delta, false);
      if (target) state.openFile(target.path);
    }

    function stepFinding(delta: number) {
      const list = orderedFindings(latest.current.findings, sidebarFiles(latest.current.groups));
      const state = store.getState();
      const currentIndex = list.findIndex((finding) => finding.id === state.selection.findingId);
      const target = step(list, currentIndex, delta, true);
      if (target) state.openFinding(target.id);
    }

    function run(result: ParseResult | null) {
      if (!result || result.type !== 'action') return;
      const { binding, count } = result;
      if (binding.action === 'escape') {
        const state = store.getState();
        if (state.ui.helpOpen) state.setHelpOpen(false);
        else if (state.ui.postOpen) state.setPostOpen(false);
        else if (state.ui.flowExpanded) state.toggleFlowExpanded();
        runRegisteredCommand('escape', count, binding);
        return;
      }
      const handler = storeHandlers[binding.action];
      if (handler) handler(count, binding);
      runRegisteredCommand(binding.action, count, binding);
    }

    const report = (mode: KeymapStatus['mode'] = 'NORMAL') => setStatus({ pending: parser.display(), mode });

    function scheduleExpiry() {
      window.clearTimeout(timer);
      if (!parser.isPending()) return;
      timer = window.setTimeout(() => {
        run(parser.expire());
        report();
      }, Math.max(0, parser.deadline() - Date.now()) + 5);
    }

    function onKeyDown(event: KeyboardEvent) {
      const token = keyToken(event);
      if (!token) return;
      const ui = store.getState().ui;
      const modalOpen = ui.helpOpen || ui.postOpen;
      if (isTypingTarget(event.target)) {
        if (token === 'Esc' && !modalOpen && event.target instanceof HTMLElement) {
          event.target.blur();
          event.preventDefault();
          report();
        }
        return;
      }
      if (modalOpen && token !== 'Esc') return;
      const result = parser.feed(token);
      if (result.type !== 'none') event.preventDefault();
      run(result);
      report();
      scheduleExpiry();
    }

    function onFocusIn(event: FocusEvent) {
      if (isTypingTarget(event.target)) setStatus({ pending: '', mode: 'INPUT' });
    }
    function onFocusOut() {
      window.setTimeout(() => {
        if (!isTypingTarget(document.activeElement)) report();
      }, 0);
    }

    document.addEventListener('keydown', onKeyDown);
    document.addEventListener('focusin', onFocusIn);
    document.addEventListener('focusout', onFocusOut);
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener('keydown', onKeyDown);
      document.removeEventListener('focusin', onFocusIn);
      document.removeEventListener('focusout', onFocusOut);
    };
  }, [store]);

  return status;
}
