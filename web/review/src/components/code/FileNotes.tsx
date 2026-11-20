// Findings that cannot sit under a visible line: file-level findings, and findings on
// lines outside the shown hunks (each with a "Show line N" control).

import type { Finding } from '../../types';
import type { ActivateHandler } from './DiffRows';
import { ComposerSlot, FindingSlot } from './CodeRows';
import { useReview } from '../../state/StoreProvider';
import type { FindingPlacement } from './placement';

export interface FileNotesProps {
  placement: FindingPlacement;
  path: string | null;
  language: string;
  activate: ActivateHandler;
  onShowLine: (finding: Finding) => void;
}

export function FileNotes({ placement, path, language, activate, onShowLine }: FileNotesProps) {
  const { fileLevel, outside } = placement;
  const composer = useReview((state) => {
    const target = state.ui.composer;
    return target && target.file === path && !target.line && !target.editingId ? target : null;
  });
  if (!fileLevel.length && !outside.length && !composer) return null;
  return (
    <section className="cv-notes cv-sticky">
      {fileLevel.length || composer ? (
        <>
          <div className="label cv-notes-title">File</div>
          {fileLevel.length ? <FindingSlot findings={fileLevel} language={language} activate={activate} /> : null}
          {composer ? <ComposerSlot target={composer} /> : null}
        </>
      ) : null}
      {outside.length ? (
        <>
          <div className="label cv-notes-title">Outside the visible lines</div>
          <FindingSlot findings={outside} language={language} activate={activate} onShowLine={onShowLine} />
        </>
      ) : null}
    </section>
  );
}
