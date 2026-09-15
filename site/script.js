const translations = {
  de: {
    title: "RootGuard – DNS-Schutz unter deiner Kontrolle",
    description: "RootGuard vereint AdGuard Home, Unbound und eine sichere Weboberfläche zu einem selbst betriebenen DNS-Schutz für dein Netzwerk."
  },
  en: {
    title: "RootGuard – DNS protection under your control",
    description: "RootGuard combines AdGuard Home, Unbound, and a secure web interface into self-hosted DNS protection for your network."
  }
};

let currentLanguage = "de";
let projectData = null;

function formatDate(value, includeTime = false) {
  if (!value) return "–";
  const options = { day: "2-digit", month: "2-digit", year: "numeric" };
  if (includeTime) Object.assign(options, { hour: "2-digit", minute: "2-digit" });
  return new Intl.DateTimeFormat(currentLanguage === "de" ? "de-DE" : "en-GB", options).format(new Date(value));
}

function createProjectRow(item, type) {
  const row = document.createElement("li");
  const link = document.createElement("a");
  const meta = document.createElement("span");
  const title = document.createElement("strong");
  const date = document.createElement("small");

  link.href = item.html_url;
  link.target = "_blank";
  link.rel = "noopener noreferrer";
  meta.textContent = type === "commit" ? item.sha : `#${item.number}`;
  title.textContent = item.message || item.title;
  date.textContent = formatDate(item.date || item.updated_at);
  link.append(meta, title, date);
  row.append(link);
  return row;
}

function createEmptyState(message) {
  const item = document.createElement("li");
  item.className = "project-empty";
  item.textContent = message;
  return item;
}

function createReleaseCard(release, index) {
  const article = document.createElement("article");
  const heading = document.createElement("div");
  const meta = document.createElement("span");
  const title = document.createElement("h4");
  const summary = document.createElement("p");
  const highlights = document.createElement("ul");
  const link = document.createElement("a");

  article.className = `release-card${index === 0 ? " latest" : ""}`;
  meta.textContent = `${release.tag} · ${formatDate(release.published_at)}`;
  title.textContent = release.name || release.tag;
  summary.textContent = release.summary || "";
  (release.highlights || []).slice(0, 4).forEach((highlight) => {
    const item = document.createElement("li");
    item.textContent = highlight;
    highlights.append(item);
  });
  link.href = release.html_url;
  link.target = "_blank";
  link.rel = "noopener noreferrer";
  link.textContent = currentLanguage === "de" ? "Release Notes ansehen ↗" : "View release notes ↗";
  heading.append(meta, title);
  article.append(heading, summary, highlights, link);
  return article;
}

function renderProjectData() {
  if (!projectData || !document.getElementById("project-status")) return;
  const commits = projectData.commits || [];
  const pulls = projectData.pull_requests || [];
  const releases = projectData.releases || [];
  const currentRelease = releases[0];
  const latestCommit = commits[0];

  document.getElementById("current-version").textContent = projectData.current_version || "–";
  const demoVersion = document.getElementById("install-demo-version");
  if (demoVersion && projectData.current_version) demoVersion.textContent = projectData.current_version;
  document.getElementById("open-pr-count").textContent = pulls.length;
  document.getElementById("release-count").textContent = releases.length;
  if (currentRelease) document.getElementById("version-link").href = currentRelease.html_url;
  if (latestCommit) {
    document.getElementById("latest-commit").textContent = latestCommit.sha;
    document.getElementById("latest-commit-date").textContent = formatDate(latestCommit.date);
    document.getElementById("commit-link").href = latestCommit.html_url;
  }

  const updated = document.getElementById("project-updated");
  updated.textContent = currentLanguage === "de"
    ? `Automatisch aktualisiert · Stand ${formatDate(projectData.generated_at, true)} Uhr`
    : `Updated automatically · As of ${formatDate(projectData.generated_at, true)}`;

  const commitList = document.getElementById("commit-list");
  commitList.replaceChildren(...(commits.length
    ? commits.slice(0, 5).map((commit) => createProjectRow(commit, "commit"))
    : [createEmptyState(currentLanguage === "de" ? "Noch keine Commits vorhanden." : "No commits yet.")]));

  const pullList = document.getElementById("pull-list");
  pullList.replaceChildren(...(pulls.length
    ? pulls.slice(0, 5).map((pull) => createProjectRow(pull, "pull"))
    : [createEmptyState(currentLanguage === "de" ? "Aktuell sind keine Pull Requests offen." : "There are currently no open pull requests.")]));

  const releaseList = document.getElementById("release-list");
  releaseList.replaceChildren(...(releases.length
    ? releases.slice(0, 1).map(createReleaseCard)
    : [createEmptyState(currentLanguage === "de" ? "Noch keine Releases vorhanden." : "No releases yet.")]));
}

