// =====================================================
// File: frontend/src/components/Sidebar.tsx
// Purpose: Premium Collapsible Sidebar
// =====================================================

import { useState } from "react";
import {
  LayoutDashboard,
  Dock,        // ← FIXED (war Docker)
  Shield,
  Activity,
} from "lucide-react";

interface Item {
  label: string;
  icon: React.ReactNode;
  path: string;
}

const items: Item[] = [
  {
    label: "Overview",
    icon: <LayoutDashboard size={18} />,
    path: "/",
  },
  {
    label: "Docker Stack",
    icon: <Dock size={18} />,   // ← FIXED
    path: "/docker",
  },
  {
    label: "Unbound Assistant",
    icon: <Shield size={18} />,
    path: "/assistant",
  },
  {
    label: "Health",
    icon: <Activity size={18} />,
    path: "/health",
  },
];

export default function Sidebar() {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <div
      style={{
        width: collapsed ? "80px" : "240px",
        transition: "all 0.3s ease",
        background: "rgba(20,30,50,0.9)",
        backdropFilter: "blur(14px)",
        borderRight: "1px solid rgba(255,255,255,0.05)",
        display: "flex",
        flexDirection: "column",
      }}
    >
      {/* Toggle */}
      <div
        style={{
          padding: "20px",
          cursor: "pointer",
          fontSize: "14px",
          opacity: 0.6,
        }}
        onClick={() => setCollapsed(!collapsed)}
      >
        {collapsed ? "→" : "←"}
      </div>

      {/* Navigation */}
      {items.map((item, i) => (
        <div
          key={i}
          style={{
            display: "flex",
            alignItems: "center",
            gap: "16px",
            padding: "14px 20px",
            cursor: "pointer",
            fontSize: "14px",
            opacity: 0.85,
          }}
        >
          {item.icon}
          {!collapsed && item.label}
        </div>
      ))}
    </div>
  );
}