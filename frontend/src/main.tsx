import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router";
import App from "./App";
import { I18nProvider } from "./i18n";
import { AuthProvider } from "./auth";
import "./index.css";
import "./styles/header.css";
import "./styles/card.css";
import "./styles/status.css";
import "./styles/dashboard.css";
import "./styles/buttons.css";
import "./styles/motion.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <I18nProvider>
        <AuthProvider>
          <App />
        </AuthProvider>
      </I18nProvider>
    </BrowserRouter>
  </React.StrictMode>
);
