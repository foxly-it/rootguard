// =====================================================
// File: frontend/src/search/SearchModal.tsx
// Purpose: Global, local-only search. Opens from its own
// header trigger, the "S" key, or Ctrl/Cmd+K; navigates to
// the matching page on selection. Landing on the exact tab
// or section within a page is a follow-up (see ROADMAP.md).
// =====================================================

import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router";
import { Search, X } from "lucide-react";
import { useI18n } from "../i18n";
import { searchIndex, type SearchEntry } from "./data";
import "../styles/search.css";

const isMac = typeof navigator !== "undefined" && /Mac|iPhone|iPad|iPod/.test(navigator.platform ?? navigator.userAgent);
const modKeyLabel = isMac ? "⌘" : "Ctrl";

function isTypingTarget(target: EventTarget | null) {
  const element = target as HTMLElement | null;
  const tag = element?.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || element?.isContentEditable === true;
}

export default function SearchModal() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [highlighted, setHighlighted] = useState(0);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const resultRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const navigate = useNavigate();
  const { t, locale } = useI18n();

  const results = useMemo(() => {
    const words = query.trim().toLocaleLowerCase(locale).split(/\s+/).filter(Boolean);
    if (words.length === 0) return searchIndex;
    return searchIndex.filter((entry) => {
      const haystack = [t(entry.labelKey), t(entry.categoryKey), ...(entry.keywords ?? [])]
        .join(" ")
        .toLocaleLowerCase(locale);
      return words.every((word) => haystack.includes(word));
    });
  }, [query, locale, t]);

  const resultRows = useMemo(() => results.map((entry, index) => ({
    entry,
    showCategory: index === 0 || entry.categoryKey !== results[index - 1].categoryKey,
  })), [results]);

  // Open shortcuts: Ctrl/Cmd+K always; bare "S" only while not typing
  // elsewhere and not already open.
  useEffect(() => {
    function onKeyDown(event: globalThis.KeyboardEvent) {
      const mod = event.metaKey || event.ctrlKey;
      if (mod && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setOpen(true);
        return;
      }
      if (!open && !mod && !event.altKey && event.key.toLowerCase() === "s" && !isTypingTarget(event.target)) {
        event.preventDefault();
        setOpen(true);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    setQuery("");
    setHighlighted(0);
    inputRef.current?.focus();
    document.body.classList.add("modal-open");

    function onKeyDown(event: globalThis.KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
        triggerRef.current?.focus();
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.body.classList.remove("modal-open");
    };
  }, [open]);

  useEffect(() => {
    setHighlighted(0);
  }, [query]);

  useEffect(() => {
    resultRefs.current[highlighted]?.scrollIntoView({ block: "nearest" });
  }, [highlighted]);

  function select(entry: SearchEntry) {
    setOpen(false);
    triggerRef.current?.focus();
    navigate(entry.route);
  }

  function onInputKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setHighlighted((current) => Math.min(current + 1, results.length - 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setHighlighted((current) => Math.max(current - 1, 0));
    } else if (event.key === "Enter") {
      event.preventDefault();
      const entry = results[highlighted];
      if (entry) select(entry);
    }
  }

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className="rg-link rg-search-trigger"
        onClick={() => setOpen(true)}
        aria-label={t("search.hint", { mod: modKeyLabel })}
        title={t("search.hint", { mod: modKeyLabel })}
      >
        <Search aria-hidden="true" />
        <span>{t("search.trigger")}</span>
      </button>

      {open && createPortal((
        <div
          className="rg-search-backdrop"
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) {
              setOpen(false);
              triggerRef.current?.focus();
            }
          }}
        >
          <div className="rg-search-panel" role="dialog" aria-modal="true" aria-label={t("search.trigger")}>
            <div className="rg-search-field">
              <Search aria-hidden="true" />
              <input
                ref={inputRef}
                type="text"
                value={query}
                placeholder={t("search.placeholder")}
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={onInputKeyDown}
                role="combobox"
                aria-expanded="true"
                aria-controls="rg-search-results"
                aria-activedescendant={results[highlighted] ? `rg-search-result-${results[highlighted].id}` : undefined}
              />
              <button type="button" className="rg-search-close" onClick={() => { setOpen(false); triggerRef.current?.focus(); }} aria-label={t("search.close")}>
                <X aria-hidden="true" />
              </button>
            </div>

            <div className="rg-search-results" id="rg-search-results" role="listbox">
              {results.length === 0 && (
                <p className="rg-search-empty">{t("search.noResults", { query })}</p>
              )}
              {resultRows.map(({ entry, showCategory }, index) => {
                return (
                  <div key={entry.id}>
                    {showCategory && <div className="rg-search-category">{t(entry.categoryKey)}</div>}
                    <button
                      id={`rg-search-result-${entry.id}`}
                      ref={(element) => { resultRefs.current[index] = element; }}
                      type="button"
                      role="option"
                      aria-selected={index === highlighted}
                      className={`rg-search-result${index === highlighted ? " active" : ""}`}
                      onMouseEnter={() => setHighlighted(index)}
                      onClick={() => select(entry)}
                    >
                      {t(entry.labelKey)}
                    </button>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      ), document.body)}
    </>
  );
}
