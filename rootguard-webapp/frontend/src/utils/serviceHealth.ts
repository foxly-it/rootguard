import type { ServiceInfo } from "../api/client";

export type Translate = (key: string, values?: Record<string, string | number>) => string;

export function runtimeTone(service?: ServiceInfo) {
  if (!service || service.status !== "running" || service.health === "unhealthy") return "danger";
  if (service.health === "starting" || service.health === "unknown") return "attention";
  return "good";
}

export function healthLabel(service: ServiceInfo | undefined, t: Translate) {
  if (!service || service.status !== "running") return t("stack.stopped");
  return t(`stack.health.${service.health}`);
}
