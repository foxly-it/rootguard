import { useCallback, useEffect, useRef, useState } from "react";
import {
  fetchAdGuardStatus,
  fetchDashboard,
  fetchInstallationStatus,
  fetchServices,
  serviceAction,
  type AdGuardStatus,
  type DashboardResponse,
  type InstallationStatus,
  type ServiceInfo,
} from "../api/client";
import { useI18n } from "../i18n";
import { useInterval } from "./useInterval";
import { blockRatePercent, pushHistory, type HistoryPoint } from "../utils/metrics";

// How many samples the resource sparklines keep in memory - purely
// client-side, resets on page load (RootGuard has no metrics time-series
// store). CPU/RAM poll every 500ms (see loadCoreMetrics) but only actually
// change once Core's own background collector refreshes its cache, roughly
// once a second (dashboardRefreshInterval in
// rootguard-core/internal/stack/metrics.go) - and pushHistory skips
// re-recording a sample whose collected_at hasn't moved, so 120 samples
// there covers a couple of minutes of real history, not literally 60s of
// polling. AdGuard-derived series (queries/blocked/filter rate) poll much
// slower, every 5s (see loadAdGuardMetrics), so the same length covers
// around 10 minutes for those.
const HISTORY_LENGTH = 120;

/**
 * All of Overview.tsx's data-fetching, polling, and mutation logic -
 * extracted out of the page component (found in review: data logic and
 * JSX were mixed into one large component) so the page itself is just a
 * presentation layer over this hook's return value.
 */
