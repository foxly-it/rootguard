document.getElementById("m-host").textContent = location.hostname;
document.getElementById("m-time").textContent = new Date().toLocaleString("de-DE");

fetch("/clientip.txt", { cache: "no-store" })
  .then((r) => (r.ok ? r.text() : ""))
  .then((ip) => { document.getElementById("m-ip").textContent = ip.trim() || "unbekannt"; })
  .catch(() => { document.getElementById("m-ip").textContent = "unbekannt"; });

// Highlights the reason card that actually matched this request. Any
// failure here (offline, AdGuard down, request timeout) just leaves the
// page's default state - every reason shown plainly, none highlighted or
// dimmed - so a lookup failure never breaks the page, it just makes it
// less specific.
fetch("/api/reason", { cache: "no-store", signal: AbortSignal.timeout(2500) })
  .then((r) => (r.ok ? r.json() : { available: false }))
  .then((data) => {
    if (!data.available) return;
    const cards = document.querySelectorAll("#reasons div[data-reason]");
    let matched = false;
    cards.forEach((card) => {
      if (card.dataset.reason === data.reason) {
        card.classList.add("reason-match");
        matched = true;
      } else {
        card.classList.add("reason-dim");
      }
    });
    if (matched) {
      document.getElementById("reasons-note").hidden = false;
    }
  })
  .catch(() => {});
