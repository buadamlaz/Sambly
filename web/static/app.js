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

// ── User picker: portal dropdown, keyboard nav, comma-separated multi-value ───
// Reads options from <datalist id="samba-users-list"> (server-rendered, synchronous).
// Dropdown is appended to <body> so it is never clipped by modal overflow.
function initUserPickers() {
  const inputs = document.querySelectorAll('[data-user-picker]');
  if (!inputs.length) return;

  const datalist = document.getElementById('samba-users-list');
  const allOptions = datalist
    ? Array.from(datalist.options).map(function(o) { return o.value; })
    : [];

  // One shared portal dropdown for all pickers on the page
  const drop = document.createElement('div');
  drop.className = 'user-picker-drop';
  document.body.appendChild(drop);

  let activeInput = null;
  let focusedIndex = -1;
  let currentItems = [];

  function getLastToken(input) {
    const parts = input.value.split(',');
    return parts[parts.length - 1].trim().toLowerCase();
  }

  function commitSelection(input, value) {
    const parts = input.value.split(',');
    parts[parts.length - 1] = ' ' + value;
    input.value = parts.join(',').replace(/^\s*,\s*/, '') + ', ';
    hideDrop();
    input.focus();
  }

  function positionDrop(input) {
    const rect = input.getBoundingClientRect();
    const spaceBelow = window.innerHeight - rect.bottom;
    const spaceAbove = rect.top;
    const dropHeight = Math.min(220, allOptions.length * 38 + 2);

    if (spaceBelow >= dropHeight || spaceBelow >= spaceAbove) {
      // Show below
      drop.style.top  = (rect.bottom + 4) + 'px';
      drop.style.bottom = 'auto';
      drop.classList.remove('up');
    } else {
      // Show above
      drop.style.top  = 'auto';
      drop.style.bottom = (window.innerHeight - rect.top + 4) + 'px';
      drop.classList.add('up');
    }
    drop.style.left  = rect.left + 'px';
    drop.style.width = rect.width + 'px';
  }

  function renderDrop(input) {
    const token = getLastToken(input);
    const matches = token
      ? allOptions.filter(u => u.toLowerCase().includes(token))
      : allOptions;

    drop.innerHTML = '';
    currentItems = [];
    focusedIndex = -1;

    if (!matches.length) {
      const empty = document.createElement('div');
      empty.className = 'user-picker-empty';
      empty.textContent = 'Kullanıcı bulunamadı';
      drop.appendChild(empty);
      currentItems = [];
    } else {
      matches.forEach(function(u) {
        const item = document.createElement('div');
        item.className = 'user-picker-item';
        item.textContent = u;
        item.addEventListener('mousedown', function(e) {
          e.preventDefault();
          commitSelection(input, u);
        });
        drop.appendChild(item);
        currentItems.push(item);
      });
    }

    positionDrop(input);
    drop.classList.add('visible');
  }

  function hideDrop() {
    drop.classList.remove('visible');
    focusedIndex = -1;
    currentItems = [];
    activeInput = null;
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

  inputs.forEach(function(input) {
    input.addEventListener('focus', function() {
      activeInput = input;
      renderDrop(input);
    });

    input.addEventListener('input', function() {
      activeInput = input;
      renderDrop(input);
    });

    input.addEventListener('blur', function() {
      setTimeout(hideDrop, 160);
    });

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
        commitSelection(input, currentItems[focusedIndex].textContent);
      } else if (e.key === 'Escape') {
        hideDrop();
      }
    });
  });

  // Reposition on scroll/resize
  window.addEventListener('scroll', function() {
    if (activeInput && drop.classList.contains('visible')) {
      positionDrop(activeInput);
    }
  }, true);

  window.addEventListener('resize', function() {
    if (activeInput && drop.classList.contains('visible')) {
      positionDrop(activeInput);
    }
  });
}