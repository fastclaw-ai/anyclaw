/**
 * AnyClaw Browser Bridge - Service Worker
 *
 * Connects to the anyclaw daemon via WebSocket, receives commands,
 * executes them in Chrome (navigate, evaluate JS, get cookies, etc.),
 * and sends results back.
 *
 * Compatible with opencli browser extension protocol.
 */

const DAEMON_PORT = 19825;
const DAEMON_WS_URL = `ws://127.0.0.1:${DAEMON_PORT}/ws`;
const IDLE_TIMEOUT_MS = 30000;

let ws = null;
let connected = false;
let reconnectAttempts = 0;
const MAX_EAGER_ATTEMPTS = 6;

// Automation window (isolated from user browsing)
let autoWindowId = null;
let activeTabId = null;
let idleTimer = null;

// ─── WebSocket Connection ────────────────────────────────────

function connect() {
  if (ws && ws.readyState <= 1) return;

  try {
    ws = new WebSocket(DAEMON_WS_URL);
  } catch {
    scheduleReconnect();
    return;
  }

  ws.onopen = () => {
    connected = true;
    reconnectAttempts = 0;
    // Send hello
    ws.send(JSON.stringify({
      type: 'hello',
      version: chrome.runtime.getManifest().version,
    }));
  };

  ws.onmessage = async (event) => {
    let cmd;
    try {
      cmd = JSON.parse(event.data);
    } catch {
      return;
    }
    resetIdleTimer();
    const result = await handleCommand(cmd);
    ws.send(JSON.stringify(result));
  };

  ws.onclose = () => {
    connected = false;
    ws = null;
    scheduleReconnect();
  };

  ws.onerror = () => {
    connected = false;
  };
}

function scheduleReconnect() {
  if (reconnectAttempts >= MAX_EAGER_ATTEMPTS) return;
  const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 60000);
  reconnectAttempts++;
  setTimeout(connect, delay);
}

// Keep-alive alarm for reconnection
chrome.alarms.create('keepalive', { periodInMinutes: 1 });
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === 'keepalive' && !connected) {
    reconnectAttempts = 0;
    connect();
  }
});

// ─── Command Handler ─────────────────────────────────────────

async function handleCommand(cmd) {
  const { id, action } = cmd;
  try {
    let data;
    switch (action) {
      case 'navigate':
        data = await cmdNavigate(cmd);
        break;
      case 'exec':
        data = await cmdExec(cmd);
        break;
      case 'tabs':
        data = await cmdTabs(cmd);
        break;
      case 'cookies':
        data = await cmdCookies(cmd);
        break;
      case 'screenshot':
        data = await cmdScreenshot(cmd);
        break;
      case 'close-window':
        await closeAutoWindow();
        data = { closed: true };
        break;
      case 'sessions':
        data = { windowId: autoWindowId, tabId: activeTabId };
        break;
      default:
        return { id, ok: false, error: `Unknown action: ${action}` };
    }
    return { id, ok: true, data };
  } catch (err) {
    return { id, ok: false, error: err.message || String(err) };
  }
}

// ─── Automation Window ───────────────────────────────────────

async function ensureAutoWindow() {
  if (autoWindowId) {
    try {
      await chrome.windows.get(autoWindowId);
      return;
    } catch {
      autoWindowId = null;
      activeTabId = null;
    }
  }

  const win = await chrome.windows.create({
    url: 'about:blank',
    type: 'normal',
    width: 1280,
    height: 900,
    focused: false,
  });
  autoWindowId = win.id;
  activeTabId = win.tabs[0]?.id || null;
}

async function closeAutoWindow() {
  if (autoWindowId) {
    try {
      await chrome.windows.remove(autoWindowId);
    } catch {}
    autoWindowId = null;
    activeTabId = null;
  }
}

function resetIdleTimer() {
  if (idleTimer) clearTimeout(idleTimer);
  idleTimer = setTimeout(() => {
    closeAutoWindow();
  }, IDLE_TIMEOUT_MS);
}

// ─── Commands ────────────────────────────────────────────────

async function cmdNavigate(cmd) {
  const { url } = cmd;
  if (!url || (!url.startsWith('http://') && !url.startsWith('https://'))) {
    throw new Error(`Invalid URL: ${url}`);
  }

  await ensureAutoWindow();

  if (activeTabId) {
    await chrome.tabs.update(activeTabId, { url });
  } else {
    const tab = await chrome.tabs.create({ windowId: autoWindowId, url });
    activeTabId = tab.id;
  }

  // Wait for page load
  await waitForTabLoad(activeTabId);
  return { tabId: activeTabId, url };
}

