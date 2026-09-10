// presetText moved out of Unbound.tsx (found in review: useUnboundData()
// needs it too, for selectPreset's own success message - previously a
// page-local helper, now shared instead of duplicated).
export function presetText(id: string, field: "name" | "description" | "bestFor", t: (key: string) => string, fallback: string) {
  const key = `unbound.preset.${id}.${field}`;
  const translated = t(key);
  return translated === key ? fallback : translated;
}
