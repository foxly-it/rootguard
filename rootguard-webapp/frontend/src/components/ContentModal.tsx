import { useEffect, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Maximize2, X } from "lucide-react";
import "../styles/content-modal.css";

const FOCUSABLE_SELECTOR = 'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

export default function ContentModal({ open, title, eyebrow, closeLabel, size = "large", onClose, children }: {
  open: boolean;
  title: string;
  eyebrow?: string;
  closeLabel: string;
  size?: "large" | "medium";
  onClose: () => void;
  children: ReactNode;
}) {
  const panelRef = useRef<HTMLElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const triggerRef = useRef<Element | null>(null);

  useEffect(() => {
    if (!open) return;
    triggerRef.current = document.activeElement;
    closeButtonRef.current?.focus();

    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") { onClose(); return; }
      if (event.key !== "Tab" || !panelRef.current) return;
      // A dialog is a keyboard dead end otherwise - Tab must cycle within
      // it, not leak into whatever the backdrop is covering.
      const focusable = Array.from(panelRef.current.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter((el) => el.offsetParent !== null);
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", onKeyDown);
    document.body.classList.add("modal-open");
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.body.classList.remove("modal-open");
      (triggerRef.current as HTMLElement | null)?.focus?.();
    };
  }, [onClose, open]);

  if (!open) return null;
  return createPortal((
    <div className="content-modal-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section ref={panelRef} className={`content-modal ${size === "medium" ? "content-modal-medium" : ""}`} role="dialog" aria-modal="true" aria-labelledby="content-modal-title">
        <header>
          <div>
            {eyebrow && <span>{eyebrow}</span>}
            <h2 id="content-modal-title"><Maximize2 size={18} /> {title}</h2>
          </div>
          <button ref={closeButtonRef} type="button" onClick={onClose} aria-label={closeLabel}><X size={19} /></button>
        </header>
        <div className="content-modal-body">{children}</div>
      </section>
    </div>
  ), document.body);
}
