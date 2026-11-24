/* Time and action calendar — the interface.
   All the scheduling lives in the binary; this page shows it and records what
   actually happened. Every request is same-origin and nothing leaves the
   machine. */
(function () {
  "use strict";

  /* ------------------------------------------------------------- the theme */

  var root = document.documentElement;

  function currentTheme() {
    var set = root.getAttribute("data-theme");
    if (set) { return set; }
    return (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) ? "dark" : "light";
  }

  var themer = document.querySelector("[data-themer]");
  function paintThemer() {
    if (!themer) { return; }
    var dark = currentTheme() === "dark";
    themer.textContent = dark ? "☀ Light" : "☾ Dark";
    themer.setAttribute("aria-label", dark ? "Switch to light theme" : "Switch to dark theme");
  }
  if (themer) {
    paintThemer();
    themer.addEventListener("click", function () {
      var next = currentTheme() === "dark" ? "light" : "dark";
      root.setAttribute("data-theme", next);
      try { localStorage.setItem("omsme-theme", next); } catch (e) { /* storage may be blocked */ }
      paintThemer();
    });
  }

  /* --------------------------------------------------------------- the API */

  function api(path, options) {
    return fetch(path, options).then(function (res) {
      return res.text().then(function (body) {
        var parsed = null;
        if (body) {
          try { parsed = JSON.parse(body); } catch (e) { parsed = null; }
        }
        if (!res.ok) {
          var message = (parsed && parsed.error) ? parsed.error : ("the tool replied with status " + res.status);
          var err = new Error(message);
          err.problems = (parsed && parsed.problems) || [message];
          throw err;
        }
        return parsed;
      });
    });
  }

  function send(method, path, body) {
    return api(path, {
      method: method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
  }

  /* ------------------------------------------------------------- the state */

  var state = null;      // the whole payload from /api/state
  var openOrderID = "";  // the order whose plan is on screen
  var editingID = "";    // the order the form is editing, if any
  var boardFilter = "all";
  var chainDraft = null; // the chain being edited, and the name it had
  var chainOriginalName = "";

  /* ---------------------------------------------------------- the plumbing */

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) { node.className = className; }
    if (text !== undefined && text !== null) { node.textContent = String(text); }
    return node;
  }

  function byID(id) { return document.getElementById(id); }

  function clear(node) {
    while (node.firstChild) { node.removeChild(node.firstChild); }
  }

  var DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  var MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

  /* nice turns an ISO date into "Tue 30 Jun 2026" without depending on the
     browser's locale, which would otherwise reorder the day and the month. */
  function nice(iso) {
    if (!iso) { return "—"; }
    var parts = String(iso).split("-");
    if (parts.length !== 3) { return String(iso); }
    var y = Number(parts[0]), m = Number(parts[1]), d = Number(parts[2]);
    var at = new Date(Date.UTC(y, m - 1, d));
    return DAYS[at.getUTCDay()] + " " + d + " " + MONTHS[m - 1] + " " + y;
  }

  function shortDate(iso) {
    if (!iso) { return "—"; }
    var parts = String(iso).split("-");
    if (parts.length !== 3) { return String(iso); }
    return Number(parts[2]) + " " + MONTHS[Number(parts[1]) - 1] + " " + parts[0].slice(2);
  }

  function days(n) {
    var v = Number(n) || 0;
    return (v === 1 || v === -1) ? (v + " day") : (v + " days");
  }

  function statusCell(status) {
    var span = el("span", "st st-" + status);
    span.appendChild(document.createTextNode(labelFor(status)));
    return span;
  }

  function labelFor(status) {
    var list = (state && state.statuses) || [];
    for (var i = 0; i < list.length; i++) {
      if (list[i].value === status) { return list[i].label; }
    }
    return status;
  }

  function basisLabel(basis) {
    var list = (state && state.bases) || [];
    for (var i = 0; i < list.length; i++) {
      if (list[i].value === basis) { return list[i].label; }
    }
    return basis;
  }

  function say(id, message) {
    var node = byID(id);
    if (node) { node.textContent = message || ""; }
  }

  function showError(id, err) {
    var node = byID(id);
    if (!node) { return; }
    clear(node);
    if (!err) { node.hidden = true; return; }
    var problems = err.problems || [err.message || String(err)];
    if (problems.length === 1) {
      node.textContent = problems[0];
    } else {
      var list = el("ul", "warn-list");
      problems.forEach(function (line) { list.appendChild(el("li", null, line)); });
      node.appendChild(list);
    }
    node.hidden = false;
  }

  function planFor(id) {
    var list = (state && state.plans) || [];
    for (var i = 0; i < list.length; i++) {
      if (list[i].orderId === id) { return list[i]; }
    }
    return null;
  }

  function orderFor(id) {
    var list = (state && state.orders) || [];
    for (var i = 0; i < list.length; i++) {
      if (list[i].id === id) { return list[i]; }
    }
    return null;
  }

  function chainNamed(name) {
    var list = (state && state.chains) || [];
    for (var i = 0; i < list.length; i++) {
      if (list[i].name === name) { return list[i]; }
    }
    return null;
  }

  /* ------------------------------------------------------------- the board */

  function renderTiles() {
    var host = byID("tiles");
    clear(host);
    var b = state.board || {};
    var tiles = [
      { n: b.late || 0, l: "Late", k: "late" },
      { n: b.atRisk || 0, l: "At risk", k: "risk" },
      { n: b.onTrack || 0, l: "On track", k: "ok" },
      { n: b.complete || 0, l: "Shipped", k: "" },
      { n: (b.piecesAtRisk || 0).toLocaleString("en-GB"), l: "Pieces at risk", k: "" }
    ];
    if (b.unusable) { tiles.push({ n: b.unusable, l: "Cannot be planned", k: "late" }); }
    tiles.forEach(function (t) {
      var tile = el("div", "tile" + (t.k ? " " + t.k : ""));
      tile.appendChild(el("div", "n", t.n));
      tile.appendChild(el("div", "l", t.l));
      host.appendChild(tile);
    });
  }

  function renderFilter() {
    var host = byID("board-filter");
    clear(host);
    var b = state.board || {};
    var options = [
      { k: "all", l: "All", n: (b.rows || []).length },
      { k: "late", l: "Late", n: b.late || 0 },
      { k: "at-risk", l: "At risk", n: b.atRisk || 0 },
      { k: "on-track", l: "On track", n: b.onTrack || 0 },
      { k: "done", l: "Shipped", n: b.complete || 0 }
    ];
    options.forEach(function (o) {
      var button = el("button", null, o.l + " (" + o.n + ")");
      button.type = "button";
      button.setAttribute("aria-pressed", boardFilter === o.k ? "true" : "false");
      button.addEventListener("click", function () {
        boardFilter = o.k;
        renderFilter();
        renderBoard();
      });
      host.appendChild(button);
    });
  }

  function renderBoard() {
    var body = byID("board-body");
    clear(body);

    var rows = ((state.board && state.board.rows) || []).filter(function (r) {
      if (boardFilter === "all") { return true; }
      if (boardFilter === "done") { return r.complete; }
      return !r.complete && r.status === boardFilter;
    });

    if (rows.length === 0) {
      var empty = el("tr");
      var cell = el("td", "empty", (state.orders || []).length === 0
        ? "No orders yet. Add one below: a buyer, a purchase-order date and a ship date are enough."
        : "No orders in this group.");
      cell.colSpan = 8;
      empty.appendChild(cell);
      body.appendChild(empty);
      return;
    }

    rows.forEach(function (r) {
      var tr = el("tr", "row-pick");

      var who = el("td");
      who.appendChild(document.createTextNode(r.label));
      var sub = el("span", "sub2");
      var bits = [];
      if (r.order.quantity) { bits.push(r.order.quantity.toLocaleString("en-GB") + " pcs"); }
      if (r.order.chain) { bits.push(r.order.chain); }
      sub.textContent = bits.join(" · ");
      who.appendChild(sub);
      tr.appendChild(who);

      tr.appendChild(el("td", "num", shortDate(r.shipDate)));
      tr.appendChild(el("td", "num", r.daysToShip));

      var st = el("td");
      st.appendChild(statusCell(r.complete ? "done" : r.status));
      if (r.error) {
        var warn = el("span", "sub2", "cannot be planned");
        st.appendChild(warn);
      } else if (r.overdue) {
        st.appendChild(el("span", "sub2", r.overdue + " past its date"));
      }
      tr.appendChild(st);

      var slip = el("td", "num", r.slipDays ? r.slipDays : "—");
      if (!r.feasible && r.shortfallDays) {
        slip.textContent = r.shortfallDays;
        slip.appendChild(el("span", "sub2", "short by"));
      }
      tr.appendChild(slip);

      tr.appendChild(el("td", "num", r.error ? "—" : r.minFloatDays));

      var chase = el("td");
      if (r.chaseName) {
        chase.appendChild(document.createTextNode(r.chaseName));
        chase.appendChild(el("span", "sub2", (r.chaseOwner ? r.chaseOwner + " · " : "") + "due " + shortDate(r.chaseDue)));
      } else {
        chase.textContent = "—";
      }
      tr.appendChild(chase);

      tr.appendChild(el("td", "num", r.doneCount + "/" + r.totalCount));

      tr.addEventListener("click", function () { openOrder(r.order.id); });
      body.appendChild(tr);
    });
  }

  /* -------------------------------------------------------- the open order */

  function openOrder(id) {
    openOrderID = id;
    renderOrder();
    var card = byID("order-card");
    if (card) { card.scrollIntoView({ block: "start" }); }
  }

  function renderOrder() {
    var card = byID("order-card");
    if (!openOrderID) { card.hidden = true; return; }

    var order = orderFor(openOrderID);
    var entry = planFor(openOrderID);
    if (!order) { card.hidden = true; openOrderID = ""; return; }
    card.hidden = false;

    byID("order-title").textContent = labelOf(order);
    byID("x-plan").href = "/api/orders/plan.csv?id=" + encodeURIComponent(order.id);

    if (!entry || entry.error) {
      showError("order-error", { message: (entry && entry.error) || "this order cannot be planned" });
      byID("r-value").textContent = "—";
      byID("r-sub").textContent = "";
      clear(byID("plan-body"));
      clear(byID("order-warnings"));
      return;
    }
    showError("order-error", null);

    var plan = entry.plan;

    var key = "Projected on board";
    var value = nice(plan.projectedShipDate);
    var sub = plan.verdict;
    if (!plan.feasible) {
      key = "Short by";
      value = days(plan.shortfallDays);
    } else if (plan.complete) {
      key = "Went on board";
    } else if (plan.slipDays) {
      key = "Late by";
      value = days(plan.slipDays);
    }
    byID("r-key").textContent = key;
    byID("r-value").textContent = value;
    byID("r-sub").textContent = sub;

    byID("f-ship").textContent = nice(plan.shipDate);
    byID("f-po").textContent = plan.poDate ? nice(plan.poDate) : "not recorded";
    byID("f-latest").textContent = nice(plan.latestPoDate);
    byID("f-earliest").textContent = nice(plan.earliestShipDate);
    byID("f-working").textContent = plan.workingDays;
    byID("f-closed").textContent = plan.closedDays;
    byID("f-slack").textContent = plan.poDateKnown ? days(plan.minFloatDays) : "not measurable";

    var warnings = byID("order-warnings");
    clear(warnings);
    (plan.warnings || []).forEach(function (line) {
      warnings.appendChild(el("div", "note", line));
    });

    renderPlanTable(order, plan);
    byID("plan-note").textContent = "Critical path: " + (plan.criticalPath || []).join(" → ");
  }

  function labelOf(order) {
    var bits = [];
    if (order.buyer) { bits.push(order.buyer); }
    if (order.style) { bits.push(order.style); }
    if (order.poNumber) { bits.push("PO " + order.poNumber); }
    if (bits.length === 0) { return "Untitled order"; }
    return bits.join(" · ");
  }

  function renderPlanTable(order, plan) {
    var body = byID("plan-body");
    clear(body);

    (plan.milestones || []).forEach(function (m) {
      var tr = el("tr");
      if (m.critical) { tr.className = "is-critical"; }
      if (m.overdue) { tr.className += " is-overdue"; }

      var name = el("td");
      name.appendChild(document.createTextNode(m.name));
      if (m.critical) { name.appendChild(el("span", "crit", "critical")); }
      var subBits = [m.code];
      if (m.drivenBy) { subBits.push("waiting on " + m.drivenBy); }
      name.appendChild(el("span", "sub2", subBits.join(" · ")));
      tr.appendChild(name);

      tr.appendChild(el("td", null, m.owner || "—"));

      var lead = el("td", "num", m.days);
      lead.appendChild(el("span", "sub2", basisLabel(m.basis)));
      tr.appendChild(lead);

      var planned = el("td", "num plan", shortDate(m.plannedFinish));
      planned.title = nice(m.plannedFinish);
      planned.appendChild(el("span", "sub2", "from " + shortDate(m.plannedStart)));
      tr.appendChild(planned);

      var slack = el("td", "num", m.floatDays);
      slack.appendChild(el("span", "sub2", basisLabel(m.basis).split(" ")[0]));
      tr.appendChild(slack);

      var st = el("td");
      st.appendChild(statusCell(m.status));
      if (m.done && m.lateDays) {
        st.appendChild(el("span", "sub2", days(m.lateDays) + " past its date"));
      } else if (!m.done && m.slipDays) {
        st.appendChild(el("span", "sub2", days(m.slipDays) + " past its date"));
      } else if (m.overdue) {
        st.appendChild(el("span", "sub2", "date already gone"));
      }
      tr.appendChild(st);

      var projected = el("td", "num", m.done ? "—" : shortDate(m.projectedFinish));
      if (!m.done) { projected.title = nice(m.projectedFinish); }
      tr.appendChild(projected);

      var act = el("td", "act");
      var input = el("input");
      input.type = "date";
      input.value = m.actual || "";
      input.setAttribute("aria-label", "Date " + m.name + " actually finished");
      act.appendChild(input);

      var save = el("button", "mini", "Save");
      save.type = "button";
      save.addEventListener("click", function () {
        if (!input.value) {
          say("order-status", "Pick the date " + m.name + " actually finished on.");
          return;
        }
        say("order-status", "Saving …");
        send("POST", "/api/orders/actuals?id=" + encodeURIComponent(order.id), { code: m.code, on: input.value })
          .then(function () { return refresh(); })
          .then(function () { say("order-status", m.name + " recorded as finished " + nice(input.value) + "."); })
          .catch(function (err) { say("order-status", err.message); });
      });
      act.appendChild(save);

      if (m.done) {
        var cleared = el("button", "mini danger", "Clear");
        cleared.type = "button";
        cleared.addEventListener("click", function () {
          say("order-status", "Clearing …");
          api("/api/orders/actuals?id=" + encodeURIComponent(order.id) + "&code=" + encodeURIComponent(m.code), { method: "DELETE" })
            .then(function () { return refresh(); })
            .then(function () { say("order-status", m.name + " is no longer recorded as finished."); })
            .catch(function (err) { say("order-status", err.message); });
        });
        act.appendChild(cleared);
      }
      tr.appendChild(act);

      body.appendChild(tr);
    });
  }

  /* -------------------------------------------------------------- the form */

  function renderChainOptions() {
    var pick = byID("i-chain");
    var current = pick.value;
    clear(pick);
    (state.chains || []).forEach(function (ch) {
      var option = el("option", null, ch.name);
      option.value = ch.name;
      pick.appendChild(option);
    });
    if (current) { pick.value = current; }
  }

  function readForm() {
    return {
      buyer: byID("i-buyer").value,
      poNumber: byID("i-po").value,
      style: byID("i-style").value,
      product: byID("i-product").value,
      quantity: Number(byID("i-qty").value) || 0,
      chain: byID("i-chain").value,
      poDate: byID("i-podate").value,
      shipDate: byID("i-shipdate").value,
      note: byID("i-note").value
    };
  }

  function fillForm(order) {
    byID("i-buyer").value = order ? (order.buyer || "") : "";
    byID("i-po").value = order ? (order.poNumber || "") : "";
    byID("i-style").value = order ? (order.style || "") : "";
    byID("i-product").value = order ? (order.product || "") : "";
    byID("i-qty").value = order && order.quantity ? order.quantity : "";
    byID("i-podate").value = order ? (order.poDate || "") : "";
    byID("i-shipdate").value = order ? (order.shipDate || "") : "";
    byID("i-note").value = order ? (order.note || "") : "";
    if (order && order.chain) { byID("i-chain").value = order.chain; }

    editingID = order ? order.id : "";
    byID("form-title").textContent = order ? "Edit this order" : "Add an order";
    byID("save-order").textContent = order ? "Save changes" : "Add order";
    byID("cancel-order").hidden = !order;
    showError("form-error", null);
    say("form-status", "");
  }

  byID("save-order").addEventListener("click", function () {
    var body = readForm();
    showError("form-error", null);
    say("form-status", "Saving …");

    var request = editingID
      ? send("PUT", "/api/orders?id=" + encodeURIComponent(editingID), body)
      : send("POST", "/api/orders", body);

    request.then(function (answer) {
      var id = (answer && answer.order && answer.order.id) || editingID;
      return refresh().then(function () {
        fillForm(null);
        say("form-status", "Saved.");
        if (id) { openOrder(id); }
      });
    }).catch(function (err) {
      say("form-status", "");
      showError("form-error", err);
    });
  });

  byID("cancel-order").addEventListener("click", function () { fillForm(null); });

  byID("edit-order").addEventListener("click", function () {
    var order = orderFor(openOrderID);
    if (!order) { return; }
    fillForm(order);
    byID("form-card").scrollIntoView({ block: "start" });
  });

  byID("close-order").addEventListener("click", function () {
    openOrderID = "";
    renderOrder();
  });

  byID("delete-order").addEventListener("click", function () {
    var order = orderFor(openOrderID);
    if (!order) { return; }
    if (!window.confirm("Remove " + labelOf(order) + " from the order book? The plan and everything recorded against it go with it.")) { return; }
    api("/api/orders?id=" + encodeURIComponent(order.id), { method: "DELETE" })
      .then(function () {
        openOrderID = "";
        return refresh();
      })
      .then(function () { say("board-note", "Order removed."); })
      .catch(function (err) { say("order-status", err.message); });
  });

  /* ---------------------------------------------------- the working calendar */

  function renderCalendar() {
    var host = byID("weekly-off");
    clear(host);
    var off = (state.calendar && state.calendar.weeklyOff) || [];

    (state.weekdays || []).forEach(function (day) {
      var on = off.indexOf(day.value) >= 0;
      var button = el("button", null, day.short);
      button.type = "button";
      button.setAttribute("aria-pressed", on ? "true" : "false");
      button.setAttribute("aria-label", day.label + (on ? ": closed" : ": working"));
      button.addEventListener("click", function () {
        var next = off.filter(function (v) { return v !== day.value; });
        if (!on) { next.push(day.value); }
        saveCalendar(next, (state.calendar && state.calendar.holidays) || []);
      });
      host.appendChild(button);
    });

    var body = byID("holiday-body");
    clear(body);
    var holidays = (state.calendar && state.calendar.holidays) || [];
    if (holidays.length === 0) {
      var empty = el("tr");
      var cell = el("td", "empty", "No holidays entered. Every day except the weekly off counts as a working day.");
      cell.colSpan = 3;
      empty.appendChild(cell);
      body.appendChild(empty);
    } else {
      holidays.forEach(function (h) {
        var tr = el("tr");
        tr.appendChild(el("td", "num", shortDate(h.on)));
        tr.appendChild(el("td", null, h.name));
        var act = el("td", "act");
        var remove = el("button", "mini danger", "Remove");
        remove.type = "button";
        remove.addEventListener("click", function () {
          api("/api/holidays?on=" + encodeURIComponent(h.on), { method: "DELETE" })
            .then(function () { return refresh(); })
            .then(function () { say("cal-status", h.name + " removed from the holiday list."); })
            .catch(function (err) { showError("cal-error", err); });
        });
        act.appendChild(remove);
        tr.appendChild(act);
        body.appendChild(tr);
      });
    }

    byID("i-atrisk").value = (state.settings && state.settings.atRiskFloatDays) || "";
  }

  function saveCalendar(weeklyOff, holidays) {
    showError("cal-error", null);
    say("cal-status", "Saving …");
    send("PUT", "/api/calendar", { weeklyOff: weeklyOff, holidays: holidays })
      .then(function () { return refresh(); })
      .then(function () { say("cal-status", "Working calendar saved. Every plan has been worked out again."); })
      .catch(function (err) {
        say("cal-status", "");
        showError("cal-error", err);
      });
  }

  byID("add-holiday").addEventListener("click", function () {
    var on = byID("i-hol-date").value;
    var name = byID("i-hol-name").value;
    if (!on) {
      say("cal-status", "Pick the date of the holiday.");
      return;
    }
    showError("cal-error", null);
    say("cal-status", "Saving …");
    send("POST", "/api/holidays", { on: on, name: name })
      .then(function () { return refresh(); })
      .then(function () {
        byID("i-hol-date").value = "";
        byID("i-hol-name").value = "";
        say("cal-status", "Holiday added. Every plan has been worked out again.");
      })
      .catch(function (err) {
        say("cal-status", "");
        showError("cal-error", err);
      });
  });

  byID("save-settings").addEventListener("click", function () {
    showError("cal-error", null);
    send("PUT", "/api/settings", { atRiskFloatDays: Number(byID("i-atrisk").value) || 0 })
      .then(function () { return refresh(); })
      .then(function () { say("cal-status", "Threshold saved."); })
      .catch(function (err) { showError("cal-error", err); });
  });

  /* ------------------------------------------------------- the chain editor */

  function renderChainEditor() {
    var pick = byID("i-chain-pick");
    var wanted = chainDraft ? chainOriginalName : pick.value;
    clear(pick);
    (state.chains || []).forEach(function (ch) {
      var option = el("option", null, ch.name);
      option.value = ch.name;
      pick.appendChild(option);
    });
    if (wanted && chainNamed(wanted)) { pick.value = wanted; }

    if (!chainDraft || !chainNamed(chainOriginalName)) {
      loadChainDraft(pick.value);
      return;
    }
    paintChainDraft();
  }

  function loadChainDraft(name) {
    var ch = chainNamed(name) || (state.chains || [])[0];
    if (!ch) { chainDraft = null; return; }
    chainOriginalName = ch.name;
    chainDraft = JSON.parse(JSON.stringify(ch));
    if (!chainDraft.stages) { chainDraft.stages = []; }
    paintChainDraft();
  }

  function paintChainDraft() {
    byID("i-chain-name").value = chainDraft.name || "";

    var body = byID("chain-body");
    clear(body);

    chainDraft.stages.forEach(function (stage, index) {
      var tr = el("tr");

      tr.appendChild(textCell(stage, "code", "SEWING", 8));
      tr.appendChild(textCell(stage, "name", "Sewing", 14));
      tr.appendChild(textCell(stage, "owner", "Production", 12));

      var leadCell = el("td", "num");
      var lead = el("input");
      lead.type = "number";
      lead.min = "1";
      lead.step = "1";
      lead.value = stage.days;
      lead.setAttribute("aria-label", "Lead time for " + (stage.name || stage.code));
      lead.addEventListener("input", function () { stage.days = Number(lead.value) || 0; });
      leadCell.appendChild(lead);
      tr.appendChild(leadCell);

      var basisCell = el("td");
      var basis = el("select");
      basis.setAttribute("aria-label", "How " + (stage.name || stage.code) + " is counted");
      (state.bases || []).forEach(function (b) {
        var option = el("option", null, b.label);
        option.value = b.value;
        basis.appendChild(option);
      });
      basis.value = stage.basis;
      basis.addEventListener("change", function () { stage.basis = basis.value; });
      basisCell.appendChild(basis);
      tr.appendChild(basisCell);

      var afterCell = el("td");
      var after = el("input");
      after.type = "text";
      after.value = (stage.after || []).join(", ");
      after.placeholder = "CUTTING, TRIMS-IN";
      after.setAttribute("aria-label", "What " + (stage.name || stage.code) + " waits on");
      after.addEventListener("input", function () {
        stage.after = after.value.split(",").map(function (s) { return s.trim(); }).filter(function (s) { return s !== ""; });
      });
      afterCell.appendChild(after);
      tr.appendChild(afterCell);

      var act = el("td", "act");
      var remove = el("button", "mini danger", "Remove");
      remove.type = "button";
      remove.addEventListener("click", function () {
        chainDraft.stages.splice(index, 1);
        paintChainDraft();
      });
      act.appendChild(remove);
      tr.appendChild(act);

      body.appendChild(tr);
    });

    var shipped = (state.defaults && state.defaults.chain) || null;
    var note = byID("chain-indicative");
    clear(note);
    note.appendChild(el("strong", null, "These lead times are indicative. "));
    note.appendChild(document.createTextNode((state.notes && state.notes.indicative) || ""));
    if (shipped) {
      var totals = [];
      var working = 0, calendar = 0;
      shipped.stages.forEach(function (s) {
        if (s.basis === "calendar") { calendar += s.days; } else { working += s.days; }
      });
      totals.push("The shipped chain holds " + shipped.stages.length + " milestones, " + working + " working days and " + calendar + " calendar days of lead time in all.");
      note.appendChild(el("p", null, totals.join(" ")));
    }
  }

  function textCell(stage, field, placeholder, size) {
    var cell = el("td");
    var input = el("input");
    input.type = "text";
    input.value = stage[field] || "";
    input.placeholder = placeholder;
    input.size = size;
    input.setAttribute("aria-label", field + " for " + (stage.name || stage.code || "this milestone"));
    input.addEventListener("input", function () { stage[field] = input.value; });
    cell.appendChild(input);
    return cell;
  }

  byID("i-chain-pick").addEventListener("change", function () {
    loadChainDraft(byID("i-chain-pick").value);
    say("chain-status", "");
    showError("chain-error", null);
  });

  byID("i-chain-name").addEventListener("input", function () {
    if (chainDraft) { chainDraft.name = byID("i-chain-name").value; }
  });

  byID("add-stage").addEventListener("click", function () {
    if (!chainDraft) { return; }
    chainDraft.stages.push({ code: "", name: "", owner: "", days: 1, basis: "working", after: [], note: "" });
    paintChainDraft();
  });

  function saveChain(replacing, message) {
    if (!chainDraft) { return; }
    showError("chain-error", null);
    say("chain-status", "Saving …");
    var path = "/api/chains";
    if (replacing) { path += "?replacing=" + encodeURIComponent(replacing); }
    send("PUT", path, chainDraft)
      .then(function (answer) {
        chainOriginalName = (answer && answer.chain && answer.chain.name) || chainDraft.name;
        chainDraft = null;
        return refresh();
      })
      .then(function () { say("chain-status", message); })
      .catch(function (err) {
        say("chain-status", "");
        showError("chain-error", err);
      });
  }

  byID("save-chain").addEventListener("click", function () {
    saveChain(chainOriginalName, "Chain saved. Every order planned on it has been worked out again.");
  });

  byID("copy-chain").addEventListener("click", function () {
    if (!chainDraft) { return; }
    if (chainDraft.name === chainOriginalName) {
      say("chain-status", "Give the copy a different name first, so the original is left as it is.");
      return;
    }
    saveChain("", "Saved as a separate chain. Point an order at it from the order form.");
  });

  byID("reset-chain").addEventListener("click", function () {
    showError("chain-error", null);
    send("POST", "/api/chains/reset", {})
      .then(function (answer) {
        chainOriginalName = (answer && answer.chain && answer.chain.name) || "";
        chainDraft = null;
        return refresh();
      })
      .then(function () { say("chain-status", "The shipped chain is back as it came, with its indicative lead times."); })
      .catch(function (err) { showError("chain-error", err); });
  });

  byID("delete-chain").addEventListener("click", function () {
    var name = byID("i-chain-pick").value;
    if (!name) { return; }
    if (!window.confirm("Remove the chain " + name + "?")) { return; }
    showError("chain-error", null);
    api("/api/chains?name=" + encodeURIComponent(name), { method: "DELETE" })
      .then(function () {
        chainDraft = null;
        chainOriginalName = "";
        return refresh();
      })
      .then(function () { say("chain-status", "Chain removed."); })
      .catch(function (err) { showError("chain-error", err); });
  });

  /* ------------------------------------------------------------ the refresh */

  function render() {
    renderTiles();
    renderFilter();
    renderBoard();
    renderChainOptions();
    renderCalendar();
    renderChainEditor();
    renderOrder();

    var foot = byID("foot-file");
    if (foot && state.notes && state.notes.storage) { foot.textContent = state.notes.storage; }
    say("board-note", "As at " + nice(state.today) + ". Worst first: late orders, then the ones at risk. Click an order to open its plan.");
  }

  function refresh() {
    return api("/api/state").then(function (payload) {
      state = payload;
      render();
      return payload;
    });
  }

  refresh().catch(function (err) {
    say("board-note", "Could not read the order book: " + err.message);
  });
})();
