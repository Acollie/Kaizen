// Findings that cannot sit under a visible line: file-level findings, and findings on
// lines outside the shown hunks (each with a "Show line N" control).

import type { Finding } from '../../types';
import type { ActivateHandler } from './DiffRows';
import { FindingSlot } from './CodeRows';
import type { FindingPlacement } from './placement';

export interface FileNotesProps {
  placement: FindingPlacement;
  language: string;
  activate: ActivateHandler;
  onShowLine: (finding: Finding) => void;
}

export function FileNotes({ placement, language, activate, onShowLine }: FileNotesProps) {
  const { fileLevel, outside } = placement;
  if (!fileLevel.length && !outside.length) return null;
  return (
    <section className="cv-notes cv-sticky">
      {fileLevel.length ? (
        <>
          <div className="label cv-notes-title">File</div>
          <FindingSlot findings={fileLevel} language={language} activate={activate} />
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
