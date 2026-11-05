(function () {
  "use strict";

  const root = document.documentElement;
  const mq = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;
  const themeButton = document.querySelector("[data-theme-toggle]");

  function theme() {
    if (root.dataset.theme === "light" || root.dataset.theme === "dark") {
      return root.dataset.theme;
    }
    return mq && mq.matches ? "dark" : "light";
  }

  function syncTheme() {
    if (!themeButton) return;
    const next = theme() === "dark" ? "light" : "dark";
    themeButton.setAttribute("aria-label", "Switch to " + next + " theme");
    themeButton.setAttribute("title", "Switch to " + next + " theme");
  }

  if (themeButton) {
    themeButton.addEventListener("click", function () {
      root.dataset.theme = theme() === "dark" ? "light" : "dark";
      try {
        localStorage.setItem("kigumi-doc-theme", root.dataset.theme);
      } catch (e) {}
      syncTheme();
    });
    syncTheme();
  }

  if (mq && mq.addEventListener) {
    mq.addEventListener("change", function () {
      if (!root.dataset.theme) syncTheme();
    });
  }

  const navToggle = document.querySelector("[data-nav-toggle]");

  function setNav(open) {
    document.body.classList.toggle("nav-open", open);
    if (navToggle) navToggle.setAttribute("aria-expanded", open ? "true" : "false");
  }

  if (navToggle) {
    navToggle.addEventListener("click", function () {
      setNav(!document.body.classList.contains("nav-open"));
    });
  }

  for (const closer of document.querySelectorAll("[data-nav-close]")) {
    closer.addEventListener("click", function () {
      setNav(false);
    });
  }

  const search = document.querySelector("#kg-search");
  const navEmpty = document.querySelector(".nav-empty");
  const packages = document.querySelectorAll("[data-search]");
  const groups = document.querySelectorAll(".nav-group");
  const cards = document.querySelectorAll("[data-card-search]");
  const cardGroups = document.querySelectorAll("[data-card-group]");
  const cardEmpty = document.querySelector("[data-card-empty]");

  function visible(el) {
    return !el.hidden;
  }

  function filterPackages() {
    if (!search) return;
    const q = search.value.trim().toLowerCase();
    let shown = 0;
    for (const pkg of packages) {
      const hit = !q || (pkg.getAttribute("data-search") || "").indexOf(q) !== -1;
      pkg.hidden = !hit;
      if (hit) shown++;
    }
    for (const group of groups) {
      let any = false;
      for (const item of group.querySelectorAll("[data-search]")) {
        if (visible(item)) {
          any = true;
          break;
        }
      }
      group.hidden = !any;
    }
    if (navEmpty) navEmpty.hidden = shown !== 0;

    let cardShown = 0;
    for (const card of cards) {
      const hit = !q || (card.getAttribute("data-card-search") || "").indexOf(q) !== -1;
      card.hidden = !hit;
      if (hit) cardShown++;
    }
    for (const group of cardGroups) {
      let any = false;
      for (const item of group.querySelectorAll("[data-card-search]")) {
        if (visible(item)) {
          any = true;
          break;
        }
      }
      group.hidden = !any;
    }
    if (cardEmpty) cardEmpty.hidden = cardShown !== 0;
  }

  if (search) search.addEventListener("input", filterPackages);

  document.addEventListener("keydown", function (e) {
    if (e.key === "/" && !e.ctrlKey && !e.metaKey && !e.altKey) {
      const target = e.target;
      if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable)) return;
      if (search) {
        e.preventDefault();
        search.focus();
        search.select();
      }
    }
    if (e.key === "Escape") {
      setNav(false);
      if (search && document.activeElement === search && search.value) {
        search.value = "";
        filterPackages();
      }
    }
  });

  const tocLinks = document.querySelectorAll(".toc a[href^='#']");
  if (tocLinks.length && window.IntersectionObserver) {
    const linkById = Object.create(null);
    for (const link of tocLinks) {
      linkById[link.getAttribute("href").slice(1)] = link;
    }
    let active = null;
    const observer = new IntersectionObserver(function (entries) {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        if (active) {
          active.classList.remove("is-active");
          active.removeAttribute("aria-current");
        }
        active = linkById[entry.target.id];
        if (active) {
          active.classList.add("is-active");
          active.setAttribute("aria-current", "location");
        }
      }
    }, { rootMargin: "-20% 0px -68% 0px" });

    for (const id in linkById) {
      if (!Object.prototype.hasOwnProperty.call(linkById, id)) continue;
      const heading = document.getElementById(id);
      if (heading) observer.observe(heading);
    }
  }

  for (const block of document.querySelectorAll(".code-block")) {
    const pre = block.querySelector("pre");
    if (!pre) continue;

    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "copy-code";
    btn.textContent = "Copy";
    btn.setAttribute("aria-label", "Copy code");

    btn.addEventListener("click", function () {
      const text = pre.textContent;

      function reset() {
        btn.textContent = "Copied";
        window.setTimeout(function () {
          btn.textContent = "Copy";
        }, 1200);
      }

      function fallback() {
        const ta = document.createElement("textarea");
        ta.value = text;
        ta.setAttribute("readonly", "readonly");
        ta.style.position = "fixed";
        ta.style.opacity = "0";
        document.body.appendChild(ta);
        ta.select();
        try {
          document.execCommand("copy");
          reset();
        } catch (e) {}
        document.body.removeChild(ta);
      }

      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(reset, fallback);
      } else {
        fallback();
      }
    });

    block.appendChild(btn);
  }
})();
