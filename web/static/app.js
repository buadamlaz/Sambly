// Sambly — frontend JS (no inline handlers, CSP-safe)

// ── Modal helpers ─────────────────────────────────────────────────────────────
function openModal(id) {
  const el = document.getElementById(id);
  if (!el) return;
  el.style.display = 'flex';
  const input = el.querySelector('input:not([type=hidden]),select,textarea');
  if (input) setTimeout(() => input.focus(), 60);
}

function closeModal(id) {
  const el = document.getElementById(id);
  if (el) el.style.display = 'none';
}

// ── Event delegation (replaces all inline onclick/onsubmit) ───────────────────
document.addEventListener('click', function(e) {
  const t = e.target;

  // [data-open-modal="modalId"]  → open modal
  const opener = t.closest('[data-open-modal]');
  if (opener) {
    const modalId = opener.dataset.openModal;

    // Set password modal: copy username into hidden input + label
    if (opener.dataset.username) {
      const usernameInput = document.getElementById('pass-username');
      const usernameLabel = document.getElementById('pass-label');
      if (usernameInput) usernameInput.value = opener.dataset.username;
      if (usernameLabel) usernameLabel.textContent = opener.dataset.username;
    }

    openModal(modalId);
    return;
  }

  // [data-close-modal="modalId"]  → close modal
  const closer = t.closest('[data-close-modal]');
  if (closer) {
    closeModal(closer.dataset.closeModal);
    return;
  }

  // Click on overlay backdrop → close
  if (t.classList.contains('modal-overlay')) {
    t.style.display = 'none';
    return;
  }

  // [data-confirm-delete]  → fill hidden form + submit after confirm
  const delBtn = t.closest('[data-confirm-delete]');
  if (delBtn) {
    const msg  = delBtn.dataset.confirmDelete;
    const form = document.getElementById(delBtn.dataset.form);
    const field = delBtn.dataset.field;
    const value = delBtn.dataset.value;
    if (!confirm(msg || 'Delete this item?')) return;
    if (form && field) {
      const input = form.querySelector(`[name="${field}"]`);
      if (input) input.value = value || '';
    }
    if (form) form.submit();
    return;
  }
});

// ── Form submit confirm via data-confirm attribute ────────────────────────────
document.addEventListener('submit', function(e) {
  const msg = e.target.dataset.confirm;
  if (msg && !confirm(msg)) {
    e.preventDefault();
  }
});

// ── Escape key closes modals ──────────────────────────────────────────────────
document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') {
    document.querySelectorAll('.modal-overlay').forEach(el => {
      el.style.display = 'none';
    });
  }
});

// ── Auto-dismiss success alerts after 5 s ────────────────────────────────────
document.addEventListener('DOMContentLoaded', function() {
  document.querySelectorAll('.alert.alert-success').forEach(function(alert) {
    setTimeout(function() {
      alert.style.transition = 'opacity 0.4s';
      alert.style.opacity = '0';
      setTimeout(() => alert.style.display = 'none', 400);
    }, 5000);
  });

  // ── User picker autocomplete ──────────────────────────────────────────────
  initUserPickers();
});

// Multi-value user picker: autocomplete on comma-separated inputs.
// Reads options from <datalist id="samba-users-list">.
// Attach to inputs with: data-user-picker attribute.
// Wrapper div .user-picker-wrap must already be in HTML (next sibling is .user-picker-drop).
function initUserPickers() {
  const datalist = document.getElementById('samba-users-list');
  if (!datalist) return;

  const options = Array.from(datalist.options).map(o => o.value);

  document.querySelectorAll('[data-user-picker]').forEach(function(input) {
    // The .user-picker-drop div is already in the HTML as a sibling
    const wrap = input.closest('.user-picker-wrap');
    if (!wrap) return;
    const drop = wrap.querySelector('.user-picker-drop');
    if (!drop) return;

    function getLastToken() {
      const parts = input.value.split(',');
      return parts[parts.length - 1].trim().toLowerCase();
    }

    function showDrop() {
      const token = getLastToken();
      if (!token) { drop.style.display = 'none'; return; }

      const matches = options.filter(u => u.toLowerCase().startsWith(token));
      if (!matches.length) { drop.style.display = 'none'; return; }

      drop.innerHTML = '';
      matches.forEach(function(u) {
        const item = document.createElement('div');
        item.className = 'user-picker-item';
        item.textContent = u;
        item.addEventListener('mousedown', function(e) {
          e.preventDefault(); // don't blur input before we update value
          const parts = input.value.split(',');
          parts[parts.length - 1] = ' ' + u;
          // Add trailing comma+space so user can immediately type next user
          input.value = parts.join(',').replace(/^,\s*/, '') + ', ';
          drop.style.display = 'none';
          input.focus();
        });
        drop.appendChild(item);
      });
      drop.style.display = 'block';
    }

    input.addEventListener('input', showDrop);
    input.addEventListener('focus', showDrop);
    input.addEventListener('blur', function() {
      // Delay hide so mousedown on item fires first
      setTimeout(function() { drop.style.display = 'none'; }, 180);
    });
  });
}