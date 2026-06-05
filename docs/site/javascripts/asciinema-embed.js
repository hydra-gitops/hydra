(function () {
  function castUrl(slug) {
    var base = document.querySelector("base");
    var prefix = base ? base.getAttribute("href") || "/" : "/";
    if (!prefix.endsWith("/")) {
      prefix += "/";
    }
    return prefix + "asciinema/help/" + slug + ".cast";
  }

  function initPlayers() {
    if (typeof AsciinemaPlayer === "undefined") {
      return;
    }
    document.querySelectorAll(".hydra-asciinema[data-cast-slug]").forEach(function (el) {
      if (el.dataset.initialized === "1") {
        return;
      }
      var slug = el.getAttribute("data-cast-slug");
      if (!slug) {
        return;
      }
      el.dataset.initialized = "1";
      // v3 API: create(src, containerElement, opts)
      AsciinemaPlayer.create(castUrl(slug), el, {
        autoPlay: true,
        preload: true,
        fit: "width",
        terminalFontSize: "small",
        // Always show the timeline scrubber (playback history), not only on hover.
        controls: true,
        // Preserve every output frame when stepping with "," / "." while paused.
        minFrameTime: 0,
      });
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initPlayers);
  } else {
    initPlayers();
  }

  document$.subscribe(function () {
    initPlayers();
  });
})();
