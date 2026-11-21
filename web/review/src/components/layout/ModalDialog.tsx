// Native <dialog> driven by a boolean: showModal() when open, close() when not.
// Esc (the dialog's cancel event) and clicks on the backdrop call onClose. The element
// marked data-autofocus receives focus on open (otherwise the browser picks the first control).

import { useEffect, useRef, type MouseEvent, type ReactNode } from 'react';
import './ModalDialog.css';

interface ModalDialogProps {
  open: boolean;
  onClose(): void;
  /** id of the element that names the dialog */
  labelledBy: string;
  className?: string;
  /** When false, Esc and backdrop clicks are ignored (e.g. while a request is in flight) */
  dismissible?: boolean;
  children: ReactNode;
}

export function ModalDialog({ open, onClose, labelledBy, className = '', dismissible = true, children }: ModalDialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const pressedBackdrop = useRef(false);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) {
      showDialog(dialog);
      dialog.querySelector<HTMLElement>('[data-autofocus]')?.focus({ preventScroll: true });
    }
    if (!open && dialog.open) {
      if (typeof dialog.close === 'function') dialog.close();
      else dialog.removeAttribute('open');
    }
  }, [open]);

  const onBackdropDown = (event: MouseEvent<HTMLDialogElement>) => {
    pressedBackdrop.current = event.target === event.currentTarget;
  };
  const onBackdropClick = (event: MouseEvent<HTMLDialogElement>) => {
    if (dismissible && pressedBackdrop.current && event.target === event.currentTarget) onClose();
    pressedBackdrop.current = false;
  };

  return (
    <dialog
      ref={ref} className={`modal ${className}`} aria-labelledby={labelledBy}
      onMouseDown={onBackdropDown} onClick={onBackdropClick}
      onCancel={(event) => { event.preventDefault(); if (dismissible) onClose(); }}
      onClose={() => { if (open) onClose(); }}
    >
      <div className="modal-surface">{children}</div>
    </dialog>
  );
}

/** showModal() where supported (jsdom lacks it), otherwise the open attribute */
function showDialog(dialog: HTMLDialogElement) {
  if (typeof dialog.showModal === 'function') {
    try {
      dialog.showModal();
      return;
    } catch {
      // Not connected yet or already open non-modally: fall through
    }
  }
  dialog.setAttribute('open', '');
}
