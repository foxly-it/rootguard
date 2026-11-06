// Drives AdGuard Home's native UI in a real browser, the way an operator
// reaches it: logged in to the WebApp, through /adguard-ui/, following the
// deep links the WebApp itself renders.
//
// curl can't cover this. The UI only works behind the proxies because its
// bundles use relative URLs and its hash routes match the links on the
// AdGuard page - neither shows up until a browser runs the JavaScript. An
// unknown hash route doesn't even fail: AdGuard Home v1.0 answers it with an
// empty content area and a 200. So each page is proven loaded by the one API
// call only that page makes, and those calls are named the same in both the
// pre-1.0 UI and v1.0's rewrite.
//
// Usage: NODE_PATH=<dir with playwright-core> node scripts/ci/adguard-ui-smoke.cjs
// Needs ROOTGUARD_ADMIN_USER and ROOTGUARD_ADMIN_PASSWORD. Writes one
// screenshot per page to SCREENSHOT_DIR.
"use strict";

const fs = require("node:fs");
const path = require("node:path");
const { chromium } = require("playwright-core");

const baseURL = process.env.ROOTGUARD_URL || "http://127.0.0.1:18080";
const screenshotDir = process.env.SCREENSHOT_DIR || "adguard-ui-screenshots";
// GitHub's runner images ship Chrome, so nothing is downloaded for this.
const executablePath = process.env.CHROME_BIN || "/usr/bin/google-chrome";
const proxyPrefix = "/adguard-ui/";

// AdGuard's own update check, which depends on its upstream's availability
// rather than on anything under test here.
const ignoredFailures = ["control/version.json"];

// In the order the AdGuard page renders its contextual links.
const linkedPages = [
  { name: "settings", api: "control/safebrowsing/status" },
  { name: "dns", api: "control/dns_info" },
  { name: "filters", api: "control/filtering/status" },
];

async function checkPage(context, name, target, api) {
  const page = await context.newPage();
  const problems = [];
  page.on("pageerror", (error) => problems.push(`script error: ${error.message}`));
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.origin === baseURL && !url.pathname.startsWith(proxyPrefix)) {
      problems.push(`request left ${proxyPrefix}: ${url.pathname}`);
    }
  });
  page.on("response", (response) => {
    const url = response.url();
    if (url.startsWith(baseURL) && response.status() >= 400 && !ignoredFailures.some((ignored) => url.includes(ignored))) {
      problems.push(`HTTP ${response.status()}: ${url}`);
    }
  });

  const marker = page.waitForResponse((response) => response.url().includes(api), { timeout: 20_000 }).catch(() => null);
  await page.goto(`${baseURL}${target}`, { waitUntil: "load" });
  const response = await marker;
  // Polling pages never go fully idle; this only gives them a moment to
  // render before the screenshot.
  await page.waitForLoadState("networkidle", { timeout: 10_000 }).catch(() => {});
  await page.screenshot({ path: path.join(screenshotDir, `${name}.png`), fullPage: true });

  if (!response) {
    problems.push(`never requested ${api}, so the page behind ${target} did not load`);
  }
  await page.close();

  console.log(`${problems.length ? "FAILED" : "ok    "} ${name}: ${target}`);
  return problems.map((problem) => `${name}: ${problem}`);
}

async function main() {
  fs.mkdirSync(screenshotDir, { recursive: true });
  const browser = await chromium.launch({ executablePath });
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const login = await context.request.post(`${baseURL}/api/auth/login`, {
      data: { username: process.env.ROOTGUARD_ADMIN_USER, password: process.env.ROOTGUARD_ADMIN_PASSWORD },
    });
    if (!login.ok()) {
      throw new Error(`WebApp login failed with HTTP ${login.status()}`);
    }

    // The filter link depends on the AdGuard version, which the page only
    // loads on a finished installation. This stack is started directly by
    // compose and never runs the installer, so report that one state as
    // finished - the status behind it is still the real AdGuard's.
    await context.route("**/api/installation", (route) =>
      route.fulfill({ json: { state: "installed", steps: [], updated_at: new Date().toISOString() } }),
    );

    const adguardPage = await context.newPage();
    await adguardPage.goto(`${baseURL}/adguard`);
    await adguardPage.locator(".adguard-upstream code", { hasText: /^v\d/ }).waitFor();
    const links = await adguardPage.$$eval("a.adguard-contextual-button", (anchors) => anchors.map((anchor) => anchor.getAttribute("href")));
    await adguardPage.screenshot({ path: path.join(screenshotDir, "rootguard-adguard-page.png"), fullPage: true });
    await adguardPage.close();
    if (links.length !== linkedPages.length) {
      throw new Error(`expected ${linkedPages.length} deep links on the AdGuard page, found: ${links.join(", ")}`);
    }

    const problems = await checkPage(context, "dashboard", proxyPrefix, "control/status");
    for (const [index, { name, api }] of linkedPages.entries()) {
      problems.push(...(await checkPage(context, name, links[index], api)));
    }
    if (problems.length) {
      throw new Error(`AdGuard UI check failed:\n${problems.join("\n")}`);
    }
  } finally {
    await browser.close();
  }
}

main().catch((error) => {
  console.error(error.message);
  process.exit(1);
});
