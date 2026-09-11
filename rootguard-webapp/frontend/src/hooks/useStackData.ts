import { useCallback, useEffect, useMemo, useState } from "react";
import {
  checkControlPlaneUpdates,
  checkUpdates,
  checkUpdaterSelfUpdate,
  fetchControlPlaneUpdateStatus,
  fetchCleanupPreview,
  fetchServices,
  fetchUpdateStatus,
  fetchUpdaterSelfUpdateStatus,
  installServiceUpdate,
  installControlPlaneUpdates,
  installUpdaterSelfUpdate,
  runManualCleanup,
  serviceAction,
  type ServiceInfo,
  type UpdateServiceStatus,
  type UpdateStatus,
  type ControlPlaneUpdateStatus,
  type UpdaterSelfUpdateStatus,
  type CleanupPreview,
} from "../api/client";
import { useI18n } from "../i18n";
import { useInterval } from "./useInterval";
import { errorMessage } from "../utils/errors";
import { formatBytes } from "../utils/format";

/**
 * All of Stack.tsx's data-fetching, polling, and mutation logic -
 * extracted out of the page component (found in review: data logic and
 * JSX were mixed into one large component) so the page itself is just a
 * presentation layer over this hook's return value.
 */
export function useStackData() {
  const { t } = useI18n();
  const [updates, setUpdates] = useState<UpdateStatus | null>(null);
  const [controlPlane, setControlPlane] = useState<ControlPlaneUpdateStatus | null>(null);
  const [updaterUpdate, setUpdaterUpdate] = useState<UpdaterSelfUpdateStatus | null>(null);
  const [cleanup, setCleanup] = useState<CleanupPreview | null>(null);
  const [runningCleanup, setRunningCleanup] = useState(false);
  const [services, setServices] = useState<ServiceInfo[]>([]);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const [nextUpdates, nextControlPlane, nextUpdaterUpdate, nextServices] = await Promise.all([
        fetchUpdateStatus(),
        fetchControlPlaneUpdateStatus(),
        fetchUpdaterSelfUpdateStatus(),
        fetchServices(),
      ]);
      setUpdates(nextUpdates);
      setControlPlane(nextControlPlane);
      setUpdaterUpdate(nextUpdaterUpdate);
      setServices(nextServices);
      setError("");
    } catch (cause) {
      setError(errorMessage(cause, t("stack.statusLoadError")));
    }
  }, [t]);

  useEffect(() => {
    const initial = window.setTimeout(load, 0);
    return () => window.clearTimeout(initial);
  }, [load]);

  const busy = updates?.state === "checking" || updates?.state === "updating"
    || controlPlane?.state === "checking" || controlPlane?.state === "updating"
    || updaterUpdate?.state === "checking" || updaterUpdate?.state === "updating" || runningCleanup;
  useInterval(load, busy ? 1500 : 10_000);

  const available = useMemo(
    () => (updates?.services.filter((service) => service.update_available).length ?? 0)
      + (controlPlane?.services.filter((service) => service.update_available).length ?? 0)
      + (updaterUpdate?.services.filter((service) => service.update_available).length ?? 0),
    [updates, controlPlane, updaterUpdate],
  );
  const history = useMemo(
    () => [
      ...(updates?.history ?? []).map((entry) => ({ ...entry, scope: entry.service === "cleanup" ? "Docker" : entry.service || "DNS" })),
      ...(controlPlane?.history ?? []).map((entry) => ({ ...entry, scope: "Control Panel" })),
      // updaterUpdate carries both "updater" and "attestation-proxy"
      // entries now (one shared Core-side manager, see the module-level
      // comment on this component's imports) - the scope label is
      // per-entry, not a single fixed string, unlike the two spreads
      // above whose whole status object only ever covers one scope.
      ...(updaterUpdate?.history ?? []).map((entry) => ({ ...entry, scope: entry.service === "attestation-proxy" ? "Attestation Proxy" : "Updater" })),
    ].sort((left, right) => Date.parse(right.created_at) - Date.parse(left.created_at)).slice(0, 12),
    [updates, controlPlane, updaterUpdate],
  );
  const updaterRuntime = services.find((service) => service.name === "updater");
  const updaterService = updaterUpdate?.services.find((service) => service.name === "updater");
  // No matching entry in `services` (Core's own dashboard inspection stays
  // scoped to its existing 5-service allowlist, see servicesHandler in
  // rootguard-core - out of scope for rootguard#481, which is purely about
  // the update *mechanism*) - ControlPlaneService's `runtime` prop is
  // already optional and falls back to `fallbackImage`/"not inspected" for
  // exactly this case, so the card still works, just without live
  // running/immutability/attestation-badge detail the allowlisted services
  // show.
  const attestationProxyService = updaterUpdate?.services.find((service) => service.name === "attestation-proxy");

  // attempt clears any stale error, runs action, and reports a new one on
  // failure - the "setError(''); try {...} catch { setError(...) }"
  // skeleton every mutation below shared (found in review). guard, when
  // given, is checked first (e.g. a window.confirm) so a declined/blocked
  // action never clears an error message still relevant to the user, and a
  // confirm dialog is asked at most once, in the same order as before.
  const attempt = useCallback(async (action: () => Promise<void>, fallback: string, guard?: () => boolean) => {
    if (guard && !guard()) return;
    setError("");
    try {
      await action();
    } catch (cause) {
      setError(errorMessage(cause, t(fallback)));
    }
  }, [t]);

  const startCheck = useCallback(() => attempt(async () => {
    const [nextUpdates, nextControlPlane, nextUpdaterUpdate] = await Promise.all([
      checkUpdates(),
      checkControlPlaneUpdates(),
      checkUpdaterSelfUpdate(),
    ]);
    setUpdates(nextUpdates);
    setControlPlane(nextControlPlane);
    setUpdaterUpdate(nextUpdaterUpdate);
  }, "stack.updateCheckError"), [attempt]);

  const startControlPlaneUpdate = useCallback(() => attempt(async () => {
    setControlPlane(await installControlPlaneUpdates());
  }, "stack.controlPlaneStartError", () => window.confirm(t("stack.controlPlaneConfirm"))), [t, attempt]);

  const startSelfUpdate = useCallback((service: "updater" | "attestation-proxy") => {
    const confirmKey = service === "updater" ? "stack.updaterSelfUpdateConfirm" : "stack.attestationProxySelfUpdateConfirm";
    const errorKey = service === "updater" ? "stack.updaterSelfUpdateStartError" : "stack.attestationProxySelfUpdateStartError";
    return attempt(async () => {
      setUpdaterUpdate(await installUpdaterSelfUpdate(service));
    }, errorKey, () => window.confirm(t(confirmKey)));
  }, [t, attempt]);

  const startUpdate = useCallback((service: UpdateServiceStatus) => attempt(async () => {
    setUpdates(await installServiceUpdate(service.name));
  }, "stack.updateStartError", () => window.confirm(t("stack.confirmUpdate", { service: service.display_name }))), [t, attempt]);

  const refreshCleanupPreview = useCallback(() => attempt(async () => {
    setCleanup(await fetchCleanupPreview());
  }, "stack.cleanupPreviewError"), [attempt]);

  const startManualCleanup = useCallback(() => {
    if (!cleanup?.resources.length || !window.confirm(t("stack.cleanupConfirm", { count: cleanup.resources.length, size: formatBytes(cleanup.estimated_bytes) }))) {
      return Promise.resolve();
    }
    setRunningCleanup(true);
    return attempt(async () => {
      await runManualCleanup();
      await Promise.all([load(), refreshCleanupPreview()]);
    }, "stack.cleanupRunError").finally(() => setRunningCleanup(false));
  }, [cleanup, t, load, refreshCleanupPreview, attempt]);

  const control = useCallback(async (name: ServiceInfo["name"], action: "start" | "stop" | "restart") => {
    if (action === "stop" && !window.confirm(t("stack.confirmStop", { service: name }))) return;
    try {
      await serviceAction(name, action);
      await load();
    } catch (cause) {
      setError(errorMessage(cause, t("stack.serviceActionError")));
    }
  }, [t, load]);

  return {
    updates,
    controlPlane,
    updaterUpdate,
    cleanup,
    runningCleanup,
    services,
    error,
    busy,
    available,
    history,
    updaterRuntime,
    updaterService,
    attestationProxyService,
    startCheck,
    startControlPlaneUpdate,
    startSelfUpdate,
    startUpdate,
    refreshCleanupPreview,
    startManualCleanup,
    control,
  };
}
