// Main pane: the selected file's diff with expandable context, changed-function risk
// rails, inline finding cards, a cursor and vim scrolling commands. Files outside the
// diff (a caller's file) render as a read-only window with the call line highlighted.

import { useCallback, useEffect, useMemo, useRef, type RefObject } from 'react';
import { useCommentFindings, useFindings, useReview, useReviewStore } from '../../state/StoreProvider';
import type { Finding } from '../../types';
import { highlightLanguage } from '../../lib/highlight';
import { createCursorStore, CursorContext } from './cursor';
import { CodeRows } from './CodeRows';
import { FileHeader } from './FileHeader';
import { FileNotes } from './FileNotes';
import { findingKey } from './FindingCard';
import { findingAnchorLine, functionRails, placeFindings } from './placement';
import { hiddenLineCount } from './rows';
import { useCodeCommands } from './useCodeCommands';
import { useFileModel } from './useFileModel';
import { useReveal } from './useReveal';
import { useViewTracking } from './useViewTracking';
import { isUserComment } from '../../state/comments';
import './CodeView.css';

/** Keeps --cv-viewport at the scroll container's visible width so cards stay in view */
function useViewportWidth(containerRef: RefObject<HTMLElement>) {
  useEffect(() => {
    const element = containerRef.current;
    if (!element) return undefined;
    const observer = new ResizeObserver(() => element.style.setProperty('--cv-viewport', `${element.clientWidth}px`));
    observer.observe(element);
    return () => observer.disconnect();
  }, [containerRef]);
}

/** AI findings (unless hidden by "Think first") and the reviewer's comments for the file */
function useFileFindings(path: string | null): Finding[] {
  const findings = useFindings();
  const comments = useCommentFindings();
  return useMemo(
    () => [...comments, ...findings].filter((finding) => finding.file === path),
    [comments, findings, path],
  );
}

export function CodeView() {
  const store = useReviewStore();
  const containerRef = useRef<HTMLDivElement>(null);
  const cursor = useMemo(() => createCursorStore(), []);
  const api = useFileModel();
  const { model } = api;
  const callSiteLine = useReview((state) => (state.selection.callSite && state.selection.file === model.path ? state.selection.line : null));
  const wrap = useReview((state) => state.ui.wrap);
  const diff = useReview((state) => (model.path ? state.index.files.get(model.path) : undefined));
  const functions = useReview((state) => (model.path ? state.index.functionsByFile.get(model.path) : undefined));
  const fileFindings = useFileFindings(model.path);
  const aiFileFindings = useMemo(() => fileFindings.filter((finding) => !isUserComment(finding)), [fileFindings]);
  const placement = useMemo(() => placeFindings(model.rows, fileFindings), [model.rows, fileFindings]);
  const rails = useMemo(() => functionRails(model.rows, functions ?? []), [model.rows, functions]);
  const language = highlightLanguage(model.path ?? '', diff?.language);
  const requestScroll = useReveal(containerRef, cursor, api);
  useViewportWidth(containerRef);
  useViewTracking(containerRef, model.status === 'ready' && !model.plain ? model.path : null);

  const placementRef = useRef(placement);
  placementRef.current = placement;
  const activate = useCallback((key: string) => {
    cursor.set(key);
    const id = key.startsWith('f:') ? key.slice(2) : findingIdOnRow(key);
    if (id && store.getState().selection.findingId !== id) {
      store.setState((state) => ({ selection: { ...state.selection, findingId: id } }));
    }
    function findingIdOnRow(rowKey: string): string | null {
      const line = rowKey.startsWith('n') ? Number(rowKey.slice(1)) : 0;
      return placementRef.current.byLine.get(line)?.[0]?.id ?? null;
    }
  }, [cursor, store]);

  const showLine = useCallback(async (finding: Finding) => {
    if (!(await api.revealLine(findingAnchorLine(finding))) && finding.line) await api.revealLine(finding.line);
    const key = findingKey(finding.id);
    requestScroll({ key, placement: 'focus', flash: [key] });
  }, [api, requestScroll]);

  useCodeCommands({ containerRef, cursor, api, placement, rails, requestScroll, activate });

  const onMouseDown = () => {
    if (store.getState().ui.focusedPane !== 'code') store.getState().setFocusedPane('code');
  };

  return (
    <CursorContext.Provider value={cursor}>
      <div className="cv" onMouseDown={onMouseDown}>
        {model.path ? (
          <FileHeader
            path={model.path} diff={diff} plain={model.plain} findings={aiFileFindings}
            hiddenLines={hiddenLineCount(model.rows)} onExpandAll={() => void api.expandAll()}
          />
        ) : null}
        <div ref={containerRef} className={`cv-scroll${wrap ? ' cv-wrap' : ''}${model.plain ? ' cv-plain' : ''}`}>
          <CodeStatus model={model} binary={Boolean(diff?.binary)} />
          <FileNotes placement={placement} path={model.path} language={language} activate={activate} onShowLine={showLine} />
          {model.status === 'ready' ? (
            <CodeRows
              model={model} language={language} placement={placement} rails={rails}
              functions={functions} callSiteLine={callSiteLine} activate={activate} onExpand={api.expand}
            />
          ) : null}
        </div>
      </div>
    </CursorContext.Provider>
  );
}

function CodeStatus({ model, binary }: { model: ReturnType<typeof useFileModel>['model']; binary: boolean }) {
  if (!model.path) return <div className="empty">Select a file or function to review.</div>;
  if (model.status === 'loading') return <div className="empty cv-status"><span className="spinner" /> Loading {model.path}</div>;
  if (model.status === 'error') return <div className="empty cv-status cv-error">Could not load {model.path}: {model.error}</div>;
  if (model.status === 'ready' && binary) return <div className="empty">Binary file not shown.</div>;
  if (model.status === 'ready' && model.rows.length === 0) return <div className="empty">No textual changes.</div>;
  return null;
}
