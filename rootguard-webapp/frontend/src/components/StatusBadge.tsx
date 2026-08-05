// =====================================================
// File: frontend/src/components/StatusBadge.tsx
// Purpose: Visual dashboard status badge
// =====================================================

interface Props {
  status: "ok" | "warning" | "error";
}

export default function StatusBadge({ status }: Props) {

  const label =
    status === "ok"
      ? "OK"
      : status === "warning"
      ? "Warning"
      : "Error";

  return (
    <span className={`status-badge ${status}`}>
      {label}
    </span>
  );
}