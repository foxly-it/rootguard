document.getElementById("m-host").textContent = location.hostname;
document.getElementById("m-time").textContent = new Date().toLocaleString("de-DE");

fetch("/clientip.txt", { cache: "no-store" })
  .then((r) => (r.ok ? r.text() : ""))
  .then((ip) => { document.getElementById("m-ip").textContent = ip.trim() || "unbekannt"; })
  .catch(() => { document.getElementById("m-ip").textContent = "unbekannt"; });

// What AdGuard's check_host reasons actually mean for someone reading this
// page. FilteredBlackList covers both ad/tracking and generic threat-list
// hits - check_host can't distinguish those any further, so "Filterliste"
// stays deliberately general rather than claiming precision that isn't
// there.
//
// Object.create(null) rather than a plain object literal - found in
// review: REASON_INFO[data.reason] below is an unguarded bracket lookup
// that, on a plain object, walks the prototype chain too. A reason value
// of "constructor" (or "__proto__"/"toString"/"valueOf") would resolve
// to the inherited Object.prototype member instead of failing the `if
// (!info) return;` guard, rendering literal "undefined" into the page.
// Not reachable today (the value comes from AdGuard's own check_host
// enum, relayed verbatim by Core) - fixed as a latent hole regardless,
// since api/reason's response shape isn't this file's to trust forever.
const REASON_INFO = Object.assign(Object.create(null), {
  FilteredBlackList: {
    label: "Filterliste",
    clause: "weil die Domain auf einer Filterliste für Werbung, Tracking oder bekannte Bedrohungen steht",
  },
  SafeBrowsing: {
    label: "Malware oder Phishing",
    clause: "weil die Domain als Malware- oder Phishing-Quelle bekannt ist",
  },
  FilteredCustomRule: {
    label: "eine manuelle Sperrregel",
    clause: "aufgrund einer manuellen Sperrregel",
  },
  FilteredBlockedService: {
    label: "einen gesperrten Dienst",
    clause: "weil sie zu einem gesperrten Dienst gehört",
  },
  Parental: {
    label: "den Jugendschutz",
    clause: "aufgrund der Jugendschutz-Einstellungen",
  },
});

// Highlights the real reason this specific request was blocked. Any
// failure here (offline, AdGuard down, request timeout, or a reason we
// don't have a phrase for - e.g. "NotFilteredNotFound") just leaves the
// headline/lead in their default, generic state - already what a
// JS-disabled or pre-fetch render shows, so there's no separate fallback
// path to maintain.
//
// AbortSignal.timeout requires a 2022-era browser (Chrome 103/Firefox
// 100/Safari 16) - found in review: calling it directly as an argument
// throws a TypeError during argument evaluation on anything older, which
// happens *before* fetch is ever called, so the .catch() below - written
// for exactly this "degrade to the generic headline" case - never gets a
// chance to attach and the failure surfaces as an uncaught page error
// instead. This page's audience is every device on the LAN, aging
// smart-TVs/tablets/WebViews included, more so than the admin UI.
// Feature-detected instead; without it, nginx's own proxy_read_timeout
// (2s, see blockpage.conf.template) still bounds the request.
const reasonRequestOptions = { cache: "no-store" };
if (typeof AbortSignal !== "undefined" && typeof AbortSignal.timeout === "function") {
  reasonRequestOptions.signal = AbortSignal.timeout(2500);
}
fetch("/api/reason", reasonRequestOptions)
  .then((r) => (r.ok ? r.json() : {}))
  .then((data) => {
    const info = REASON_INFO[data.reason];
    if (!info) return;
    const headline = document.getElementById("headline");
    const span = document.createElement("span");
    span.className = "headline-reason";
    span.textContent = " " + info.label + ".";
    headline.textContent = "Diese Seite ist blockiert -";
    headline.appendChild(span);
    const lead = document.getElementById("lead");
    const hostname = document.createElement("strong");
    hostname.textContent = location.hostname;
    lead.textContent = "RootGuard hat den Zugriff auf ";
    lead.appendChild(hostname);
    lead.appendChild(document.createTextNode(" blockiert, " + info.clause + "."));
  })
  .catch(() => {});
