export const hostLabelPattern = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;

export function validIPv4(value: string): boolean {
  const parts = value.split(".");
  return parts.length === 4 && parts.every((part) => /^\d{1,3}$/.test(part) && Number(part) <= 255);
}

export function normalizeZoneName(value: string, rootError: string, formatError: string): string {
  const normalized = value.trim().toLowerCase().replace(/\.*$/, "") + ".";
  if (normalized === ".") throw new Error(rootError);
  const labels = normalized.slice(0, -1).split(".");
  if (normalized.length > 254 || !labels.every((label) => hostLabelPattern.test(label))) {
    throw new Error(formatError);
  }
  return normalized;
}

export function normalizeHostLabel(value: string, formatError: string): string {
  const normalized = value.trim().toLowerCase();
  if (!hostLabelPattern.test(normalized)) throw new Error(formatError);
  return normalized;
}

export function localZoneSection(config: string): string {
  const lines = config.split("\n").filter((line) =>
    line.includes("# Local host inventory:") ||
    line.trimStart().startsWith("local-zone:") ||
    line.trimStart().startsWith("local-data:") ||
    line.trimStart().startsWith("local-data-ptr:"),
  );
  return lines.length > 0 ? `server:\n${lines.join("\n")}` : "# No local-zone directives.";
}