function initializeManualNavigation() {
  const navigation = document.querySelector(".manual-nav");
  if (!navigation) return;
  const links = [...navigation.querySelectorAll('a[href^="#"]')];
  const sections = links
    .map((link) => document.getElementById(link.hash.slice(1)))
    .filter(Boolean);

  let currentSectionId = "";
  let updateScheduled = false;

  const markCurrent = (sectionId) => {
    if (!sectionId || sectionId === currentSectionId) return;
    currentSectionId = sectionId;
    links.forEach((link) => {
      const isCurrent = link.hash === `#${sectionId}`;
      link.classList.toggle("current", isCurrent);
      if (isCurrent) link.setAttribute("aria-current", "location");
      else link.removeAttribute("aria-current");
      if (isCurrent) {
        const linkTop = link.offsetTop;
        const linkBottom = linkTop + link.offsetHeight;
        if (linkTop < navigation.scrollTop + 12) navigation.scrollTop = Math.max(0, linkTop - 12);
        else if (linkBottom > navigation.scrollTop + navigation.clientHeight - 12) {
          navigation.scrollTop = linkBottom - navigation.clientHeight + 12;
        }
      }
    });
  };

  const updateCurrentSection = () => {
    updateScheduled = false;
    const readingLine = Math.min(180, window.innerHeight * 0.24);
    let current = sections[0];

    for (const section of sections) {
      if (section.getBoundingClientRect().top <= readingLine) current = section;
      else break;
    }

    const documentBottom = document.documentElement.scrollHeight - 2;
    if (window.scrollY + window.innerHeight >= documentBottom) current = sections.at(-1);
    markCurrent(current?.id);
  };

  const scheduleUpdate = () => {
    if (updateScheduled) return;
    updateScheduled = true;
    requestAnimationFrame(updateCurrentSection);
  };

  window.addEventListener("scroll", scheduleUpdate, { passive: true });
  window.addEventListener("resize", scheduleUpdate);
  scheduleUpdate();
}

function initializeHeaderNavigation() {
  const dropdowns = [...document.querySelectorAll(".nav-dropdown")];
  if (!dropdowns.length) return;

  dropdowns.forEach((dropdown) => {
    dropdown.addEventListener("toggle", () => {
      if (!dropdown.open) return;
      dropdowns.forEach((other) => {
        if (other !== dropdown) other.open = false;
      });
    });
    dropdown.querySelectorAll("a").forEach((link) => {
      link.addEventListener("click", () => { dropdown.open = false; });
    });
  });

  document.addEventListener("click", (event) => {
    if (event.target.closest(".nav-dropdown")) return;
    dropdowns.forEach((dropdown) => { dropdown.open = false; });
  });

  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") return;
    const openDropdown = dropdowns.find((dropdown) => dropdown.open);
    if (!openDropdown) return;
    openDropdown.open = false;
    openDropdown.querySelector("summary")?.focus();
  });
}

function initializeBackToTop() {
  const label = { de: "Nach oben scrollen", en: "Scroll to top" };
  const button = document.createElement("button");
  button.type = "button";
  button.id = "back-to-top";
  button.setAttribute("aria-label", label[currentLanguage]);
  button.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 19V5M5 12l7-7 7 7"/></svg>';

  let visible = false;
  let ticking = false;
  const updateVisibility = () => {
    ticking = false;
    const shouldShow = window.scrollY > 500;
    if (shouldShow === visible) return;
    visible = shouldShow;
    button.classList.toggle("visible", visible);
  };

  window.addEventListener("scroll", () => {
    if (ticking) return;
    ticking = true;
    requestAnimationFrame(updateVisibility);
  }, { passive: true });

  button.addEventListener("click", () => {
    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    window.scrollTo({ top: 0, behavior: reduceMotion ? "auto" : "smooth" });
  });

  document.body.append(button);
  updateVisibility();
  return button;
}

