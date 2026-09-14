(function () {
  var root = document.documentElement;
  var btn = document.getElementById("themeBtn");
  if (!btn) return;

  var modes = ["system", "light", "dark"];
  var SVG_NS = "http://www.w3.org/2000/svg";

  function svgEl(tag, attrs) {
    var el = document.createElementNS(SVG_NS, tag);
    for (var key in attrs) el.setAttribute(key, attrs[key]);
    return el;
  }

  // Built as real DOM nodes instead of an innerHTML-assigned markup
  // string - found in review: innerHTML here was only ever fed these
  // three fixed, developer-written strings (never anything
  // attacker-influenced, so not exploitable as things stand), but a sink
  // that happens to be safe today is still a sink. Building each icon as
  // actual SVG elements removes it from this file entirely instead of
  // relying on "nothing dangerous reaches it" staying true forever.
  var icons = {
    system: (function () {
      var svg = svgEl("svg", { viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", "stroke-width": "2", "stroke-linecap": "round", "stroke-linejoin": "round", "aria-hidden": "true" });
      svg.appendChild(svgEl("circle", { cx: "12", cy: "12", r: "4" }));
      svg.appendChild(svgEl("path", { d: "M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" }));
      return svg;
    })(),
    light: (function () {
      var svg = svgEl("svg", { viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", "stroke-width": "2", "stroke-linecap": "round", "stroke-linejoin": "round", "aria-hidden": "true" });
      svg.appendChild(svgEl("circle", { cx: "12", cy: "12", r: "4" }));
      return svg;
    })(),
    dark: (function () {
      var svg = svgEl("svg", { viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", "stroke-width": "2", "stroke-linecap": "round", "stroke-linejoin": "round", "aria-hidden": "true" });
      svg.appendChild(svgEl("path", { d: "M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z" }));
      return svg;
    })()
  };

  // Found in review: an invalid stored value (a leftover from an older
  // schema, or simply hand-edited/corrupted localStorage - nothing
  // requires it to be one of the three real modes) made icons[mode]
  // undefined, and btn.appendChild(undefined) throws - crashing this
  // whole IIFE partway through applyTheme's very first call at startup,
  // before the click listener below is even registered. The theme
  // toggle button stayed permanently dead until the stale value was
  // manually cleared. Falls back to "system" for anything not one of
  // the three real modes instead of trusting the stored value blindly.
  //
  // Found in review, round 2: reading (or even just accessing) the
  // localStorage property itself throws a SecurityError in several real
  // configurations (site data blocked, "block all cookies", private/
  // lockdown browsing modes) - not just an invalid value, the property
  // access never returns at all. Since this ran before btn's click
  // listener was registered, the same "toggle permanently dead" failure
  // as above, through a trigger the earlier fix didn't cover. Guarded so
  // a throw degrades to "toggle still works, choice just isn't
  // remembered across reloads" instead.
  function storedMode() {
    try {
      var stored = localStorage.getItem("rootguard.blockpage.theme");
      return modes.indexOf(stored) !== -1 ? stored : "system";
    } catch (e) {
      return "system";
    }
  }

  function rememberMode(mode) {
    try {
      localStorage.setItem("rootguard.blockpage.theme", mode);
    } catch (e) {
      // Storage unavailable - currentMode below still tracks the choice
      // for the rest of this page view, it just won't survive a reload.
    }
  }

  // Tracked in memory rather than re-reading storage on every click: if
  // storage is blocked, rememberMode above silently never persists a
  // choice, so re-deriving "current" from storedMode() on each click
  // would keep landing back on "system" and the toggle could only ever
  // cycle between its first two modes, never reaching the third.
  var currentMode = storedMode();

  function applyTheme(mode) {
    currentMode = mode;
    if (mode === "system") root.removeAttribute("data-theme");
    else root.setAttribute("data-theme", mode);
    while (btn.firstChild) btn.removeChild(btn.firstChild);
    btn.appendChild(icons[mode]);
    rememberMode(mode);
  }

  applyTheme(currentMode);
  btn.addEventListener("click", function () {
    applyTheme(modes[(modes.indexOf(currentMode) + 1) % modes.length]);
  });
})();
