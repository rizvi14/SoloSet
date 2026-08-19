// SoloSet status page: subscribes to the server's SSE stream and renders the
// current phase. Falls back to polling /api/status if the stream drops.

const el = {
  indicator: document.getElementById("indicator"),
  message: document.getElementById("status-message"),
  detail: document.getElementById("status-detail"),
  actions: document.getElementById("actions"),
  credentials: document.getElementById("credentials"),
  credUrl: document.getElementById("cred-url"),
  credUser: document.getElementById("cred-user"),
  credPass: document.getElementById("cred-pass"),
  logsToggle: document.getElementById("logs-toggle"),
  logsBody: document.getElementById("logs-body"),
  conn: document.getElementById("conn"),
};

// Per-phase action buttons. POST actions call server endpoints added in later
// milestones (Docker detection/install, Superset start/stop); a phase with no
// entry here simply shows no buttons.
function actionsFor(status) {
  switch (status.phase) {
    case "ready":
      return [
        { label: "Open Superset", href: status.supersetUrl, kind: "primary" },
        { label: "Stop Superset", post: "/api/stop", kind: "secondary" },
        { label: "Quit SoloSet", post: "/api/quit", kind: "secondary" },
      ];
    case "stopped":
      return [
        { label: "Start Superset", post: "/api/start", kind: "primary" },
        { label: "Quit SoloSet", post: "/api/quit", kind: "secondary" },
      ];
    case "docker_missing":
      return [
        { label: "Install Docker Desktop", post: "/api/install-docker", kind: "primary" },
        { label: "I installed it — retry", post: "/api/retry", kind: "secondary" },
      ];
    case "docker_stopped":
      return [{ label: "Docker is running — retry", post: "/api/retry", kind: "primary" }];
    case "error":
      return [{ label: "Retry", post: "/api/retry", kind: "primary" }];
    default:
      return [];
  }
}

function renderActions(status) {
  const defs = actionsFor(status);
  el.actions.replaceChildren();
  if (defs.length === 0) {
    el.actions.hidden = true;
    return;
  }
  el.actions.hidden = false;
  for (const def of defs) {
    let node;
    if (def.href) {
      node = document.createElement("a");
      node.href = def.href;
      node.target = "_blank";
      node.rel = "noopener";
    } else {
      node = document.createElement("button");
      node.type = "button";
      node.addEventListener("click", () => postAction(def.post, node));
    }
    node.className = "btn btn-" + def.kind;
    node.textContent = def.label;
    el.actions.appendChild(node);
  }
}

async function postAction(path, node) {
  node.disabled = true;
  try {
    await fetch(path, { method: "POST" });
  } catch (_) {
    node.disabled = false;
  }
  // The resulting phase change arrives over SSE and re-renders the actions.
}

function renderCredentials(status) {
  if (status.phase === "ready" && status.username) {
    el.credUrl.textContent = status.supersetUrl || "";
    el.credUser.textContent = status.username;
    el.credPass.textContent = status.password;
    el.credentials.hidden = false;
  } else {
    el.credentials.hidden = true;
  }
}

function renderLogs(status) {
  const lines = status.logs || [];
  const atBottom =
    el.logsBody.scrollTop + el.logsBody.clientHeight >= el.logsBody.scrollHeight - 4;
  el.logsBody.textContent = lines.join("\n");
  if (atBottom) el.logsBody.scrollTop = el.logsBody.scrollHeight;
}

function render(status) {
  el.indicator.dataset.phase = status.phase;
  el.message.textContent = status.message || "";
  el.detail.textContent = status.detail || "";
  document.title = status.phase === "ready" ? "SoloSet — ready" : "SoloSet";
  renderActions(status);
  renderCredentials(status);
  renderLogs(status);
}

function setConn(state, label) {
  el.conn.dataset.state = state;
  el.conn.textContent = label;
}

// Logs are collapsed by default.
el.logsToggle.addEventListener("click", () => {
  const expanded = el.logsToggle.getAttribute("aria-expanded") === "true";
  el.logsToggle.setAttribute("aria-expanded", String(!expanded));
  el.logsBody.hidden = expanded;
  if (!expanded) el.logsBody.scrollTop = el.logsBody.scrollHeight;
});

function connect() {
  const source = new EventSource("/api/events");
  source.onopen = () => setConn("open", "live");
  source.onmessage = (e) => {
    try {
      render(JSON.parse(e.data));
    } catch (_) {
      /* ignore malformed frame */
    }
  };
  source.onerror = () => {
    setConn("connecting", "reconnecting…");
    // EventSource auto-reconnects; nothing else to do.
  };
}

// Initial paint from a plain fetch so the page is populated even before the
// stream opens, then switch to live updates.
fetch("/api/status")
  .then((r) => r.json())
  .then(render)
  .catch(() => {});
connect();
