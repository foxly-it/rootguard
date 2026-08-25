import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// React Testing Library's auto-cleanup relies on a global afterEach hook,
// which only exists if vitest's `test.globals` option is enabled - this
// project doesn't (every test file imports its own describe/it/expect from
// "vitest" explicitly), so without this each test file's later tests would
// find every earlier test's still-mounted DOM tree still in the document.
afterEach(() => {
  cleanup();
});
