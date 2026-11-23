/**
 * Anime Quotes API - Main JavaScript
 * Vanilla JavaScript enhancements for toasts, modals, and API helpers
 */

// ============================================
// Toast Notifications
// ============================================

/**
 * Show a toast notification
 * @param {string} message - The message to display
 * @param {string} type - Type of toast: 'info', 'success', 'warning', 'error'
 * @param {number} duration - How long to show the toast (ms)
 */
function showToast(message, type = 'info', duration = 3000) {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = `toast ${type}`;

  const icons = {
    success: '✓',
    error: '✕',
    warning: '⚠',
    info: 'ℹ'
  };

  const icon = document.createElement('span');
  icon.className = 'toast-icon';
  icon.setAttribute('aria-hidden', 'true');
  icon.textContent = icons[type] || icons.info;
  const text = document.createElement('span');
  text.className = 'toast-message';
  text.textContent = message;
  const close = document.createElement('button');
  close.className = 'toast-close';
  close.type = 'button';
  close.dataset.action = 'close-toast';
  close.setAttribute('aria-label', document.documentElement.dataset.closeNotification || 'Close notification');
  close.textContent = '×';
  close.addEventListener('click', () => toast.remove());
  toast.append(icon, text, close);

  container.appendChild(toast);

  setTimeout(() => {
    toast.classList.add('toast-dismissed');
    setTimeout(() => toast.remove(), 300);
  }, duration);
}

// ============================================
// Modal Dialogs
// ============================================

/**
 * Show a modal dialog
 * @param {string} title - Modal title
 * @param {string} content - Plain-text modal content
 * @param {string} modalId - Optional modal ID
 */
function showModal(title, content, modalId = 'dynamic-modal') {
  const existingModal = document.getElementById(modalId);
  if (existingModal) {
    existingModal.remove();
  }

  const modal = document.createElement('div');
  modal.id = modalId;
  modal.className = 'modal active';
  const backdrop = document.createElement('div');
  backdrop.className = 'modal-backdrop';
  backdrop.dataset.closeModal = modalId;
  const panel = document.createElement('div');
  panel.className = 'modal-content';
  const header = document.createElement('div');
  header.className = 'modal-header';
  const heading = document.createElement('h2');
  heading.textContent = title;
  const close = document.createElement('button');
  close.className = 'modal-close';
  close.type = 'button';
  close.dataset.closeModal = modalId;
  close.setAttribute('aria-label', document.documentElement.dataset.closeDialog || 'Close dialog');
  close.textContent = '×';
  header.append(heading, close);
  const body = document.createElement('div');
  body.className = 'modal-body';
  body.textContent = content;
  panel.append(header, body);
  modal.append(backdrop, panel);

  let container = document.getElementById('modal-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'modal-container';
    document.body.appendChild(container);
  }
  container.appendChild(modal);
}

/**
 * Close a modal dialog
 * @param {string} modalId - Modal ID to close
 */
function closeModal(modalId = 'dynamic-modal') {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.classList.remove('active');
    setTimeout(() => modal.remove(), 300);
  }
}

// ============================================
// API Helpers
// ============================================

/**
 * Make a GET request to the API
 * @param {string} endpoint - API endpoint
 * @returns {Promise<any>} - Response data
 */
async function apiGet(endpoint) {
  try {
    const response = await fetch(endpoint);
    if (!response.ok) {
      throw new Error(`HTTP error ${response.status}`);
    }
    const payload = await response.json();
    return payload && payload.ok === true ? payload.data : payload;
  } catch (error) {
    showToast(`API Error: ${error.message}`, 'error');
    throw error;
  }
}

/**
 * Make a POST request to the API
 * @param {string} endpoint - API endpoint
 * @param {object} data - Data to send
 * @returns {Promise<any>} - Response data
 */
async function apiPost(endpoint, data) {
  try {
    const response = await fetch(endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      throw new Error(`HTTP error ${response.status}`);
    }
    return await response.json();
  } catch (error) {
    showToast(`API Error: ${error.message}`, 'error');
    throw error;
  }
}

// ============================================
// Quote Utilities
// ============================================

/**
 * Fetch and display a random quote
 */
async function getRandomQuote() {
  try {
    const quote = await apiGet('/api/v1/random');
    displayQuote(quote);
    showToast('New quote loaded!', 'success', 2000);
  } catch (error) {
    console.error('Failed to fetch random quote:', error);
  }
}

/**
 * Display a quote in the quote container
 * @param {object} quote - Quote object with anime, character, and quote properties
 */
function displayQuote(quote) {
  const quoteText = document.getElementById('quote-text');
  const quoteCharacter = document.getElementById('quote-character');
  const quoteAnime = document.getElementById('quote-anime');

  if (quoteText) quoteText.textContent = quote.quote;
  if (quoteCharacter) quoteCharacter.textContent = quote.character;
  if (quoteAnime) quoteAnime.textContent = quote.anime;
}

/**
 * Copy quote to clipboard
 */
