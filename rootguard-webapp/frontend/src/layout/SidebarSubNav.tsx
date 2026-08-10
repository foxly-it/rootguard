import { createContext, useContext, useEffect, type ReactNode } from "react";

export interface SidebarSubNavItem {
  id: string;
  label: string;
  icon?: ReactNode;
  active: boolean;
  onSelect: () => void;
}

interface SidebarSubNavContextValue {
  setSubNav: (path: string, items: SidebarSubNavItem[]) => void;
  clearSubNav: (path: string) => void;
}

export const SidebarSubNavContext = createContext<SidebarSubNavContextValue | null>(null);

// Lets a page register nested links under its own entry in the main
// sidebar (e.g. Unbound's per-tab subsections) instead of rendering its
// own floating or sticky in-page nav widget - those never worked well
// below very wide viewports. Registers on mount/whenever items change,
// clears on unmount so a page never leaves stale sub-items behind for the
// next one.
export function useSidebarSubNav(path: string, items: SidebarSubNavItem[]) {
  const ctx = useContext(SidebarSubNavContext);
  useEffect(() => {
    if (!ctx) return;
    ctx.setSubNav(path, items);
    return () => ctx.clearSubNav(path);
  }, [ctx, path, items]);
}
