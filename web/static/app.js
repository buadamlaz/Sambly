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
});
