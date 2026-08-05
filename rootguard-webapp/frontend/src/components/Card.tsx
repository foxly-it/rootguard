// =====================================================
// File: frontend/src/components/Card.tsx
// Purpose: Reusable Card Container
// Supports custom className (e.g. card-full)
// =====================================================

import type { ReactNode } from "react";

interface Props {
  children: ReactNode;
  className?: string;
}

export default function Card({ children, className = "" }: Props) {
  return (
    <div className={`rg-card ${className}`}>
      {children}
    </div>
  );
}