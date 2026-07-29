import { useEffect, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Maximize2, X } from "lucide-react";
import "../styles/content-modal.css";

export default function ContentModal({ open, title, eyebrow, closeLabel, onClose, children }: {
  open: boolean;
  title: string;
  eyebrow?: string;
  closeLabel: string;
  onClose: () => void;
  children: ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", closeOnEscape);
    document.body.classList.add("modal-open");
    return () => {
      document.removeEventListener("keydown", closeOnEscape);
      document.body.classList.remove("modal-open");
    };
  }, [onClose, open]);

  if (!open) return null;
  return createPortal((
    <div className="content-modal-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section className="content-modal" role="dialog" aria-modal="true" aria-labelledby="content-modal-title">
        <header>
          <div>
            {eyebrow && <span>{eyebrow}</span>}
            <h2 id="content-modal-title"><Maximize2 size={18} /> {title}</h2>
          </div>
          <button type="button" onClick={onClose} aria-label={closeLabel}><X size={19} /></button>
        </header>
        <div className="content-modal-body">{children}</div>
      </section>
    </div>
  ), document.body);
}
