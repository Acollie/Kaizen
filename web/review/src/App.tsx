// App shell: header, the three panes (changes sidebar, code view, context panel) and
// overlays. Engine events stream into the store; the keymap is installed here.

import { useEffect } from 'react';
import { useEventStream } from './hooks/useEventStream';
import { useKeymap } from './hooks/useKeymap';
import { useReview, useReviewStore } from './state/StoreProvider';
import { Header } from './components/header/Header';
import { Sidebar } from './components/sidebar/Sidebar';
import { MainPane } from './components/overview/MainPane';
import { ContextPanel } from './components/layout/ContextPanel';
import { Workspace } from './components/layout/Workspace';
import { ModeIndicator } from './components/layout/ModeIndicator';
import { HelpOverlay } from './components/help/HelpOverlay';
import { PostModal } from './components/post/PostModal';
import { VerifyQueue } from './components/verify/VerifyQueue';
import './App.css';

export function App() {
  useEventStream();
  const keymap = useKeymap();
  useInitialSelection();
  const pr = useReview((state) => state.context.pr);

  useEffect(() => {
    document.title = `#${pr.number} ${pr.title} · Kaizen Review`;
  }, [pr.number, pr.title]);

  return (
    <div className="app">
      <Header />
      <Workspace sidebar={<Sidebar />} code={<MainPane />} context={<ContextPanel />} />
      <ModeIndicator status={keymap} />
      <HelpOverlay />
      <PostModal />
      <VerifyQueue />
    </div>
  );
}

/** Select the riskiest function (or first file) on load, staying on the Overview */
function useInitialSelection() {
  const store = useReviewStore();
  useEffect(() => {
    const state = store.getState();
    if (state.selection.file) return;
    const functions = state.context.functions ?? [];
    const first = functions.find((fn) => fn.focus) ?? functions[0];
    if (first) state.selectFunction(first.id, { record: false, showCode: false });
    else if (state.context.files?.[0]) state.openFile(state.context.files[0].path, { record: false, showCode: false });
  }, [store]);
}