function initializeInstallCopyButton() {
  const button = document.getElementById("install-copy-button");
  const source = document.getElementById("install-command-text");
  if (!button || !source) return;
  const label = button.querySelector("span");
  const idle = { de: "Kopieren", en: "Copy" };
  const done = { de: "Kopiert!", en: "Copied!" };
  let resetTimer = null;
  button.addEventListener("click", () => {
    navigator.clipboard.writeText(source.textContent.trim()).then(() => {
      button.classList.add("copied");
      label.textContent = done[currentLanguage];
      window.clearTimeout(resetTimer);
      resetTimer = window.setTimeout(() => {
        button.classList.remove("copied");
        label.textContent = idle[currentLanguage];
      }, 1800);
    }).catch(() => {});
  });
}

// Lines start visible-but-transparent (see the prefers-reduced-motion rule
// in styles.css) and get a per-line transition-delay set here, then a
// single .revealed class added once the terminal scrolls into view -
// staggering via JS-computed delays rather than a fixed set of :nth-child
// CSS rules keeps this working if a line gets added/removed later without
// anyone remembering to update a matching CSS selector list. Skips the
// stagger (and the observer entirely) under prefers-reduced-motion, where
// the CSS never hides the lines in the first place - nothing to reveal.
function initializeInstallDemoReveal() {
  const terminal = document.getElementById("install-demo-terminal");
  if (!terminal) return;
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  const lines = terminal.querySelectorAll(".line");
  lines.forEach((line, index) => {
    line.style.transitionDelay = `${index * 130}ms`;
  });
  const observer = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (!entry.isIntersecting) return;
      terminal.classList.add("revealed");
      observer.disconnect();
    });
  }, { threshold: 0.3 });
  observer.observe(terminal);
}

// Viewport-wide emoji confetti plus a short toast to mark the 1.0 release.
// Runs once per browser (localStorage) on whichever page loads first, and
// is skipped entirely under prefers-reduced-motion - nothing here carries
// any information, it's purely decorative.
function initializeReleaseCelebration() {
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  if (localStorage.getItem("rootguard-seen-1.0-celebration")) return;
  localStorage.setItem("rootguard-seen-1.0-celebration", "1");

  const panel = document.querySelector(".project-status-panel");
  if (panel) panel.classList.add("just-shipped");

  const emoji = ["🎉", "🛡️", "✨", "🎊"];
  const confetti = document.createElement("div");
  confetti.className = "release-confetti";
  confetti.setAttribute("aria-hidden", "true");
  for (let i = 0; i < 40; i++) {
    const piece = document.createElement("span");
    piece.className = "confetti-piece";
    piece.textContent = emoji[i % emoji.length];
    piece.style.left = `${Math.random() * 100}%`;
    piece.style.animationDelay = `${Math.random() * 700}ms`;
    piece.style.setProperty("--drift", `${Math.random() * 160 - 80}px`);
    piece.style.setProperty("--spin", `${Math.random() * 360 - 180}deg`);
    confetti.appendChild(piece);
  }
  document.body.appendChild(confetti);
  setTimeout(() => confetti.remove(), 3600);

  const toast = document.createElement("div");
  toast.className = "release-toast";
  toast.setAttribute("role", "status");
  const message = currentLanguage === "de" ? "RootGuard 1.0 ist endlich da!" : "RootGuard 1.0 is finally here!";
  toast.innerHTML = `<span class="emoji" aria-hidden="true">🎉</span><span>${message}</span>`;
  document.body.appendChild(toast);
  setTimeout(() => toast.remove(), 4300);
}

// Shared fade+rise reveal for any .reveal element, replacing a bespoke
// observer per section. Delay is staggered per sibling group (elements
// sharing the same parent, e.g. cards in the same grid) rather than
// globally, so unrelated sections don't inherit each other's timing. A
// no-op under prefers-reduced-motion: styles.css never applies the
// opacity:0 rule there in the first place, so there's nothing to reveal.
function initializeScrollReveal() {
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  const elements = [...document.querySelectorAll(".reveal")];
  if (!elements.length) return;

  const groups = new Map();
  elements.forEach((el) => {
    const siblings = groups.get(el.parentElement) || [];
    siblings.push(el);
    groups.set(el.parentElement, siblings);
  });
  groups.forEach((siblings) => {
    siblings.forEach((el, index) => {
      el.style.transitionDelay = `${index * 90}ms`;
    });
  });

  const observer = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      if (!entry.isIntersecting) return;
      entry.target.classList.add("revealed");
      observer.unobserve(entry.target);
    });
  }, { threshold: 0.15 });
  elements.forEach((el) => observer.observe(el));
}