async function cmdExec(cmd) {
  const { code, tabId } = cmd;
  if (!code) throw new Error('Missing code');

  const targetTab = tabId || activeTabId;
  if (!targetTab) throw new Error('No active tab');

  // Use chrome.debugger to evaluate JS (more powerful than scripting API)
  await attachDebugger(targetTab);

  const result = await chrome.debugger.sendCommand(
    { tabId: targetTab },
    'Runtime.evaluate',
    {
      expression: code,
      returnByValue: true,
      awaitPromise: true,
    }
  );

  await detachDebugger(targetTab);

  if (result.exceptionDetails) {
    throw new Error(result.exceptionDetails.text || 'Evaluation error');
  }

  return result.result?.value;
}

async function cmdTabs(cmd) {
  const { subAction } = cmd;
  switch (subAction) {
    case 'list': {
      const tabs = autoWindowId
        ? await chrome.tabs.query({ windowId: autoWindowId })
        : [];
      return tabs.map(t => ({ id: t.id, url: t.url, title: t.title }));
    }
    case 'create': {
      await ensureAutoWindow();
      const tab = await chrome.tabs.create({
        windowId: autoWindowId,
        url: cmd.url || 'about:blank',
      });
      activeTabId = tab.id;
      return { tabId: tab.id };
    }
    case 'close': {
      if (cmd.tabId) {
        await chrome.tabs.remove(cmd.tabId);
        if (cmd.tabId === activeTabId) activeTabId = null;
      }
      return { closed: true };
    }
    default:
      throw new Error(`Unknown tabs subAction: ${subAction}`);
  }
}

async function cmdCookies(cmd) {
  const { domain, url: cookieUrl } = cmd;
  const params = {};
  if (domain) params.domain = domain;
  if (cookieUrl) params.url = cookieUrl;
  const cookies = await chrome.cookies.getAll(params);
  return cookies.map(c => ({
    name: c.name,
    value: c.value,
    domain: c.domain,
    path: c.path,
  }));
}

async function cmdScreenshot(cmd) {
  const targetTab = cmd.tabId || activeTabId;
  if (!targetTab) throw new Error('No active tab');

  await attachDebugger(targetTab);
  const result = await chrome.debugger.sendCommand(
    { tabId: targetTab },
    'Page.captureScreenshot',
    { format: cmd.format || 'png' }
  );
  await detachDebugger(targetTab);

  return { dataUrl: `data:image/png;base64,${result.data}` };
}

// ─── Debugger Helpers ────────────────────────────────────────

const attachedTabs = new Set();

async function attachDebugger(tabId) {
  if (attachedTabs.has(tabId)) return;
  await chrome.debugger.attach({ tabId }, '1.3');
  attachedTabs.add(tabId);
}

async function detachDebugger(tabId) {
  if (!attachedTabs.has(tabId)) return;
  try {
    await chrome.debugger.detach({ tabId });
  } catch {}
  attachedTabs.delete(tabId);
}

// Clean up on tab close
chrome.tabs.onRemoved.addListener((tabId) => {
  attachedTabs.delete(tabId);
  if (tabId === activeTabId) activeTabId = null;
});

// Clean up on window close
chrome.windows.onRemoved.addListener((windowId) => {
  if (windowId === autoWindowId) {
    autoWindowId = null;
    activeTabId = null;
  }
});

// ─── Tab Load Helper ─────────────────────────────────────────

function waitForTabLoad(tabId) {
  return new Promise((resolve) => {
    function listener(updatedTabId, info) {
      if (updatedTabId === tabId && info.status === 'complete') {
        chrome.tabs.onUpdated.removeListener(listener);
        resolve();
      }
    }
    chrome.tabs.onUpdated.addListener(listener);
    // Timeout fallback
    setTimeout(() => {
      chrome.tabs.onUpdated.removeListener(listener);
      resolve();
    }, 15000);
  });
}

// ─── Popup Status ────────────────────────────────────────────

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === 'getStatus') {
    sendResponse({
      connected,
      reconnecting: !connected && reconnectAttempts > 0,
    });
  }
  return true;
});

// ─── Start ───────────────────────────────────────────────────

connect();
