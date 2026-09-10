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

  const startCheck = useCallback(async () => {
    setError("");
    try {
      const [nextUpdates, nextControlPlane, nextUpdaterUpdate] = await Promise.all([
        checkUpdates(),
        checkControlPlaneUpdates(),
        checkUpdaterSelfUpdate(),
      ]);
      setUpdates(nextUpdates);
      setControlPlane(nextControlPlane);
      setUpdaterUpdate(nextUpdaterUpdate);
    } catch (cause) {
      setError(errorMessage(cause, t("stack.updateCheckError")));
    }
  }, [t]);

  const startControlPlaneUpdate = useCallback(async () => {
    if (!window.confirm(t("stack.controlPlaneConfirm"))) return;
    setError("");
    try {
      setControlPlane(await installControlPlaneUpdates());
    } catch (cause) {
      setError(errorMessage(cause, t("stack.controlPlaneStartError")));
    }
  }, [t]);

  const startSelfUpdate = useCallback(async (service: "updater" | "attestation-proxy") => {
    const confirmKey = service === "updater" ? "stack.updaterSelfUpdateConfirm" : "stack.attestationProxySelfUpdateConfirm";
    if (!window.confirm(t(confirmKey))) return;
    setError("");
    try {
      setUpdaterUpdate(await installUpdaterSelfUpdate(service));
    } catch (cause) {
      const errorKey = service === "updater" ? "stack.updaterSelfUpdateStartError" : "stack.attestationProxySelfUpdateStartError";
      setError(errorMessage(cause, t(errorKey)));
    }
  }, [t]);

  const startUpdate = useCallback(async (service: UpdateServiceStatus) => {
    const accepted = window.confirm(
      t("stack.confirmUpdate", { service: service.display_name }),
    );
    if (!accepted) return;
    setError("");
    try {
      setUpdates(await installServiceUpdate(service.name));
    } catch (cause) {
      setError(errorMessage(cause, t("stack.updateStartError")));
    }
  }, [t]);

  const refreshCleanupPreview = useCallback(async () => {
    setError("");
    try {
      setCleanup(await fetchCleanupPreview());
    } catch (cause) {
      setError(errorMessage(cause, t("stack.cleanupPreviewError")));
    }
  }, [t]);

  const startManualCleanup = useCallback(async () => {
    if (!cleanup?.resources.length || !window.confirm(t("stack.cleanupConfirm", { count: cleanup.resources.length, size: formatBytes(cleanup.estimated_bytes) }))) return;
    setRunningCleanup(true);
    setError("");
    try {
      await runManualCleanup();
      await Promise.all([load(), refreshCleanupPreview()]);
    } catch (cause) {
      setError(errorMessage(cause, t("stack.cleanupRunError")));
    } finally {
      setRunningCleanup(false);
    }
  }, [cleanup, t, load, refreshCleanupPreview]);

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
