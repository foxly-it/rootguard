// Found in review: the "Erneut prüfen" button used location.reload()
// through an inline onclick attribute - blocked outright by this page's
// own CSP (script-src 'self', no 'unsafe-inline'/'unsafe-hashes', see
// blockpage.conf.template), since script-src also governs inline event
// handlers. The button rendered normally and simply did nothing on
// click, on the one page every blocked request in the product lands on.
// A tiny external, same-origin script sidesteps that entirely - no hash
// to keep in sync (the actual bug that broke the WebApp's own inline
// theme script elsewhere in this codebase), unlike an inline <script>
// would need. Plain var/function, matching theme.js's own ES5 style -
// this page's audience includes old smart-TV/tablet browsers.
(function () {
  var button = document.getElementById("reload-button");
  if (!button) return;
  button.addEventListener("click", function () {
    location.reload();
  });
})();
