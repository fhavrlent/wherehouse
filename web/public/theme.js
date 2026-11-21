// Applies the saved theme before first paint to avoid a light/dark flash.
(function () {
  try {
    var t = localStorage.getItem("wherehouse-theme") || "system";
    var dark = t === "dark" || (t === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
    document.documentElement.classList.toggle("dark", dark);
    // A chosen theme also themes the page before the styles load.
    if (t !== "system") document.querySelector('meta[name="color-scheme"]').content = t;
  } catch (e) {}
})();
