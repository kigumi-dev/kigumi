try {
  const theme = localStorage.getItem("kigumi-doc-theme");
  if (theme === "light" || theme === "dark") {
    document.documentElement.dataset.theme = theme;
  }
} catch (e) {}