function copyQuote() {
  const quoteText = document.getElementById('quote-text');
  const quoteCharacter = document.getElementById('quote-character');
  const quoteAnime = document.getElementById('quote-anime');

  if (quoteText && quoteCharacter && quoteAnime) {
    const text = `"${quoteText.textContent}" - ${quoteCharacter.textContent}, ${quoteAnime.textContent}`;
    navigator.clipboard.writeText(text).then(() => {
      showToast('Quote copied to clipboard!', 'success', 2000);
    }).catch(() => {
      showToast('Failed to copy quote', 'error');
    });
  }
}

// ============================================
// Keyboard Shortcuts
// ============================================

/**
 * Handle keyboard shortcuts
 */
document.addEventListener('keydown', (e) => {
  // Space bar for new quote (on homepage)
  if (e.code === 'Space' && e.target.tagName !== 'INPUT' && e.target.tagName !== 'TEXTAREA') {
    const getQuoteBtn = document.getElementById('get-quote-btn');
    if (getQuoteBtn) {
      e.preventDefault();
      getRandomQuote();
    }
  }
});

// ============================================
// Initialization
// ============================================

const bootTimer = window.setTimeout(async () => {
  let recovered = false;
  try {
    recovered = sessionStorage.getItem('anime_recovered') === '1';
    if (!recovered) sessionStorage.setItem('anime_recovered', '1');
  } catch {
    recovered = true;
  }
  if (recovered) {
    document.body.replaceChildren();
    const main = document.createElement('main');
    const title = document.createElement('h1');
    const description = document.createElement('p');
    const reload = document.createElement('button');
    title.textContent = document.documentElement.dataset.recoveryTitle;
    description.textContent = document.documentElement.dataset.recoveryDescription;
    reload.textContent = document.documentElement.dataset.recoveryReload;
    reload.addEventListener('click', () => window.location.reload());
    main.append(title, description, reload);
    document.body.append(main);
    return;
  }
  try {
    const registrations = await navigator.serviceWorker?.getRegistrations() ?? [];
    await Promise.all(registrations.map((registration) => registration.unregister()));
    const names = await caches?.keys() ?? [];
    await Promise.all(names.map((name) => caches.delete(name)));
  } catch {
    // The reload remains the final recovery attempt when browser storage is unavailable.
  }
  window.location.reload();
}, 10000);

/**
 * Initialize the application
 */
document.addEventListener('DOMContentLoaded', () => {
  window.clearTimeout(bootTimer);
  try {
    sessionStorage.removeItem('anime_recovered');
  } catch {
    // Session storage is optional; the page remains usable without it.
  }
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/sw.js').catch(() => {});
  }
  const menuToggle = document.getElementById('nav-toggle');
  const menuPanel = document.getElementById('nav-panel');
  const menuButton = document.querySelector('.mobile-menu-toggle');
  if (menuPanel) menuPanel.inert = !menuToggle?.checked;
  document.addEventListener('keydown', (event) => {
    if (!menuToggle?.checked) return;
    if (event.key === 'Escape') {
      menuToggle.checked = false;
      menuButton?.focus();
      return;
    }
    if (event.key !== 'Tab' || !menuPanel) return;
    const focusable = menuPanel.querySelectorAll('a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])');
    if (!focusable.length) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  });
  menuToggle?.addEventListener('change', () => {
    if (menuPanel) menuPanel.inert = !menuToggle.checked;
    menuButton?.setAttribute('aria-expanded', String(menuToggle.checked));
    if (menuToggle.checked) menuPanel?.querySelector('a')?.focus();
    else menuButton?.focus();
  });
  const updateMenuOverflow = () => {
    document.documentElement.classList.toggle('nav-collapsed', window.innerWidth >= 768 && Boolean(document.querySelector('.nav-links')?.scrollWidth > document.querySelector('.nav-links')?.clientWidth));
  };
  updateMenuOverflow();
  window.addEventListener('resize', updateMenuOverflow);
  document.querySelectorAll('[data-action]').forEach((control) => {
    control.addEventListener('click', () => {
      switch (control.dataset.action) {
        case 'random-quote': getRandomQuote(); break;
        case 'copy-quote': copyQuote(); break;
        case 'copy-text': {
          const target = document.getElementById(control.dataset.copyTarget);
          if (target && navigator.clipboard) {
            navigator.clipboard.writeText(target.value || target.textContent || '').catch(() => {});
          }
          break;
        }
      }
    });
  });
  document.addEventListener('click', (event) => {
    const closeControl = event.target.closest('[data-close-modal]');
    if (closeControl) closeModal(closeControl.dataset.closeModal);
  });

  const queryForm = document.querySelector('[data-query-form]');
  if (queryForm) {
    queryForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      const query = document.getElementById('query').value;
      const result = document.getElementById('result');
      try {
        const response = await fetch('/api/graphql', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query }),
        });
        result.textContent = JSON.stringify(await response.json(), null, 2);
      } catch {
        result.textContent = 'The query request failed. Submit the form again to retry.';
      }
    });
  }

  // Log initialization
  console.log('Anime Quotes API - Frontend initialized');
});
