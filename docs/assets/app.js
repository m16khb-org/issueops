/* IssueOps site interactions. Every widget renders from #site-data and stays
   keyboard operable; motion stops under prefers-reduced-motion. */
(function () {
  "use strict";
  var data = JSON.parse(document.getElementById("site-data").textContent);
  var reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var $ = function (sel, root) { return (root || document).querySelector(sel); };
  var $$ = function (sel, root) { return Array.prototype.slice.call((root || document).querySelectorAll(sel)); };
  function el(tag, attrs, text) {
    var node = document.createElement(tag);
    if (attrs) Object.keys(attrs).forEach(function (k) {
      if (k === "class") node.className = attrs[k]; else node.setAttribute(k, attrs[k]);
    });
    if (text != null) node.textContent = text;
    return node;
  }
  // Arrow-key roving focus for a tablist; select(i) does the rest.
  function rove(buttons, select, vertical) {
    buttons.forEach(function (b, i) {
      b.addEventListener("keydown", function (e) {
        var next = null;
        if (e.key === (vertical ? "ArrowDown" : "ArrowRight")) next = (i + 1) % buttons.length;
        if (e.key === (vertical ? "ArrowUp" : "ArrowLeft")) next = (i - 1 + buttons.length) % buttons.length;
        if (e.key === "Home") next = 0;
        if (e.key === "End") next = buttons.length - 1;
        if (next === null) return;
        e.preventDefault();
        select(next, true);
        buttons[next].focus();
      });
    });
  }

  /* ---------- Cycle explorer ---------- */
  (function cycle() {
    var root = $("#cycle");
    if (!root) return;
    var list = $(".stages", root), detail = $(".stage-detail", root);
    var bar = $(".cycle-foot .bar i", root), toggle = $(".cycle-foot button", root);
    var buttons = data.stages.map(function (s, i) {
      var li = el("li");
      var b = el("button", { class: "stage-btn", role: "tab", id: "stage-" + i, "aria-selected": "false", "aria-controls": "stage-panel", tabindex: "-1" });
      b.appendChild(el("span", { class: "node", "aria-hidden": "true" }));
      b.appendChild(el("span", { class: "label" }, s.n + ". " + (s.short || s.name)));
      if (s.gate) b.appendChild(el("span", { class: "gate" }, "gate"));
      b.setAttribute("aria-label", s.n + ". " + s.name);
      b.addEventListener("click", function () { stopAuto(); select(i); });
      li.appendChild(b); list.appendChild(li);
      return b;
    });
    var current = 0, started = 0, elapsed = 0, playing = !reduce, visible = false, raf = 0;
    var STEP = 2800;
    function select(i, user) {
      if (user) stopAuto();
      current = i;
      buttons.forEach(function (b, j) {
        b.setAttribute("aria-selected", j === i ? "true" : "false");
        b.tabIndex = j === i ? 0 : -1;
        b.classList.toggle("past", j < i);
      });
      var s = data.stages[i];
      detail.textContent = "";
      detail.setAttribute("aria-labelledby", "stage-" + i);
      detail.appendChild(el("span", { class: "num" }, data.ui.stage + " " + s.n + "/10"));
      detail.appendChild(el("h3", null, s.name));
      var skills = el("div", { class: "skills" });
      s.skills.forEach(function (k) { skills.appendChild(el("code", null, k)); });
      detail.appendChild(skills);
      detail.appendChild(el("p", null, s.out));
      if (s.gate) detail.appendChild(el("div", { class: "gate-note" }, s.gate));
      if (s.term) {
        var t = el("div", { class: "term" }); t.appendChild(el("pre", null, s.term)); detail.appendChild(t);
      }
      started = performance.now(); elapsed = 0;
    }
    function tick(now) {
      raf = 0;
      if (!playing || !visible) return;
      var p = Math.min(1, (now - started) / STEP);
      bar.style.transform = "scaleX(" + p + ")";
      if (p >= 1) select((current + 1) % buttons.length);
      raf = requestAnimationFrame(tick);
    }
    function run() { if (!raf && playing && visible) { started = performance.now() - elapsed; raf = requestAnimationFrame(tick); } }
    function halt() { if (raf) { cancelAnimationFrame(raf); elapsed = performance.now() - started; } raf = 0; }
    function stopAuto() { playing = false; halt(); bar.style.transform = "scaleX(0)"; label(); }
    function label() { toggle.textContent = playing ? data.ui.pause : data.ui.play; toggle.setAttribute("aria-pressed", playing ? "true" : "false"); }
    toggle.addEventListener("click", function () {
      playing = !playing; label();
      if (playing) { elapsed = 0; run(); } else { halt(); }
    });
    root.addEventListener("mouseenter", halt);
    root.addEventListener("mouseleave", run);
    root.addEventListener("focusin", halt);
    root.addEventListener("focusout", run);
    new IntersectionObserver(function (es) { visible = es[0].isIntersecting; visible ? run() : halt(); }).observe(root);
    document.addEventListener("visibilitychange", function () { document.hidden ? halt() : run(); });
    rove(buttons, select, !window.matchMedia("(max-width: 560px)").matches);
    if (reduce) toggle.hidden = true;
    label();
    select(0);
  })();

  /* ---------- Record viewer ---------- */
  (function records() {
    var root = $("#record-viewer");
    if (!root) return;
    var tabs = $$(".tab", root), body = $(".file", root), path = $(".filepath", root), legend = $("#record-legend");
    var active = null;
    function show(i) {
      var kind = tabs[i].dataset.kind, rec = data.samples[kind];
      tabs.forEach(function (t, j) { t.setAttribute("aria-selected", j === i ? "true" : "false"); t.tabIndex = j === i ? 0 : -1; });
      body.textContent = "";
      body.appendChild(el("h4", null, rec.title));
      rec.fields.forEach(function (f) {
        var row = el("div", { class: "field", "data-field": f[0] });
        row.appendChild(el("b", null, f[0])); row.appendChild(el("span", null, f[1]));
        body.appendChild(row);
      });
      path.textContent = "";
      var a = el("a", { href: "https://github.com/m16khb-org/issueops/blob/main/" + rec.path }, rec.path);
      path.appendChild(a);
      legend.textContent = "";
      rec.fields.forEach(function (f) {
        var li = el("li"), b = el("button", { type: "button", "aria-pressed": "false" });
        b.appendChild(el("b", null, f[0])); b.appendChild(el("span", null, data.fieldHelp[f[0]] || ""));
        b.addEventListener("click", function () { highlight(f[0]); });
        b.addEventListener("mouseenter", function () { if (!active) mark(f[0]); });
        b.addEventListener("mouseleave", function () { if (!active) mark(null); });
        li.appendChild(b); legend.appendChild(li);
      });
      active = null; mark(null);
    }
    function mark(name) {
      $$(".field", body).forEach(function (r) { r.classList.toggle("hi", r.dataset.field === name); });
      $$("button", legend).forEach(function (b) { b.setAttribute("aria-pressed", b.firstChild.textContent === active ? "true" : "false"); });
    }
    function highlight(name) { active = active === name ? null : name; mark(active || null); }
    tabs.forEach(function (t, i) { t.addEventListener("click", function () { show(i); }); });
    rove(tabs, show, false);
    show(0);
  })();

  /* ---------- route-docs playground: same rule as internal/domain/projectdoc/records.go ---------- */
  (function route() {
    var root = $("#route-play");
    if (!root) return;
    var input = $("input", root), results = $(".results", root), termsBox = $(".terms", root);
    var KO = { "배포": "deploy", "빌드": "build", "이미지": "image", "컨테이너": "container", "디스크": "disk",
      "메모리": "memory", "서버": "server", "마이그레이션": "migration", "데이터베이스": "database",
      "테스트": "test", "커밋": "commit", "브랜치": "branch", "머지": "merge", "병합": "merge",
      "훅": "hook", "스킬": "skill", "캐시": "cache", "로그": "log", "설치": "install",
      "업데이트": "update", "권한": "permission", "인증": "auth", "세션": "session", "워크트리": "worktree",
      "의존성": "dependency", "성능": "performance", "보안": "security", "백업": "backup", "롤백": "rollback",
      "프롬프트": "prompt", "모델": "model", "스키마": "schema", "문서": "doc", "검증": "verify",
      "리뷰": "review", "이슈": "issue", "잠금": "lock" };
    var KO_STOP = new Set(("한다 하는 하고 하지 했다 하면 해야 있다 있는 없다 없는 된다 되는 되어 이다 이번 모든 그리 위해 대한 통해 " +
      "경우 때문 같은 다른 이미 아직 다시 먼저 직접 추가 변경 요약 결과 에서 경로 명령 상태 수정 구현 완료 사이 정리 계획 문제").split(" "));
    var EN_STOP = new Set(("the and for with from into that this before after was were are not but all any its has have had " +
      "when then than also only out via per one two use used uses using can must should will").split(" "));
    var HANGUL = /\p{Script=Hangul}/u, LETTER = /[\p{L}\p{Nd}]/u;
    var enc = new TextEncoder();
    function bytes(s) { return enc.encode(s).length; }
    function latinTerm(w) {
      if (bytes(w) < 3 || EN_STOP.has(w)) return null;
      var suffixes = [["ment", 7], ["ing", 6], ["ed", 5], ["s", 4]];
      for (var i = 0; i < suffixes.length; i++) {
        var s = suffixes[i][0];
        if (bytes(w) > suffixes[i][1] && w.endsWith(s) && !(s === "s" && w.endsWith("ss"))) return w.slice(0, w.length - s.length);
      }
      return w;
    }
    function addHangul(terms, w) {
      var mapped = false;
      Object.keys(KO).forEach(function (k) { if (w.indexOf(k) >= 0) { terms.add(KO[k]); mapped = true; } });
      var runes = Array.from(w);
      if (mapped || runes.length < 2) return;
      var stem = runes.slice(0, 2).join("");
      if (!KO_STOP.has(stem)) terms.add(stem);
    }
    function routeTerms(text) {
      var terms = new Set(), word = [], hangul = false;
      function flush() {
        if (!word.length) return;
        var w = word.join("");
        if (hangul) addHangul(terms, w); else { var t = latinTerm(w); if (t) terms.add(t); }
        word = [];
      }
      Array.from(text.toLowerCase()).forEach(function (ch) {
        var isH = HANGUL.test(ch), isL = !isH && LETTER.test(ch);
        if (!isH && !isL) { flush(); return; }
        if (word.length && isH !== hangul) flush();
        hangul = isH; word.push(ch);
      });
      flush();
      return terms;
    }
    var records = null, recTerms = null, df = null;
    function index(list) {
      records = list; df = new Map();
      recTerms = list.map(function (r) {
        var t = routeTerms(r.Title + " " + r.Summary + " " + r.Source);
        t.forEach(function (term) { df.set(term, (df.get(term) || 0) + 1); });
        return t;
      });
    }
    function match(task) {
      var normalized = task.trim().toLowerCase() || "general";
      var taskTerms = routeTerms(normalized);
      if (taskTerms.size < 2) return { terms: taskTerms, docs: [] };
      var n = records.length, out = [];
      records.forEach(function (r, i) {
        var shared = [], score = 0;
        taskTerms.forEach(function (term) {
          if (recTerms[i].has(term)) { shared.push(term); score += Math.log(n / df.get(term)); }
        });
        if (shared.length >= 2) out.push({ r: r, score: score, shared: shared.sort() });
      });
      out.sort(function (a, b) { return b.score - a.score || (a.r.Rel < b.r.Rel ? 1 : a.r.Rel > b.r.Rel ? -1 : 0); });
      return { terms: taskTerms, docs: out.slice(0, 5) };
    }
    function render() {
      if (!records) return;
      var res = match(input.value), hits = new Set();
      res.docs.forEach(function (d) { d.shared.forEach(function (t) { hits.add(t); }); });
      termsBox.textContent = "";
      termsBox.appendChild(document.createTextNode(data.ui.terms + " "));
      if (!res.terms.size) termsBox.appendChild(document.createTextNode("—"));
      res.terms.forEach(function (t) { termsBox.appendChild(el("span", { class: "term-pill" + (hits.has(t) ? " hit" : "") }, t)); });
      results.textContent = "";
      if (!res.docs.length) {
        results.appendChild(el("li", { class: "empty" }, res.terms.size < 2 ? data.ui.needTwo : data.ui.noMatch));
        return;
      }
      var top = res.docs[0].score || 1;
      res.docs.forEach(function (d, i) {
        var li = el("li", { class: "result" });
        li.style.animationDelay = (i * 45) + "ms";
        li.appendChild(el("span", { class: "rank" }, String(i + 1)));
        var mid = el("div");
        mid.appendChild(el("a", { href: "https://github.com/m16khb-org/issueops/blob/main/" + d.r.Rel }, d.r.Title));
        mid.appendChild(el("div", { class: "rel" }, d.r.Rel));
        var pills = el("div");
        d.shared.forEach(function (t) { pills.appendChild(el("span", { class: "term-pill hit" }, t)); });
        mid.appendChild(pills);
        li.appendChild(mid);
        var sc = el("div", { class: "score" }, data.ui.score + " " + d.score.toFixed(2));
        var barEl = el("i"); barEl.style.setProperty("--w", Math.max(8, Math.round(d.score / top * 100)) + "%");
        sc.appendChild(barEl); li.appendChild(sc);
        results.appendChild(li);
      });
    }
    var debounce = 0;
    input.addEventListener("input", function () { clearTimeout(debounce); debounce = setTimeout(render, 120); });
    $$(".presets button", root).forEach(function (b) {
      b.addEventListener("click", function () { input.value = b.textContent; render(); input.focus(); });
    });
    fetch(root.dataset.records).then(function (r) { return r.json(); }).then(function (list) {
      index(list);
      $(".count", root).textContent = String(list.length);
      render();
    }).catch(function () { results.textContent = ""; results.appendChild(el("li", { class: "empty" }, data.ui.loadFail)); });
  })();

  /* ---------- Gate simulator ---------- */
  (function gates() {
    var root = $("#gate-sim");
    if (!root) return;
    var g = data.gate, state = { verdict: "none", published: false, diff: 1 };
    var log = $(".sim-log", root);
    var verdictEl = $("[data-k=verdict]", root), diffEl = $("[data-k=diff]", root), prEl = $("[data-k=pr]", root);
    function badge(node, text, cls) { node.textContent = ""; node.appendChild(el("span", { class: "badge " + cls }, text)); }
    function paint() {
      badge(verdictEl, g.verdict[state.verdict], state.verdict === "fresh" ? "open" : state.verdict === "stale" ? "review" : "");
      badge(diffEl, g.diffLabel + " #" + state.diff, "");
      badge(prEl, state.published ? g.prPublished : g.prNone, state.published ? "merged" : "");
      $$("[data-act]", root).forEach(function (b) { b.disabled = state.published && b.dataset.act !== "reset"; });
    }
    function write(kind, tag, text) {
      var li = el("li", { class: kind }); li.appendChild(el("b", null, tag)); li.appendChild(el("span", null, text));
      log.appendChild(li); log.scrollTop = log.scrollHeight;
    }
    var acts = {
      verdict: function () { state.verdict = "fresh"; write("ok", "docs", g.log.verdict.replace("{n}", state.diff)); },
      edit: function () {
        state.diff += 1;
        if (state.verdict === "fresh") { state.verdict = "stale"; write("warn", "stale", g.log.stale); }
        else write("info", "diff", g.log.edit.replace("{n}", state.diff));
      },
      publish: function () {
        if (state.verdict === "fresh") { state.published = true; write("ok", "pr", g.log.published); return; }
        write("no", "refused", state.verdict === "stale" ? g.log.refusedStale : g.log.refusedNone);
        root.classList.remove("flash"); void root.offsetWidth; root.classList.add("flash");
      },
      reset: function () { state = { verdict: "none", published: false, diff: 1 }; log.textContent = ""; write("info", "start", g.log.start); }
    };
    $$("[data-act]", root).forEach(function (b) { b.addEventListener("click", function () { acts[b.dataset.act](); paint(); }); });
    acts.reset(); paint();
  })();

  /* ---------- Host tabs ---------- */
  (function hosts() {
    var root = $("#hosts-panel");
    if (!root) return;
    var tabs = $$(".host-tabs .tab", root), dl = $("dl", root), flowHost = $(".flow .host", root);
    function show(i) {
      tabs.forEach(function (t, j) { t.setAttribute("aria-selected", j === i ? "true" : "false"); t.tabIndex = j === i ? 0 : -1; });
      var h = data.hosts[i];
      dl.textContent = "";
      h.rows.concat(data.sharedRows).forEach(function (r) { dl.appendChild(el("dt", null, r[0])); dl.appendChild(el("dd", null, r[1])); });
      flowHost.textContent = h.name;
    }
    tabs.forEach(function (t, i) { t.addEventListener("click", function () { show(i); }); });
    rove(tabs, show, false);
    show(0);
  })();

  /* ---------- Copy buttons ---------- */
  $$(".copy-btn").forEach(function (b) {
    b.addEventListener("click", function () {
      var text = $("pre", b.parentNode).innerText.split("\n").map(function (l) { return l.replace(/^\$ /, ""); })
        .filter(function (l) { return l.trim() && l.trim().charAt(0) !== "#"; }).map(function (l) { return l.replace(/\s+#.*$/, ""); }).join("\n");
      navigator.clipboard.writeText(text).then(function () {
        b.textContent = data.ui.copied;
        setTimeout(function () { b.textContent = data.ui.copy; }, 1600);
      });
    });
  });

  /* ---------- Branch rail + section nodes ---------- */
  (function rail() {
    var main = $(".main"), fill = $(".rail i"), secs = $$(".sec");
    if (!main || !fill) return;
    var pending = false;
    function update() {
      pending = false;
      var r = main.getBoundingClientRect(), mid = window.innerHeight * 0.55;
      var p = Math.max(0, Math.min(1, (mid - r.top) / r.height));
      fill.style.setProperty("--p", p.toFixed(4));
      secs.forEach(function (s) { s.classList.toggle("on", s.getBoundingClientRect().top + 130 < mid); });
    }
    window.addEventListener("scroll", function () { if (!pending) { pending = true; requestAnimationFrame(update); } }, { passive: true });
    window.addEventListener("resize", update);
    update();
  })();

  /* ---------- Reveal ---------- */
  if (!reduce && "IntersectionObserver" in window) {
    document.documentElement.classList.add("js");
    var io = new IntersectionObserver(function (es) {
      es.forEach(function (e) { if (e.isIntersecting) { e.target.classList.add("in"); io.unobserve(e.target); } });
    }, { rootMargin: "0px 0px -8% 0px" });
    $$(".reveal").forEach(function (n) { io.observe(n); });
  }
})();
