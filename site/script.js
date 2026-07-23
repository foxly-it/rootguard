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

function setLanguage(language, persist = true) {
  if (!translations[language]) return;
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
}

document.querySelectorAll(".lang-button").forEach((button) => {
  button.addEventListener("click", () => setLanguage(button.dataset.language));
});

document.getElementById("year").textContent = new Date().getFullYear();
const preferred = localStorage.getItem("rootguard-language") || (navigator.language.startsWith("de") ? "de" : "en");
setLanguage(preferred, false);
