// =====================================================
// File: frontend/src/utils/statusMapper.ts
// Purpose: Maps backend health states to UI states
// =====================================================

import type { BackendHealthStatus, ServiceStatus }
  from "../types/status";

/**
 * Maps backend health to UI-specific service status.
 * This isolates backend naming from frontend UI logic.
 */
export function mapBackendStatus(
  status: BackendHealthStatus
): ServiceStatus {

  switch (status) {
    case "ok":
      return "healthy";

    case "warn":
      return "degraded";

    case "error":
      return "down";

    default:
      return "down";
  }
}