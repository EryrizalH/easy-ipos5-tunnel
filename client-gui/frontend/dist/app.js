const stateEls = {
  connection: document.getElementById("connectionState"),
  serviceState: document.getElementById("serviceState"),
  serviceName: document.getElementById("serviceName"),
  lastChecked: document.getElementById("lastChecked"),
  serverPublicIP: document.getElementById("serverPublicIP"),
  serverHost: document.getElementById("serverHost"),
  dashboardStatus: document.getElementById("dashboardStatus"),
  controlStatus: document.getElementById("controlStatus"),
  message: document.getElementById("message"),
  configPath: document.getElementById("configPath"),
};

function goCall(method, ...args) {
  const app = window?.go?.main?.App;
  if (!app || typeof app[method] !== "function") {
    throw new Error("Wails backend method unavailable: " + method);
  }
  return app[method](...args);
}

function setMessage(text, type = "") {
  stateEls.message.textContent = text;
  stateEls.message.className = `message ${type}`.trim();
}

function decorateStatus(el, ok) {
  el.classList.remove("ok", "bad", "warn");
  el.classList.add(ok ? "ok" : "bad");
}

function renderStatus(snapshot) {
  stateEls.connection.textContent = snapshot.connectionState || "-";
  decorateStatus(stateEls.connection, snapshot.connectionState === "Connected");

  stateEls.serviceState.textContent = snapshot.serviceState || "-";
  decorateStatus(stateEls.serviceState, snapshot.serviceState === "running");

  stateEls.serviceName.textContent = snapshot.serviceName || "EasyRatholeClient";
  stateEls.lastChecked.textContent = `Last checked: ${snapshot.lastCheckedAt || "-"}`;
  stateEls.serverPublicIP.textContent = snapshot.serverPublicIP || "-";
  stateEls.serverHost.textContent = `Host: ${snapshot.serverHost || "-"}`;
  stateEls.dashboardStatus.textContent = `Dashboard: ${snapshot.dashboardReachable ? "Up" : "Down"}`;
  stateEls.controlStatus.textContent = `Control Port: ${snapshot.controlPortReachable ? "Up" : "Down"} (${snapshot.serverControlPort || "-"})`;

  if (snapshot.lastError) {
    setMessage(snapshot.lastError, "warn");
  }
}

async function refreshStatus() {
  try {
    const snapshot = await goCall("GetStatus");
    renderStatus(snapshot);
  } catch (err) {
    setMessage(err.message || String(err), "bad");
  }
}

async function runAction(method, okMessage) {
  try {
    const result = await goCall(method);
    if (result?.ok) {
      setMessage(okMessage, "ok");
      await refreshStatus();
      return;
    }
    setMessage(result?.message || "Action failed", "bad");
  } catch (err) {
    setMessage(err.message || String(err), "bad");
  }
}

async function bootstrap() {
  try {
    const config = await goCall("GetConfig");
    if (config?.configPath) {
      stateEls.configPath.value = config.configPath;
    }
  } catch (err) {
    setMessage(err.message || String(err), "warn");
  }

  document.getElementById("btnRefresh").addEventListener("click", refreshStatus);
  document.getElementById("btnStart").addEventListener("click", () => runAction("StartService", "Service started."));
  document.getElementById("btnStop").addEventListener("click", () => runAction("StopService", "Service stopped."));
  document.getElementById("btnRestart").addEventListener("click", () => runAction("RestartService", "Service restarted."));
  document.getElementById("btnEnableAutostart").addEventListener("click", () =>
    runAction("EnableAutoStart", "Auto-start enabled."),
  );
  document.getElementById("btnDisableAutostart").addEventListener("click", () =>
    runAction("DisableAutoStart", "Auto-start disabled."),
  );
  document.getElementById("btnSaveConfig").addEventListener("click", async () => {
    const path = stateEls.configPath.value.trim();
    try {
      const result = await goCall("SetConfigPath", path);
      if (result?.ok) {
        setMessage("Config path saved.", "ok");
        await refreshStatus();
      } else {
        setMessage(result?.message || "Failed to save config path.", "bad");
      }
    } catch (err) {
      setMessage(err.message || String(err), "bad");
    }
  });

  await refreshStatus();
  setInterval(refreshStatus, 5000);
}

bootstrap();

