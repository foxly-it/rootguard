document.getElementById("m-host").textContent = location.hostname;
document.getElementById("m-time").textContent = new Date().toLocaleString("de-DE");

fetch("/clientip.txt", { cache: "no-store" })
  .then((r) => (r.ok ? r.text() : ""))
  .then((ip) => { document.getElementById("m-ip").textContent = ip.trim() || "unbekannt"; })
  .catch(() => { document.getElementById("m-ip").textContent = "unbekannt"; });

// Highlights the reason card that actually matched this request. Any
// failure here (offline, AdGuard down, request timeout, or a reason we
// don't have a card for - e.g. "NotFilteredNotFound") just leaves the
// page's default state - every reason shown plainly, none highlighted or
// dimmed - so a lookup failure never breaks the page, it just makes it
// less specific. On success, nginx proxies AdGuard's check_host response
// through verbatim (just "reason"/"rule"/"rules", no wrapper field), so
// matching against a real card is the only signal we need - no separate
// "available" flag to keep in sync with the failure-path shape.
fetch("/api/reason", { cache: "no-store", signal: AbortSignal.timeout(2500) })
  .then((r) => (r.ok ? r.json() : {}))
  .then((data) => {
    const cards = Array.from(document.querySelectorAll("#reasons div[data-reason]"));
    const matchedCard = cards.find((card) => card.dataset.reason === data.reason);
    if (!matchedCard) return;
    cards.forEach((card) => {
      card.classList.toggle("reason-match", card === matchedCard);
      card.classList.toggle("reason-dim", card !== matchedCard);
    });
    document.getElementById("reasons-note").hidden = false;
  })
  .catch(() => {});
