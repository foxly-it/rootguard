import { useCallback, useEffect, useState } from "react";
import {
  bootstrapAdGuard,
  fetchAdGuardFilterReport,
  fetchAdGuardStatus,
  fetchInstallationStatus,
  setAdGuardFiltering,
  setAdGuardProtection,
  type AdGuardFilterReport,
  type AdGuardStatus,
  type InstallationStatus,
} from "../api/client";
import { useI18n } from "../i18n";
import { useInterval } from "./useInterval";
import { errorMessage } from "../utils/errors";
import { formatCountdown } from "../utils/countdown";

/**
 * All of AdGuard.tsx's data-fetching, polling, and mutation logic -
 * extracted out of the page component (found in review: the same gap
 * already closed for Overview.tsx/Stack.tsx/Unbound.tsx) so the page itself
 * is just a presentation layer over this hook's return value. Which filter-
 * test modal is open stays in the page component - it's purely
 * presentational routing state, not data this hook owns.
 */
export function useAdGuardData() {
  const { t } = useI18n();
  const [status, setStatus] = useState<AdGuardStatus | null>(null);
  const [installation, setInstallation] = useState<InstallationStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [bootstrapping, setBootstrapping] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [filterReport, setFilterReport] = useState<AdGuardFilterReport | null>(null);
  const [testingFilters, setTestingFilters] = useState(false);
  const [filterError, setFilterError] = useState("");
  const [filteringBusy, setFilteringBusy] = useState(false);
  const [protectionBusy, setProtectionBusy] = useState(false);
  const [protectionChoice, setProtectionChoice] = useState("");
  // When `status` was last actually fetched/updated - paired with
  // status.protection_disabled_duration_ms (AdGuard's own remaining-pause
  // figure at that moment) to compute a live countdown without re-fetching
  // every second. `now` is ticked from a 1s interval rather than read via
  // Date.now() directly in render, matching the Sparkline hover-age pattern
  // elsewhere in this codebase (react-hooks/purity rejects impure reads
  // during render).
  const [statusFetchedAt, setStatusFetchedAt] = useState<number | null>(null);
  const [now, setNow] = useState(0);

  const applyStatus = useCallback((next: AdGuardStatus | null) => {
    setStatus(next);
    setStatusFetchedAt(Date.now());
  }, []);

  const load = useCallback(async () => {
    setError("");
    try {
      const currentInstallation = await fetchInstallationStatus();
      setInstallation(currentInstallation);
      if (currentInstallation.state === "installed") {
        applyStatus(await fetchAdGuardStatus());
      } else {
        applyStatus(null);
      }
    } catch (cause) {
      setError(errorMessage(cause, t("adguard.statusLoadError")));
    } finally {
      setLoading(false);
    }
  }, [t, applyStatus]);

  useEffect(() => {
    const initialLoad = window.setTimeout(load, 0);
    return () => window.clearTimeout(initialLoad);
  }, [load]);

  // AdGuard re-enables protection itself once a timed pause elapses (see
  // changeProtection) - without polling here, RootGuard would keep showing
  // "paused" until the page was manually reloaded. Only runs while actually
  // paused, so it doesn't add load the rest of the time.
  useInterval(() => {
    fetchAdGuardStatus().then(applyStatus).catch(() => {});
  }, status && !status.protection_enabled ? 5000 : null);

  // Drives the visible countdown between polls above - only ticks while a
  // *timed* pause (not an indefinite one) is showing. Not useInterval: this
  // one deliberately resyncs `now` to Date.now() and restarts its 1s phase
  // on every fresh `status` (i.e. after each poll above), not just when
  // entering/leaving a pause - useInterval's fixed-cadence contract would
  // drop that resync.
  useEffect(() => {
    if (!status || status.protection_enabled || status.protection_disabled_duration_ms <= 0) return;
    setNow(Date.now());
    const interval = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(interval);
  }, [status]);

  const remainingMs = status && !status.protection_enabled && status.protection_disabled_duration_ms > 0 && statusFetchedAt !== null && now > 0
    ? Math.max(0, status.protection_disabled_duration_ms - (now - statusFetchedAt))
    : null;

  async function initialize() {
    if (bootstrapping) return;
    setBootstrapping(true);
    setMessage("");
    setError("");
    try {
      const updated = await bootstrapAdGuard();
      applyStatus(updated);
      setMessage(t("adguard.bootstrapComplete"));
    } catch (cause) {
      setError(errorMessage(cause, t("adguard.bootstrapError")));
    } finally {
      setBootstrapping(false);
    }
  }

  async function testFilters() {
    if (testingFilters) return;
    setTestingFilters(true);
    setFilterError("");
    try {
      setFilterReport(await fetchAdGuardFilterReport());
    } catch (cause) {
      setFilterError(errorMessage(cause, t("adguard.filterTestError")));
    } finally {
      setTestingFilters(false);
    }
  }

  async function toggleFiltering() {
    if (filteringBusy || !status) return;
    setFilteringBusy(true);
    setError("");
    try {
      applyStatus(await setAdGuardFiltering(!status.filtering_enabled));
    } catch (cause) {
      setError(errorMessage(cause, t("adguard.filteringToggleError")));
    } finally {
      setFilteringBusy(false);
    }
  }

  // Mirrors AdGuard Home's own "Protection" dropdown (Off/10 minutes/1
  // hour) - unlike filtering above, AdGuard itself re-enables protection
  // after the chosen duration, no RootGuard-side scheduling needed. The
  // select is an action trigger, not a state display (see protectionChoice
  // reset below) - found via code review: binding its value straight to
  // protection_enabled made a 10-minute pause look identical to "off
  // indefinitely" the instant it was chosen, since both just set
  // protection_enabled to false. The actual state is shown separately
  // (protectionStatusLabel below).
  async function changeProtection(choice: "on" | "off" | "10m" | "1h") {
    setProtectionChoice("");
    if (protectionBusy || !status) return;
    setProtectionBusy(true);
    setError("");
    try {
      const durations: Record<typeof choice, number> = { on: 0, off: 0, "10m": 600, "1h": 3600 };
      applyStatus(await setAdGuardProtection(choice === "on", durations[choice]));
    } catch (cause) {
      setError(errorMessage(cause, t("adguard.protectionToggleError")));
    } finally {
      setProtectionBusy(false);
    }
  }

  // reachable: AdGuard is configured and answering - independent of whether
  // protection/filtering happen to be paused right now. Gates things that
  // stay usable during a pause (opening the native UI, running a filter
  // test, the pause control itself - it would be self-defeating if pausing
  // protection also hid the control needed to un-pause it).
  const reachable = Boolean(status?.configured && status.healthy && status.upstream_ready);
  // ready: reachable AND actually filtering traffic right now. Found via
  // code review: "PROTECTED"/the STATUS badge previously ignored
  // protection_enabled entirely, so pausing protection for a client still
  // showed fully green everywhere. Matches Overview.tsx's protectedState.
  const ready = reachable && status?.protection_enabled === true && status?.filtering_enabled === true;

  const protectionStatusLabel = !status
    ? ""
    : status.protection_enabled
      ? t("adguard.protectionStatusActive")
      : remainingMs !== null
        ? t("adguard.protectionStatusPausedFor", { time: formatCountdown(remainingMs) })
        : t("adguard.protectionStatusPausedIndefinite");

  return {
    status,
    installation,
    loading,
    bootstrapping,
    message,
    error,
    filterReport,
    testingFilters,
    filterError,
    filteringBusy,
    protectionBusy,
    protectionChoice,
    reachable,
    ready,
    protectionStatusLabel,
    initialize,
    testFilters,
    toggleFiltering,
    changeProtection,
  };
}
