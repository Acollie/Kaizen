// Notice shown in place of a generated file's diff. The diff was dropped from the UI
// and the AI, so the reviewer is told why, and can still read the head version.

import type { FileDiff } from '../../types';
import { withheldReason } from './withheld';

export interface WithheldNoticeProps {
  diff: FileDiff;
  /** The head version is loaded below */
  shown: boolean;
  loading: boolean;
  onShow: () => void;
}

export function WithheldNotice({ diff, shown, loading, onShow }: WithheldNoticeProps) {
  const { reason, hint } = withheldReason(diff);
  return (
    <div className="cv-withheld" role="note">
      <p className="cv-withheld-text">
        <span className="cv-withheld-title">Diff withheld: {reason}. Not shown to the AI.</span>
        {hint ? <span className="cv-withheld-hint">{hint.charAt(0).toUpperCase() + hint.slice(1)}.</span> : null}
        {shown ? <span className="cv-withheld-hint">Showing the head version, read-only. The AI did not see it.</span> : null}
      </p>
      {shown ? null : (
        <button type="button" className="btn btn-outline" onClick={onShow} disabled={loading}>
          {loading ? 'Loading…' : 'Show file anyway'}
        </button>
      )}
    </div>
  );
}
