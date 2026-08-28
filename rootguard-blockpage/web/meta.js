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
const REASON_INFO = {
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
};

// Highlights the real reason this specific request was blocked. Any
// failure here (offline, AdGuard down, request timeout, or a reason we
// don't have a phrase for - e.g. "NotFilteredNotFound") just leaves the
// headline/lead in their default, generic state - already what a
// JS-disabled or pre-fetch render shows, so there's no separate fallback
// path to maintain.
fetch("/api/reason", { cache: "no-store", signal: AbortSignal.timeout(2500) })
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
