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

  // ── Table search ──────────────────────────────────────────────────────────
  initTableSearch();
});

// ── Table search: filters rows by text content ────────────────────────────────
function initTableSearch() {
  document.querySelectorAll('[data-search-for]').forEach(function(input) {
    const table = document.getElementById(input.dataset.searchFor);
    if (!table) return;
    const tbody = table.querySelector('tbody');
    if (!tbody) return;
    input.addEventListener('input', function() {
      const q = input.value.trim().toLowerCase();
      Array.from(tbody.rows).forEach(function(row) {
        row.style.display = (!q || row.textContent.toLowerCase().includes(q)) ? '' : 'none';
      });
    });
  });
}

// ── User picker: inline dropdown, keyboard nav, comma-separated multi-value ──
// Reads options from <datalist id="samba-users-list"> (server-rendered).
// Each input uses its own .user-picker-drop sibling inside .user-picker-wrap.
function initUserPickers() {
  const inputs = document.querySelectorAll('[data-user-picker]');
  if (!inputs.length) return;

  const datalist = document.getElementById('samba-users-list');
  const allOptions = datalist
    ? Array.from(datalist.options).map(function(o) { return o.value; })
    : [];

  inputs.forEach(function(input) {
    const wrap = input.closest('.user-picker-wrap');
    if (!wrap) return;
    const drop = wrap.querySelector('.user-picker-drop');
    if (!drop) return;

    let focusedIndex = -1;
    let currentItems = [];

    function getLastToken() {
      const parts = input.value.split(',');
      return parts[parts.length - 1].trim().toLowerCase();
    }

    function commitSelection(value) {
      const parts = input.value.split(',');
      parts[parts.length - 1] = ' ' + value;
      input.value = parts.join(',').replace(/^\s*,\s*/, '') + ', ';
      hideDrop();
      input.focus();
    }

    function renderDrop() {
      const token = getLastToken();
      const matches = token
        ? allOptions.filter(function(u) { return u.toLowerCase().includes(token); })
        : allOptions;

      drop.innerHTML = '';
      currentItems = [];
      focusedIndex = -1;

      if (!matches.length) {
        const empty = document.createElement('div');
        empty.className = 'user-picker-empty';
        empty.textContent = 'No users found';
        drop.appendChild(empty);
      } else {
        matches.forEach(function(u) {
          const item = document.createElement('div');
          item.className = 'user-picker-item';
          item.textContent = u;
          item.addEventListener('mousedown', function(e) {
            e.preventDefault();
            commitSelection(u);
          });
          drop.appendChild(item);
          currentItems.push(item);
        });
      }

      drop.classList.add('visible');
    }

    function hideDrop() {
      drop.classList.remove('visible');
      focusedIndex = -1;
      currentItems = [];
    }

    function setFocus(idx) {
      currentItems.forEach(function(el, i) {
        el.classList.toggle('active', i === idx);
      });
      focusedIndex = idx;
      if (currentItems[idx]) {
        currentItems[idx].scrollIntoView({ block: 'nearest' });
      }
    }

    input.addEventListener('focus', renderDrop);
    input.addEventListener('input', renderDrop);
    input.addEventListener('blur', function() { setTimeout(hideDrop, 160); });

    input.addEventListener('keydown', function(e) {
      if (!drop.classList.contains('visible')) return;
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setFocus(Math.min(focusedIndex + 1, currentItems.length - 1));
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setFocus(Math.max(focusedIndex - 1, 0));
      } else if (e.key === 'Enter' && focusedIndex >= 0) {
        e.preventDefault();
        commitSelection(currentItems[focusedIndex].textContent);
      } else if (e.key === 'Escape') {
        hideDrop();
      }
    });
  });
}