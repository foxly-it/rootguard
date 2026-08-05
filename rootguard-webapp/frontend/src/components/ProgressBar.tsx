// =====================================================
// File: frontend/src/components/ProgressBar.tsx
// =====================================================

interface Props {
  value: number; // percentage 0-100
  color?: string;
}

export default function ProgressBar({ value, color = "#22c55e" }: Props) {

  return (
    <div style={{
      background: "rgba(255,255,255,0.08)",
      borderRadius: "999px",
      height: "8px",
      overflow: "hidden",
      marginTop: "8px"
    }}>
      <div style={{
        width: `${value}%`,
        height: "100%",
        borderRadius: "999px",
        background: `linear-gradient(90deg, ${color}, ${color}aa)`,
        boxShadow: `0 0 10px ${color}88`,
        transition: "width 0.4s ease"
      }} />
    </div>
  );
}
