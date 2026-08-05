// =====================================================
// File: frontend/src/components/StatusIndicator.tsx
// Purpose: Compact status dot indicator
// =====================================================

import type { ServiceStatus } from "../types/status";
import "./status-indicator.css";

interface Props {
  status: ServiceStatus;
}

export default function StatusIndicator({ status }: Props) {
  const label =
    status === "healthy"
      ? "Healthy"
      : status === "degraded"
      ? "Degraded"
      : "Down";

  return (
    <div className="status-wrapper">
      <div className={`status-dot status-${status}`} />
      <span className="status-label">{label}</span>
    </div>
  );
}