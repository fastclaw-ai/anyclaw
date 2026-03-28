chrome.runtime.sendMessage({ type: 'getStatus' }, (resp) => {
  const dot = document.getElementById('dot');
  const status = document.getElementById('status');
  const hint = document.getElementById('hint');

  if (chrome.runtime.lastError || !resp) {
    dot.className = 'dot disconnected';
    status.innerHTML = '<strong>Not connected</strong>';
    hint.style.display = 'block';
    return;
  }

  if (resp.connected) {
    dot.className = 'dot connected';
    status.innerHTML = '<strong>Connected</strong>';
    hint.style.display = 'none';
  } else if (resp.reconnecting) {
    dot.className = 'dot connecting';
    status.innerHTML = '<strong>Reconnecting...</strong>';
    hint.style.display = 'none';
  } else {
    dot.className = 'dot disconnected';
    status.innerHTML = '<strong>Not connected</strong>';
    hint.style.display = 'block';
  }
});
