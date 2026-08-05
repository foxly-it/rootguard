// =====================================================
// File: frontend/src/components/ServiceControls.tsx
// Purpose: Async Service Action Buttons with Spinner
// =====================================================

import { useState } from "react";

interface Props {
  onStart: () => Promise<void>;
  onStop: () => Promise<void>;
  onRestart: () => Promise<void>;
}

type Action = "start" | "stop" | "restart" | null;

export default function ServiceControls({
  onStart,
  onStop,
  onRestart,
}: Props) {
  const [loading, setLoading] = useState<Action>(null);

  async function handle(action: Action, fn: () => Promise<void>) {
    if (loading) return;

    try {
      setLoading(action);
      await fn();
    } finally {
      setLoading(null);
    }
  }

  return (
    <div style={{ display: "flex", gap: "16px" }}>
      <Button
        label="Start"
        active={loading === "start"}
        disabled={!!loading}
        onClick={() => handle("start", onStart)}
      />
      <Button
        label="Stop"
        active={loading === "stop"}
        disabled={!!loading}
        onClick={() => handle("stop", onStop)}
      />
      <Button
        label="Restart"
        active={loading === "restart"}
        disabled={!!loading}
        onClick={() => handle("restart", onRestart)}
      />
    </div>
  );
}

function Button({
  label,
  active,
  disabled,
  onClick,
}: {
  label: string;
  active: boolean;
  disabled: boolean;
  onClick: () => void;
}) {
  return (
    <button
      disabled={disabled}
      onClick={onClick}
      style={{
        padding: "10px 20px",
        borderRadius: "14px",
        border: "1px solid rgba(255,255,255,0.1)",
        background: "rgba(255,255,255,0.05)",
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.6 : 1,
        display: "flex",
        alignItems: "center",
        gap: "8px",
      }}
    >
      {active && <Spinner />}
      {label}
    </button>
  );
}

function Spinner() {
  return (
    <div
      style={{
        width: "14px",
        height: "14px",
        border: "2px solid rgba(255,255,255,0.3)",
        borderTop: "2px solid white",
        borderRadius: "50%",
        animation: "spin 0.8s linear infinite",
      }}
    />
  );
}