export function useOverviewData() {
  const { t } = useI18n();
  const [dashboard, setDashboard] = useState<DashboardResponse | null>(null);
  const [installation, setInstallation] = useState<InstallationStatus | null>(null);
  const [services, setServices] = useState<ServiceInfo[]>([]);
  const [adGuard, setAdGuard] = useState<AdGuardStatus | null>(null);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [busyService, setBusyService] = useState("");
  const [error, setError] = useState("");
  const [cpuHistory, setCpuHistory] = useState<HistoryPoint[]>([]);
  const [memoryHistory, setMemoryHistory] = useState<HistoryPoint[]>([]);
  const [queriesHistory, setQueriesHistory] = useState<HistoryPoint[]>([]);
  const [blockedHistory, setBlockedHistory] = useState<HistoryPoint[]>([]);
  const [blockRateHistory, setBlockRateHistory] = useState<HistoryPoint[]>([]);

  // Tracks the latest known installation state outside React state so
  // loadMetrics (see below) can decide whether to also fetch AdGuard stats
  // without re-fetching installation status itself every time - that's
  // loadStatus's job, on its own slower cadence.
  const installationRef = useRef<InstallationStatus | null>(null);

  // Every poll cycle can have several requests in flight at once (the
  // startup burst, the steady interval, a manual refresh, a post-restart
  // reload) with no guarantee they resolve in the order they were sent. A
  // sequence counter per loader ensures only the most recently *started*
  // request's response is ever applied - a slow, stale response finishing
  // after a newer one can no longer clobber fresher data, reset
  // lastChecked to the wrong time, or resurrect an error that already
  // cleared.
  const metricsSeq = useRef(0);
  const adGuardSeq = useRef(0);
  const statusSeq = useRef(0);
  // Core's /api/dashboard now serves from a background-refreshed cache
  // (see rootguard-core/internal/stack/metrics.go) instead of shelling out
  // to `docker stats` per request, so it's normally fast. This guard is
  // kept anyway as cheap insurance: it's exactly what would have prevented
  // the metrics never populating at all when that wasn't true yet
  // (reproduced live at a 1s interval, before the Core fix: every tick's
  // request got superseded by the next one - the sequence guard above only
  // decides which *result* wins, it doesn't stop requests piling up) - and
  // it still protects against any transient slow response (e.g. Core under
  // load, or the brief window before its first background collection
  // completes) at this now much faster 500ms cadence.
  const metricsInFlight = useRef(false);
  const adGuardInFlight = useRef(false);

  const loadCoreMetrics = useCallback(async () => {
    if (metricsInFlight.current) return;
    metricsInFlight.current = true;
    const seq = ++metricsSeq.current;
    try {
      const nextDashboard = await fetchDashboard();
      if (seq !== metricsSeq.current) return;

      setDashboard(nextDashboard);
      // Pass the cache's own collected_at instead of the receipt time: Core
      // only refreshes this roughly once a second (dashboardRefreshInterval
      // in rootguard-core/internal/stack/metrics.go) while this poll runs
      // twice that fast, so most polls just re-read the same cached sample.
      // pushHistory skips re-appending when the timestamp is unchanged -
      // without collected_at here, every poll would look like a distinct,
      // freshly-sampled point even when nothing new was actually measured,
      // which the sparkline tooltip's "N seconds ago" would then understate.
      const collectedAt = nextDashboard.docker.collected_at;
      setCpuHistory((prev) => pushHistory(prev, nextDashboard.docker.metrics_available ? nextDashboard.docker.cpu : null, HISTORY_LENGTH, collectedAt));
      setMemoryHistory((prev) => pushHistory(prev, nextDashboard.docker.metrics_available ? nextDashboard.docker.memory : null, HISTORY_LENGTH, collectedAt));
      setLastChecked(new Date());
      setError("");
    } catch (cause) {
      if (seq !== metricsSeq.current) return;
      setError(cause instanceof Error ? cause.message : t("overview.loadError"));
    } finally {
      metricsInFlight.current = false;
    }
  }, [t]);

  // AdGuard's own status endpoint makes up to four real requests to
  // AdGuard's API per call (see rootguard-core/internal/adguard/manager.go),
  // unlike the dashboard endpoint above which now reads from a cache. Found
  // via code review: polling it at the same 500ms cadence as CPU/RAM meant
  // up to eight AdGuard API calls per second per open dashboard, multiplied
  // by however many tabs/users had one open. Query/block counts also don't
  // change fast enough to need sub-second freshness, so this runs on its
  // own, much slower interval instead.
  const loadAdGuardMetrics = useCallback(async () => {
    if (installationRef.current?.state !== "installed") return;
    if (adGuardInFlight.current) return;
    adGuardInFlight.current = true;
    const seq = ++adGuardSeq.current;
    try {
      const nextAdGuard = await fetchAdGuardStatus().catch(() => null);
      if (seq !== adGuardSeq.current) return;

      setAdGuard(nextAdGuard);
      setQueriesHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? nextAdGuard.queries : null, HISTORY_LENGTH));
      setBlockedHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? nextAdGuard.blocked : null, HISTORY_LENGTH));
      setBlockRateHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? blockRatePercent(nextAdGuard.blocked, nextAdGuard.queries) : null, HISTORY_LENGTH));
    } finally {
      adGuardInFlight.current = false;
    }
  }, []);

  const loadMetrics = useCallback(async () => {
    await Promise.all([loadCoreMetrics(), loadAdGuardMetrics()]);
  }, [loadCoreMetrics, loadAdGuardMetrics]);

  const loadStatus = useCallback(async () => {
    const seq = ++statusSeq.current;
    try {
      const [nextInstallation, nextServices] = await Promise.all([
        fetchInstallationStatus(),
        fetchServices(),
      ]);
      if (seq !== statusSeq.current) return;
      installationRef.current = nextInstallation;
      setInstallation(nextInstallation);
      setServices(nextServices);
      setError("");
    } catch (cause) {
      if (seq !== statusSeq.current) return;
      setError(cause instanceof Error ? cause.message : t("overview.loadError"));
    }
  }, [t]);

  const refreshAll = useCallback(async () => {
    await Promise.all([loadStatus(), loadMetrics()]);
  }, [loadStatus, loadMetrics]);

  // Backgrounded/hidden tabs have no reason to keep polling Core, Docker,
  // and AdGuard every second - nobody's watching the charts. Pausing there
  // and firing an immediate load on return keeps the same "feels current"
  // behavior without the wasted requests in between.
  const [visible, setVisible] = useState(() => !document.hidden);
  useEffect(() => {
    function handleVisibilityChange() {
      setVisible(!document.hidden);
    }
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => document.removeEventListener("visibilitychange", handleVisibilityChange);
  }, []);

  // At a 500ms cadence the interval itself already delivers a fresh sample
  // about as fast as a burst ever could, so the earlier startup-burst
  // scheme (extra timeouts at 2/4/7s to front-load a few samples before
  // the old, much slower 10s interval caught up) is redundant now and was
  // removed - one immediate call plus the interval is already fast.
  useEffect(() => {
    if (!visible) return;
    loadStatus();
    loadCoreMetrics();
    loadAdGuardMetrics();
  }, [visible, loadStatus, loadCoreMetrics, loadAdGuardMetrics]);

  useInterval(loadCoreMetrics, visible ? 500 : null);
  // AdGuard's status endpoint makes several real upstream requests per call
  // (see loadAdGuardMetrics) - a much slower cadence than CPU/RAM keeps
  // that load reasonable without the query/block counts feeling stale,
  // since they don't change on a sub-second timescale anyway.
  useInterval(loadAdGuardMetrics, visible ? 5_000 : null);
  // Service/installation status changes far less often than CPU/memory/
  // query counts - a slower cadence is plenty fresh for it and avoids
  // hitting Core for a full service/Docker inspect every single second.
  useInterval(loadStatus, visible ? 20_000 : null);

  const restart = useCallback(async (service: ServiceInfo["name"]) => {
    setBusyService(service);
    try {
      await serviceAction(service, "restart");
      await refreshAll();
    } finally {
      setBusyService("");
    }
  }, [refreshAll]);

  return {
    dashboard,
    installation,
    services,
    adGuard,
    lastChecked,
    busyService,
    error,
    cpuHistory,
    memoryHistory,
    queriesHistory,
    blockedHistory,
    blockRateHistory,
    restart,
    refreshAll,
  };
}
