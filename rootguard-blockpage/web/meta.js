document.getElementById("m-host").textContent = location.hostname;
document.getElementById("m-time").textContent = new Date().toLocaleString("de-DE");

fetch("/clientip.txt", { cache: "no-store" })
  .then((r) => (r.ok ? r.text() : ""))
  .then((ip) => { document.getElementById("m-ip").textContent = ip.trim() || "unbekannt"; })
  .catch(() => { document.getElementById("m-ip").textContent = "unbekannt"; });
