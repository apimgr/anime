/**
 * Anime Quotes API - Main JavaScript
 * Vanilla JavaScript utilities for theme toggle, toast notifications, modals, and API helpers
 */

// ============================================
// Theme Management
// ============================================

/**
 * Initialize theme from localStorage or default to dark
 */
function initTheme() {
  const savedTheme = localStorage.getItem('theme') || 'dark';
  document.documentElement.setAttribute('data-theme', savedTheme);
  updateThemeIcon(savedTheme);
}

/**
 * Toggle between dark and light themes
 */
function toggleTheme() {
  const currentTheme = document.documentElement.getAttribute('data-theme');
  const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', newTheme);
  localStorage.setItem('theme', newTheme);
  updateThemeIcon(newTheme);
  showToast(`Switched to ${newTheme} theme`, 'info', 2000);
}

/**
 * Update theme toggle icon
 */
function updateThemeIcon(theme) {
  const themeIcon = document.querySelector('.theme-icon');
  if (themeIcon) {
    themeIcon.textContent = theme === 'dark' ? '🌙' : '☀️';
  }
}

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

  toast.innerHTML = `
    <span class="toast-icon">${icons[type] || icons.info}</span>
    <span class="toast-message">${message}</span>
    <button class="toast-close" onclick="this.parentElement.remove()">×</button>
  `;

  container.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    setTimeout(() => toast.remove(), 300);
  }, duration);
}

// ============================================
// Modal Dialogs
// ============================================

/**
 * Show a modal dialog
 * @param {string} title - Modal title
 * @param {string} content - Modal content (HTML)
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
  modal.innerHTML = `
    <div class="modal-backdrop" onclick="closeModal('${modalId}')"></div>
    <div class="modal-content">
      <div class="modal-header">
        <h2>${title}</h2>
        <button class="modal-close" onclick="closeModal('${modalId}')">×</button>
      </div>
      <div class="modal-body">
        ${content}
      </div>
    </div>
  `;

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
// Mobile Menu Toggle
// ============================================

/**
 * Toggle mobile navigation menu
 */
function toggleMobileMenu() {
  const nav = document.getElementById('main-nav');
  if (nav) {
    nav.classList.toggle('active');
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
    return await response.json();
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
  // Escape key closes modals
  if (e.key === 'Escape') {
    const activeModals = document.querySelectorAll('.modal.active');
    activeModals.forEach(modal => {
      modal.classList.remove('active');
      setTimeout(() => modal.remove(), 300);
    });
  }

  // Ctrl/Cmd + K for theme toggle
  if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
    e.preventDefault();
    toggleTheme();
  }

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

/**
 * Initialize the application
 */
document.addEventListener('DOMContentLoaded', () => {
  // Initialize theme
  initTheme();

  // Close mobile menu when clicking outside
  document.addEventListener('click', (e) => {
    const nav = document.getElementById('main-nav');
    const toggle = document.querySelector('.mobile-menu-toggle');
    if (nav && toggle && nav.classList.contains('active')) {
      if (!nav.contains(e.target) && !toggle.contains(e.target)) {
        nav.classList.remove('active');
      }
    }
  });

  // Log initialization
  console.log('Anime Quotes API - Frontend initialized');
});
