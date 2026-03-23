/* i18n — Skwad internationalization (zero dependencies) */
(function () {
  'use strict';

  var SUPPORTED = ['en','de','it','bg','zh-TW','ko','ja','fr','es','pt-BR','zh-CN','th','nl','pl'];
  var FALLBACK = 'en';
  var currentLang = FALLBACK;
  var strings = {};      // { lang: { key: value } }
  var pluralRules = {};  // { lang: Intl.PluralRules } — cached per language
  var ready = false;
  var onReady = [];

  // ── Public API ──────────────────────────────────────────────────

  // Translate a key with optional parameter interpolation.
  // t('POWER_BADGE', { power: 200 }) → "200 mW MAX"
  window.t = function (key, params) {
    var s = (strings[currentLang] && strings[currentLang][key])
         || (strings[FALLBACK] && strings[FALLBACK][key])
         || key;
    if (params) {
      Object.keys(params).forEach(function (k) {
        s = s.split('{{' + k + '}}').join(params[k]);
      });
    }
    return s;
  };

  // Translate a plural key. Uses Intl.PluralRules (cached per language)
  // to select the correct suffix: key_zero, key_one, key_two, key_few, key_many, key_other.
  // Falls back to key_other, then key itself.
  // tPlural('PILOT_COUNT', 4) → "4 PILOTS"
  window.tPlural = function (key, count, params) {
    if (!pluralRules[currentLang]) pluralRules[currentLang] = new Intl.PluralRules(currentLang);
    var rule = pluralRules[currentLang].select(count);
    var p = params ? Object.assign({ count: count }, params) : { count: count };
    var s = (strings[currentLang] && (strings[currentLang][key + '_' + rule] || strings[currentLang][key + '_other']))
         || (strings[FALLBACK] && (strings[FALLBACK][key + '_' + rule] || strings[FALLBACK][key + '_other']))
         || key;
    Object.keys(p).forEach(function (k) {
      s = s.split('{{' + k + '}}').join(p[k]);
    });
    return s;
  };

  // Re-translate all DOM elements with data-i18n attributes.
  // Uses textContent only — never innerHTML — for XSS safety.
  window.translateDOM = function () {
    document.querySelectorAll('[data-i18n]').forEach(function (el) {
      var key = el.getAttribute('data-i18n');
      el.textContent = t(key);
    });
    document.querySelectorAll('[data-i18n-placeholder]').forEach(function (el) {
      el.placeholder = t(el.getAttribute('data-i18n-placeholder'));
    });
    document.querySelectorAll('[data-i18n-aria]').forEach(function (el) {
      el.setAttribute('aria-label', t(el.getAttribute('data-i18n-aria')));
    });
    document.querySelectorAll('[data-i18n-title]').forEach(function (el) {
      el.title = t(el.getAttribute('data-i18n-title'));
    });
    // CSS content replacement via data-label
    document.querySelectorAll('[data-i18n-label]').forEach(function (el) {
      el.setAttribute('data-label', t(el.getAttribute('data-i18n-label')));
    });
    // Update lang attribute
    document.documentElement.lang = currentLang;
  };

  // Switch language. Returns a promise that resolves when strings are loaded.
  window.setLanguage = function (lang) {
    if (SUPPORTED.indexOf(lang) === -1) lang = FALLBACK;
    currentLang = lang;
    localStorage.setItem('skwad-lang', lang);
    return loadStrings(lang).then(function () {
      translateDOM();
      // Dispatch custom event so app.js can re-render dynamic content.
      // Uses 'skwad-languagechange' to avoid collision with the browser's
      // native 'languagechange' event (fired on OS/browser language change).
      window.dispatchEvent(new CustomEvent('skwad-languagechange', { detail: { lang: lang } }));
    });
  };

  window.getCurrentLanguage = function () { return currentLang; };
  window.getSupportedLanguages = function () { return SUPPORTED.slice(); };

  // Language display names (in their own language) — kept here so the
  // mapping is in one place alongside the SUPPORTED array.
  var LANG_NAMES = {
    en: 'English', de: 'Deutsch', it: 'Italiano', bg: '\u0411\u044a\u043b\u0433\u0430\u0440\u0441\u043a\u0438',
    'zh-TW': '\u7e41\u9ad4\u4e2d\u6587', ko: '\ud55c\uad6d\uc5b4', ja: '\u65e5\u672c\u8a9e', fr: 'Fran\u00e7ais',
    es: 'Espa\u00f1ol', 'pt-BR': 'Portugu\u00eas', 'zh-CN': '\u7b80\u4f53\u4e2d\u6587',
    th: '\u0e44\u0e17\u0e22', nl: 'Nederlands', pl: 'Polski'
  };
  window.getLanguageName = function (code) { return LANG_NAMES[code] || code; };

  // Register a callback to run after i18n is initialized.
  window.onI18nReady = function (fn) {
    if (ready) fn();
    else onReady.push(fn);
  };

  // ── Internal ────────────────────────────────────────────────────

  function loadStrings(lang) {
    if (strings[lang]) return Promise.resolve();
    return fetch('/locales/' + lang + '.json')
      .then(function (r) { return r.json(); })
      .then(function (data) { strings[lang] = data; })
      .catch(function () {
        console.warn('[i18n] Failed to load locale: ' + lang);
        strings[lang] = {};
      });
  }

  function detectLanguage() {
    var saved = localStorage.getItem('skwad-lang');
    if (saved && SUPPORTED.indexOf(saved) !== -1) return saved;
    // Match navigator language to supported list
    var nav = (navigator.language || '').replace('_', '-');
    // Try exact match first (e.g., zh-TW, pt-BR)
    if (SUPPORTED.indexOf(nav) !== -1) return nav;
    // Try base language (e.g., "de-AT" → "de")
    var base = nav.split('-')[0];
    for (var i = 0; i < SUPPORTED.length; i++) {
      if (SUPPORTED[i] === base || SUPPORTED[i].split('-')[0] === base) return SUPPORTED[i];
    }
    return FALLBACK;
  }

  // ── Init ────────────────────────────────────────────────────────

  var detected = detectLanguage();
  // Always load English as fallback, then detected language
  var loads = [loadStrings(FALLBACK)];
  if (detected !== FALLBACK) loads.push(loadStrings(detected));

  Promise.all(loads).then(function () {
    currentLang = detected;
    // Wait for DOM to be ready before translating — i18n.js loads before
    // app.js, so the fetch() promise may resolve before DOMContentLoaded.
    function apply() {
      document.documentElement.lang = currentLang;
      translateDOM();
      ready = true;
      onReady.forEach(function (fn) { fn(); });
      onReady = [];
    }
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', apply);
    } else {
      apply();
    }
  });
})();
