import { useCallback, useEffect, useState } from "react";
import {
  fetchUnboundSettings,
  previewUnboundSettings,
  updateUnboundSettings,
  type UnboundPreview,
  type UnboundSettings,
} from "../api/client";

export function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

interface UseUnboundDraftWorkflowOptions {
  version?: string;
  onActivated: () => Promise<void>;
  loadErrorMessage: string;
  concurrentMessage: string;
  previewRejectedMessage: string;
  confirmActivateMessage: string;
  activateErrorMessage: string;
  /** Defaults to identity. Private domains and forward zones backfill
   *  defaulted sub-fields (reverse-zone policy, allow_unsigned/allow_private_addresses)
   *  so a freshly-loaded value and a re-fetched one compare equal. */
  normalize?: (settings: UnboundSettings) => UnboundSettings;
  /** Defaults to a whole-object compare. Guided zones and router import
   *  narrow this to only the field they own (local_zones), so an unrelated
   *  change elsewhere in the settings doesn't block their own activation. */
  sameSettings?: (a: UnboundSettings, b: UnboundSettings) => boolean;
  /** Runs after every successful load (including the post-activate reload),
   *  with the freshly normalized settings - the hook keeps no field-specific
   *  draft state of its own, so callers use this to seed theirs. */
  onLoad?: (settings: UnboundSettings) => void;
}

const defaultSameSettings = (a: UnboundSettings, b: UnboundSettings) => JSON.stringify(a) === JSON.stringify(b);

/**
 * The draft -> preview -> validate -> activate workflow shared by every
 * guided Unbound settings surface (guided zones, router import, private
 * domains, forward zones): fetch the current settings, let the caller edit
 * its own slice of them, re-fetch and optimistically check nothing else
 * changed underneath before building a candidate, preview it against
 * Unbound, then activate with the same concurrency check repeated.
 *
 * Deliberately does NOT own the draft itself - each consumer edits a
 * different field (or two, for private domains) with a different shape, so
 * "what changed" has to stay caller-side. What's shared is everything
 * around that: the fetch/reload lifecycle, the concurrency-checked
 * preview/activate calls, and the busy/message/error bookkeeping.
 */
export function useUnboundDraftWorkflow({
  version,
  onActivated,
  loadErrorMessage,
  concurrentMessage,
  previewRejectedMessage,
  confirmActivateMessage,
  activateErrorMessage,
  normalize = (settings) => settings,
  sameSettings = defaultSameSettings,
  onLoad,
}: UseUnboundDraftWorkflowOptions) {
  const [source, setSource] = useState<UnboundSettings | null>(null);
  const [preview, setPreview] = useState<UnboundPreview | null>(null);
  const [candidate, setCandidate] = useState<UnboundSettings | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    const settings = normalize(await fetchUnboundSettings());
    setSource(settings);
    setPreview(null);
    setCandidate(null);
    setError("");
    onLoad?.(settings);
    return settings;
    // normalize/onLoad are passed fresh every render by callers that close
    // over local state setters - re-creating `load` every time they change
    // would fire the mount effect below in a loop instead of once per
    // version change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    load().catch((cause: unknown) => setError(errorMessage(cause, loadErrorMessage)));
    // Deliberately excludes the locale/message strings from the dependency
    // list - every consumer previously included `t` here, meaning a plain
    // language switch silently re-fetched settings from the server. Only a
    // new `version` (a fresh history entry) should trigger that.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [load, version]);

  function resetPreview() {
    setPreview(null);
    setCandidate(null);
  }

  async function createPreview(
    buildCandidate: (active: UnboundSettings) => UnboundSettings,
    onPreviewStart?: (proposed: UnboundSettings) => Promise<void>,
  ): Promise<UnboundPreview | null> {
    if (!source || busy) return null;
    setBusy(true);
    setMessage("");
    setError("");
    try {
      const active = normalize(await fetchUnboundSettings());
      if (!sameSettings(active, source)) throw new Error(concurrentMessage);
      const proposed = buildCandidate(active);
      const [result] = await Promise.all([
        previewUnboundSettings(proposed),
        onPreviewStart ? onPreviewStart(proposed) : Promise.resolve(),
      ]);
      setCandidate(proposed);
      setPreview(result);
      return result;
    } catch (cause) {
      resetPreview();
      setError(errorMessage(cause, previewRejectedMessage));
      return null;
    } finally {
      setBusy(false);
    }
  }

  async function activate(): Promise<boolean> {
    if (!source || !candidate || !preview?.changed || busy) return false;
    if (!window.confirm(confirmActivateMessage)) return false;
    setBusy(true);
    setError("");
    try {
      const active = normalize(await fetchUnboundSettings());
      if (!sameSettings(active, source)) throw new Error(concurrentMessage);
      await updateUnboundSettings(candidate);
      await onActivated();
      await load();
      return true;
    } catch (cause) {
      setError(errorMessage(cause, activateErrorMessage));
      return false;
    } finally {
      setBusy(false);
    }
  }

  return { source, preview, candidate, busy, message, setMessage, error, setError, load, resetPreview, createPreview, activate };
}
