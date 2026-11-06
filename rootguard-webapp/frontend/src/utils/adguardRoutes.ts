// AdGuard Home's hash routes are not a stable interface. v1.0's rewritten UI
// renamed the filter page from #filters to #/blocklists and has no fallback
// route, so a stale link opens an empty page without any error. Installations
// stay on the pre-1.0 UI until they update, so this has to serve both. The
// settings and DNS pages kept their paths, and #/settings and #/dns work on
// both UIs. An unknown version gets the current route: the links render
// before a status is known, and follow the real version once it arrives.
export function adguardFiltersHash(version?: string) {
  const major = Number(/^v?(\d+)\./.exec(version ?? "")?.[1]);
  return major === 0 ? "#filters" : "#/blocklists";
}
