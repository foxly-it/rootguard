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

fetch("project-data.json", { cache: "no-cache" })
  .then((response) => {
    if (!response.ok) throw new Error(`Project data request failed: ${response.status}`);
    return response.json();
  })
  .then((data) => {
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
