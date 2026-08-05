# RootGuard locale interface

Every locale is a `LocaleDefinition` with a stable language code, a label, and
a flat message catalog. German is the fallback catalog; another locale may be
introduced incrementally because missing keys fall back to German.

To add a language:

1. create `src/i18n/<code>.ts` implementing `LocaleDefinition`;
2. import it during application bootstrap;
3. call `registerLocale(locale)` before `I18nProvider` is mounted.

Components use `useI18n()` for `t()`, locale-aware date formatting, and the
current locale. They never branch on a particular language code.

Placeholders use `{name}` syntax:

```ts
t("stack.runningCount", { count: 2 })
```

The selected locale is stored as `rootguard.locale`, follows the browser
language on first use, and updates the document `lang` attribute.
