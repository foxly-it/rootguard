// =====================================================
// File: frontend/src/components/icons/DnsIcon.tsx
// Purpose: DNS Engine Icon (inline SVG, theme aware)
// =====================================================

export default function DnsIcon() {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      className="card-title-icon"
    >
      <circle cx="12" cy="12" r="10" />
      <path d="M2 12h20" />
      <path d="M12 2a15 15 0 0 1 0 20" />
    </svg>
  );
}
