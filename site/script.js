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
  if (!projectData) return;
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
    ? releases.slice(0, 3).map(createReleaseCard)
    : [createEmptyState(currentLanguage === "de" ? "Noch keine Releases vorhanden." : "No releases yet.")]));
}

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
    document.getElementById("project-updated").textContent = currentLanguage === "de"
      ? "Live-Daten sind vorübergehend nicht verfügbar."
      : "Live data is temporarily unavailable.";
  });
