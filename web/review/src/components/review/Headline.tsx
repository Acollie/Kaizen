// The review verdict, or live progress while Claude is still reviewing, or the error.

import { useReview } from '../../state/StoreProvider';
import type { Finding } from '../../types';
import { AgentTag } from '../shared/bits';
import { engineLabel } from '../header/headerModel';
import { countsSentence, verdictView } from './reviewModel';

export function Headline({ findings, hidden = false }: { findings: readonly Finding[]; hidden?: boolean }) {
  const status = useReview((state) => state.status);
  const result = useReview((state) => state.result);

  if (hidden && status.state !== 'error' && (result || status.state === 'done')) {
    return (
      <div className="rv-headline">
        <div className="rv-verdict">AI review ready</div>
        <div className="rv-counts rv-hidden-note">Verdict and findings hidden while you think first.</div>
      </div>
    );
  }

  if (result) {
    const verdict = verdictView(result.verdict);
    return (
      <div className={`rv-headline rv-tone-${verdict.tone}`}>
        <div className="rv-verdict"><span className="sev-dot" aria-hidden="true" />{verdict.label}</div>
        <div className="rv-counts">{countsSentence(findings)}</div>
        <div className="rv-engine mono">{engineLabel(result)}</div>
      </div>
    );
  }

  if (status.state === 'error') {
    return (
      <div className="rv-headline rv-tone-critical" role="alert">
        <div className="rv-verdict"><span className="sev-dot" aria-hidden="true" />Review failed</div>
        <div className="rv-counts rv-error-text">{status.message}</div>
        {findings.length > 0 && <div className="rv-engine">{countsSentence(findings)} before the failure</div>}
      </div>
    );
  }

  if (status.state === 'done') {
    return (
      <div className="rv-headline">
        <div className="rv-verdict">{status.message || 'Finished'}</div>
        <div className="rv-counts">{findings.length ? countsSentence(findings) : 'The engine finished without a review result.'}</div>
      </div>
    );
  }

  return (
    <div className="rv-headline rv-running" aria-live="polite">
      <div className="rv-verdict"><span className="spinner" aria-hidden="true" />Claude is reviewing</div>
      <div className="rv-counts">{hidden ? 'Findings hidden while you think first' : findings.length ? `${countsSentence(findings)} so far` : 'No findings yet'}</div>
      <div className="rv-now" title={status.message}>
        {status.agent && <AgentTag agent={status.agent} />}
        <span className="rv-now-text mono">{status.message}</span>
      </div>
    </div>
  );
}