// Tabbed real-screenshot showcase (the "See it in action" section). Only
// runs on pages that have it (index.html). Auto-advances every 6s unless
// reduced motion is preferred, in which case it just shows the first tab
// and waits for manual clicks - no timer fighting that preference.
function initializeScreenshotShowcase() {
  const section = document.querySelector(".showcase");
  if (!section) return;
  const tabs = [...section.querySelectorAll(".showcase-tab")];
  const images = [...section.querySelectorAll(".showcase-image")];
  const urlLabel = document.getElementById("showcase-url");
  if (!tabs.length || !images.length) return;

  function activate(target) {
    tabs.forEach((tab) => {
      const active = tab.dataset.target === target;
      tab.classList.toggle("active", active);
      tab.setAttribute("aria-selected", String(active));
    });
    images.forEach((img) => img.classList.toggle("active", img.dataset.key === target));
    const activeTab = tabs.find((tab) => tab.dataset.target === target);
    if (activeTab && urlLabel) urlLabel.textContent = activeTab.dataset.url;
  }

  let timer = null;
  function restartAutoAdvance() {
    clearInterval(timer);
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    timer = setInterval(() => {
      const currentIndex = tabs.findIndex((tab) => tab.classList.contains("active"));
      const next = tabs[(currentIndex + 1) % tabs.length];
      activate(next.dataset.target);
    }, 6000);
  }

  tabs.forEach((tab) => {
    tab.addEventListener("click", () => {
      activate(tab.dataset.target);
      restartAutoAdvance();
    });
  });
  restartAutoAdvance();
}

const backToTopButton = initializeBackToTop();

function setLanguage(language, persist = true) {
  if (!translations[language]) return;
  currentLanguage = language;
  document.documentElement.lang = language;
  document.querySelectorAll("[data-de][data-en]").forEach((element) => {
    element.innerHTML = element.dataset[language];
  });
  document.querySelectorAll(".lang-button").forEach((button) => {
    button.classList.toggle("active", button.dataset.language === language);
  });
  backToTopButton.setAttribute("aria-label", language === "de" ? "Nach oben scrollen" : "Scroll to top");
  const pageTitle = document.body.dataset[language === "de" ? "titleDe" : "titleEn"];
  const pageDescription = document.body.dataset[language === "de" ? "descriptionDe" : "descriptionEn"];
  document.title = pageTitle || translations[language].title;
  document.querySelector('meta[name="description"]').content = pageDescription || translations[language].description;
  if (persist) localStorage.setItem("rootguard-language", language);
  renderProjectData();
}

document.querySelectorAll(".lang-button").forEach((button) => {
  button.addEventListener("click", () => setLanguage(button.dataset.language));
});

document.getElementById("year").textContent = new Date().getFullYear();
const preferred = localStorage.getItem("rootguard-language") || (navigator.language.startsWith("de") ? "de" : "en");
setLanguage(preferred, false);
initializeHeaderNavigation();
initializeManualNavigation();
initializeInstallCopyButton();
initializeInstallDemoReveal();
initializeReleaseCelebration();
initializeScrollReveal();
initializeScreenshotShowcase();

fetch("project-data.json", { cache: "no-cache" })
  .then((response) => {
    if (!response.ok) throw new Error(`Project data request failed: ${response.status}`);
    return response.json();
  })
  .then((data) => {
    const staticVersion = document.getElementById("current-version")?.textContent.trim();
    if (staticVersion && data.current_version && data.current_version !== staticVersion) {
      throw new Error(`Stale project data: ${data.current_version} != ${staticVersion}`);
    }
    projectData = data;
    renderProjectData();
  })
  .catch(() => {
    const projectUpdated = document.getElementById("project-updated");
    if (projectUpdated) {
      projectUpdated.textContent = currentLanguage === "de"
        ? "Live-Daten sind vorübergehend nicht verfügbar."
        : "Live data is temporarily unavailable.";
    }
  });
