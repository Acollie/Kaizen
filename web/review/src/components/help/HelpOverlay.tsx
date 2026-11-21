// Keyboard shortcuts overlay generated from BINDINGS (keys/keymap.ts), so it can never
// drift from what the keys do. Opened with ?; Esc or a click outside closes it.

import { Fragment, useMemo } from 'react';
import { useReview } from '../../state/StoreProvider';
import { helpGroups, verifyHelpGroup, type HelpGroup } from '../../keys/keymap';
import { ModalDialog } from '../layout/ModalDialog';
import { splitKeyLabel } from './keyLabels';
import './HelpOverlay.css';

export function HelpOverlay() {
  const open = useReview((state) => state.ui.helpOpen);
  const setHelpOpen = useReview((state) => state.setHelpOpen);
  const groups = useMemo(() => [...helpGroups(), verifyHelpGroup()], []);
  return (
    <ModalDialog open={open} onClose={() => setHelpOpen(false)} labelledBy="hp-title" className="hp-dialog">
      <header className="modal-head">
        <h2 id="hp-title" className="modal-title">Keyboard shortcuts</h2>
        <button type="button" className="btn modal-close" onClick={() => setHelpOpen(false)} aria-label="Close">×</button>
      </header>
      <div className="modal-body hp-grid" tabIndex={-1} data-autofocus>
        {groups.map((group) => <HelpSection key={group.group} group={group} />)}
      </div>
      <footer className="modal-foot hp-foot">
        <span>Prefix a count to repeat: <kbd>5</kbd><kbd>j</kbd></span>
        <span>Close <kbd>Esc</kbd></span>
      </footer>
    </ModalDialog>
  );
}

function HelpSection({ group }: { group: HelpGroup }) {
  return (
    <section className="hp-group">
      <h3 className="label hp-group-title">{group.group}</h3>
      <dl className="hp-rows">
        {group.rows.map((row) => (
          <Fragment key={row.description}>
            <dt className="hp-keys">
              {row.labels.map((label, index) => (
                <span key={label} className="hp-alt">
                  {index > 0 && <span className="hp-or">or</span>}
                  {splitKeyLabel(label).map((key, keyIndex) => <kbd key={keyIndex}>{key}</kbd>)}
                </span>
              ))}
            </dt>
            <dd className="hp-desc">{row.description}</dd>
          </Fragment>
        ))}
      </dl>
    </section>
  );
}
