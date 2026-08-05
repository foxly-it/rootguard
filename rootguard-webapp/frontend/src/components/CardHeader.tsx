// =====================================================
// File: frontend/src/components/CardHeader.tsx
// Purpose: Header with Correct Status Mapping
// =====================================================

import type { ServiceStatus } from "../types/status";

interface Props {
  title: string;
  status: ServiceStatus;
  lastChecked: string;
}

export default function CardHeader({
  title,
  status,
  lastChecked,
}: Props) {
  // Status → Farbe sauber gemappt
  const color =
    status === "healthy"
      ? "#22c55e"
      : status === "degraded"
      ? "#f59e0b"
      : "#ef4444";

  const label =
    status === "healthy"
      ? "Healthy"
      : status === "degraded"
      ? "Degraded"
      : "Down";

  return (
    <div
      style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        marginBottom: "40px",
      }}
    >
      <h2
        style={{
          fontSize: "22px",
          fontWeight: 600,
        }}
      >
        {title}
      </h2>

      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "10px",
          fontSize: "14px",
        }}
      >
        <span
          className="status-dot"
          style={{ backgroundColor: color }}
        />

        <span>{label}</span>

        <span style={{ opacity: 0.5 }}>
          {lastChecked}
        </span>
      </div>
    </div>
  );
}