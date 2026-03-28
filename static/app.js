/* ================================================================
   SKWAD — FPV Frequency Coordinator
   Vanilla JS frontend — no frameworks, no build step
   ================================================================ */

(function () {
  'use strict';

  // ── State ──────────────────────────────────────────────────────
  const state = {
    sessionCode: null,
    pilotId: null,
    knownVersion: 0,
    pollTimer: null,
    // Setup wizard state
    callsign: '',
    videoSystem: '',
    fccUnlocked: false,
    goggles: '',
    bandwidthMHz: 0,
    raceMode: false,
    analogBands: ['R'],
    walksnailMode: '', // 'standard' or 'race'
    preferredFreqMHz: 0,
    // Tracked assignment for change detection
    myChannel: null,
    myFreqMHz: null,
    // Leader state
    isLeader: false,
    leaderPilotId: null,
    // Track whether we initiated the current assignment change
    expectingAssignmentChange: false,
    // Whether this pilot created the session (show leader info step)
    isCreator: false,
    // True when user is changing video system from within a session
    _changingVideoSystem: false,
    // Power ceiling set by session creator (mW), 0 = no limit
    powerCeilingMW: 0,
    // Slider index in the power step (default 200 mW = index 2)
    powerStepIndex: 2,
    // Power ceiling of the session we've joined (from server)
    sessionPowerCeiling: 0,
    // Fixed channels of the session we've joined (from server), JSON string
    sessionFixedChannels: '',
    // Session options chosen on leader info step
    optPowerCeiling: false,
    optFixedChannels: false,
    fixedChannels: '',
    // Pending power ceiling (mW) chosen in power step, 0 = no limit, -1 = not yet set
    pendingPowerMW: 0,
    // Feedback form
    feedbackReturnTo: null, // 'landing' or 'qr-overlay'
    feedbackType: 'feedback', // 'bug', 'feedback', or 'translation'
  };

  // ── Buddy group colors ────────────────────────────────────────
  const BUDDY_COLORS = [
    '', '#ff3333', '#33ff33', '#3399ff', '#ffcc00',
    '#ff66ff', '#00ffcc', '#ff9900', '#cc66ff'
  ];

  // ── Power ceiling slider steps ────────────────────────────────
  var POWER_STEPS = [
    { mw: 25,   dbm: '14 dBm',   guard: 10, channels: 8, tipKey: 'POWER_TIP_25MW' },
    { mw: 100,  dbm: '20 dBm',   guard: 12, channels: 8, tipKey: 'POWER_TIP_100MW' },
    { mw: 200,  dbm: '23 dBm',   guard: 14, channels: 8, tipKey: 'POWER_TIP_200MW' },
    { mw: 400,  dbm: '26 dBm',   guard: 16, channels: 8, tipKey: 'POWER_TIP_400MW' },
    { mw: 600,  dbm: '27.8 dBm', guard: 24, channels: 4, tipKey: 'POWER_TIP_600MW' },
    { mw: 800,  dbm: '29 dBm',   guard: 28, channels: 4, tipKey: 'POWER_TIP_800MW' },
    { mw: 1000, dbm: '30 dBm',   guard: 32, channels: 4, tipKey: 'POWER_TIP_1000MW' },
  ];

  // ── Fixed channel sets ────────────────────────────────────────
  var FIXED_CHANNEL_SETS = {
    2: [
      { name: 'MAX SPREAD', channels: [{ n: 'R1', f: 5658 }, { n: 'R8', f: 5917 }], spacing: 259, imd: 100, power: 'ANY POWER', systems: 'ANALOG / HDZERO', powerColor: '#4ade80' },
      { name: 'DJI SPREAD', channels: [{ n: 'DJI-CH1', f: 5669, dji: true }, { n: 'DJI-CH7', f: 5876, dji: true }], spacing: 207, imd: 100, power: 'ANY POWER', systems: 'DJI', powerColor: '#4ade80' }
    ],
    3: [
      { name: 'IMD CLEAN', channels: [{ n: 'R1', f: 5658 }, { n: 'R4', f: 5769 }, { n: 'R8', f: 5917 }], spacing: 111, imd: 100, power: 'ANY POWER', systems: 'ANALOG / HDZERO', powerColor: '#4ade80' },
      { name: 'MIXED CLEAN', channels: [{ n: 'R1', f: 5658 }, { n: 'DJI-CH5', f: 5805, dji: true }, { n: 'R8', f: 5917 }], spacing: 112, imd: 100, power: 'ANY POWER', systems: 'MIXED', powerColor: '#4ade80' },
      { name: 'DJI SPREAD', channels: [{ n: 'DJI-CH1', f: 5669, dji: true }, { n: 'DJI-CH4', f: 5769, dji: true }, { n: 'DJI-CH7', f: 5876, dji: true }], spacing: 100, imd: 78, power: 'ANY POWER', systems: 'DJI', powerColor: '#4ade80' }
    ],
    4: [
      { name: 'IMD CLEAN', channels: [{ n: 'R1', f: 5658 }, { n: 'R3', f: 5732 }, { n: 'R6', f: 5843 }, { n: 'R8', f: 5917 }], spacing: 74, imd: 100, power: 'ANY POWER', systems: 'ANALOG / HDZERO', powerColor: '#4ade80' },
      { name: 'MIXED CLEAN', channels: [{ n: 'R1', f: 5658 }, { n: 'DJI-CH3', f: 5741, dji: true }, { n: 'DJI-CH6', f: 5840, dji: true }, { n: 'R8', f: 5917 }], spacing: 77, imd: 98, power: 'ANY POWER', systems: 'MIXED', powerColor: '#4ade80' },
      { name: 'DJI SPREAD', channels: [{ n: 'DJI-CH1', f: 5669, dji: true }, { n: 'DJI-CH3', f: 5741, dji: true }, { n: 'DJI-CH5', f: 5805, dji: true }, { n: 'DJI-CH7', f: 5876, dji: true }], spacing: 64, imd: 69, power: 'ANY POWER', systems: 'DJI', powerColor: '#4ade80' }
    ],
    5: [
      { name: 'MIXED OPTIMAL', channels: [{ n: 'R1', f: 5658 }, { n: 'DJI-CH2', f: 5705, dji: true }, { n: 'R4', f: 5769 }, { n: 'R6', f: 5843 }, { n: 'R8', f: 5917 }], spacing: 47, imd: 91, power: '\u2264 600 mW', systems: 'MIXED', powerColor: '#60a5fa' },
      { name: 'ET5A', channels: [{ n: 'E3', f: 5665, multi: true }, { n: 'F1', f: 5740, multi: true }, { n: 'F4', f: 5800, multi: true }, { n: 'F7', f: 5860, multi: true }, { n: 'E6', f: 5905, multi: true }], spacing: 45, imd: 88, power: '\u2264 600 mW', systems: 'ANALOG (MULTI-BAND)', powerColor: '#60a5fa' },
      { name: 'RACEBAND 5', channels: [{ n: 'R1', f: 5658 }, { n: 'R3', f: 5732 }, { n: 'R5', f: 5806 }, { n: 'R6', f: 5843 }, { n: 'R8', f: 5917 }], spacing: 37, imd: 40, power: '\u2264 400 mW', systems: 'ANALOG / HDZERO', powerColor: '#f59e0b' },
      { name: 'DJI 5', channels: [{ n: 'DJI-CH1', f: 5669, dji: true }, { n: 'DJI-CH3', f: 5741, dji: true }, { n: 'DJI-CH5', f: 5805, dji: true }, { n: 'DJI-CH6', f: 5840, dji: true }, { n: 'DJI-CH7', f: 5876, dji: true }], spacing: 36, imd: 100, power: '\u2264 400 mW', systems: 'DJI', powerColor: '#f59e0b' }
    ]
  };

  // ── IMD (Intermodulation Distortion) Helpers ──────────────────
  function calcIMDProducts(pilots) {
    var products = [];
    if (!pilots || pilots.length < 2) return products;
    // Build freq list with original pilot indices preserved
    var freqs = [];
    for (var fi = 0; fi < pilots.length; fi++) {
      if (pilots[fi].AssignedFreqMHz) freqs.push({ origIdx: fi, freq: pilots[fi].AssignedFreqMHz });
    }
    for (var i = 0; i < freqs.length; i++) {
      for (var j = 0; j < freqs.length; j++) {
        if (i === j) continue;
        var imd = 2 * freqs[i].freq - freqs[j].freq;
        if (imd >= 5300 && imd <= 6000) {
          var hitIdx = -1;
          for (var k = 0; k < freqs.length; k++) {
            if (k !== i && k !== j && Math.abs(freqs[k].freq - imd) < 12) {
              hitIdx = freqs[k].origIdx;
              break;
            }
          }
          products.push({ freq: imd, hitIdx: hitIdx, sources: [freqs[i].origIdx, freqs[j].origIdx] });
        }
      }
    }
    return products;
  }

  function calcIMDScore(pilots) {
    if (!pilots || pilots.length < 2) return 100;
    var products = calcIMDProducts(pilots);
    // Proximity-weighted scoring: products closer to active channels score worse.
    // Inspired by ET's IMD Tools but with a 20 MHz threshold (matches our guard band range).
    var IMD_THRESHOLD = 20;
    var penaltySum = 0;
    var activeFreqs = [];
    pilots.forEach(function(p) { if (p.AssignedFreqMHz) activeFreqs.push(p.AssignedFreqMHz); });
    var seen = {};
    products.forEach(function(p) {
      var key = Math.round(p.freq);
      if (seen[key]) return;
      seen[key] = true;
      // Find closest active channel
      var minDist = IMD_THRESHOLD;
      for (var i = 0; i < activeFreqs.length; i++) {
        var d = Math.abs(p.freq - activeFreqs[i]);
        if (d < minDist) minDist = d;
      }
      if (minDist < IMD_THRESHOLD) {
        var gap = IMD_THRESHOLD - minDist;
        penaltySum += gap * gap; // quadratic — closer = much worse
      }
    });
    if (activeFreqs.length < 2) return 100;
    var score = 100 - penaltySum / (5 * activeFreqs.length);
    return Math.max(0, Math.min(100, Math.round(score)));
  }

  function getIMDSourcesForPilot(pilots, targetIdx) {
    var products = calcIMDProducts(pilots);
    var sources = [];
    var seen = {};
    products.forEach(function(p) {
      if (p.hitIdx !== targetIdx) return;
      var key = p.sources[0] + ',' + p.sources[1];
      if (seen[key]) return;
      seen[key] = true;
      sources.push({ a: pilots[p.sources[0]], b: pilots[p.sources[1]] });
    });
    return sources;
  }

  function formatIMDSources(pilots, targetIdx) {
    var sources = getIMDSourcesForPilot(pilots, targetIdx);
    if (sources.length === 0) return '';
    var parts = sources.map(function(s) {
      return s.a.Callsign + ' + ' + s.b.Callsign;
    });
    return t('IMD_FROM', { sources: parts.join(', ') });
  }

  function getIMDHitPilots(pilots) {
    var products = calcIMDProducts(pilots);
    var hitPilots = {};
    products.forEach(function(p) {
      if (p.hitIdx >= 0) hitPilots[p.hitIdx] = true;
    });
    return hitPilots;
  }

  // ── Video system display names ────────────────────────────────
  const SYSTEM_LABEL_KEYS = {
    analog: 'SYS_ANALOG',
    dji_v1: 'SYS_DJI_V1',
    dji_o3: 'SYS_DJI_O3',
    dji_o4: 'SYS_DJI_O4',
    hdzero: 'SYS_HDZERO',
    walksnail_std: 'SYS_WALKSNAIL',
    walksnail_race: 'SYS_WALKSNAIL_RACE',
    openipc: 'SYS_OPENIPC',
    spotter: 'SYS_SPOTTER',
  };

  // ── Channel tables (mirrors Go freq/tables.go) ────────────────
  const CHANNELS = {
    raceband: [
      { name: 'R1', freq: 5658 }, { name: 'R2', freq: 5695 },
      { name: 'R3', freq: 5732 }, { name: 'R4', freq: 5769 },
      { name: 'R5', freq: 5806 }, { name: 'R6', freq: 5843 },
      { name: 'R7', freq: 5880 }, { name: 'R8', freq: 5917 },
    ],
    fatshark: [
      { name: 'F1', freq: 5740 }, { name: 'F2', freq: 5760 },
      { name: 'F3', freq: 5780 }, { name: 'F4', freq: 5800 },
      { name: 'F5', freq: 5820 }, { name: 'F6', freq: 5840 },
      { name: 'F7', freq: 5860 }, { name: 'F8', freq: 5880 },
    ],
    boscam_e: [
      { name: 'E1', freq: 5705 }, { name: 'E2', freq: 5685 },
      { name: 'E3', freq: 5665 }, { name: 'E4', freq: 5645 },
      { name: 'E5', freq: 5885 }, { name: 'E6', freq: 5905 },
      { name: 'E7', freq: 5925 }, { name: 'E8', freq: 5945 },
    ],
    lowrace: [
      { name: 'L1', freq: 5362 }, { name: 'L2', freq: 5399 },
      { name: 'L3', freq: 5436 }, { name: 'L4', freq: 5473 },
      { name: 'L5', freq: 5510 }, { name: 'L6', freq: 5547 },
      { name: 'L7', freq: 5584 }, { name: 'L8', freq: 5621 },
    ],
    dji_v1_fcc: [
      { name: 'DJI-CH1', freq: 5660 }, { name: 'DJI-CH2', freq: 5695 },
      { name: 'DJI-CH3', freq: 5735 }, { name: 'DJI-CH4', freq: 5770 },
      { name: 'DJI-CH5', freq: 5805 }, { name: 'DJI-CH6', freq: 5878 },
      { name: 'DJI-CH7', freq: 5914 }, { name: 'DJI-CH8', freq: 5839 },
    ],
    dji_v1_stock: [
      { name: 'DJI-CH3', freq: 5735 }, { name: 'DJI-CH4', freq: 5770 },
      { name: 'DJI-CH5', freq: 5805 }, { name: 'DJI-CH8', freq: 5839 },
    ],
    dji_o3_stock: [
      { name: 'O3-CH1', freq: 5769 }, { name: 'O3-CH2', freq: 5805 },
      { name: 'O3-CH3', freq: 5840 },
    ],
    dji_o3_fcc: [
      { name: 'O3-CH1', freq: 5669 }, { name: 'O3-CH2', freq: 5705 },
      { name: 'O3-CH3', freq: 5769 }, { name: 'O3-CH4', freq: 5805 },
      { name: 'O3-CH5', freq: 5840 }, { name: 'O3-CH6', freq: 5876 },
      { name: 'O3-CH7', freq: 5912 },
    ],
    dji_o3_40_fcc: [{ name: 'O3-CH1', freq: 5677 }, { name: 'O3-CH2', freq: 5795 }, { name: 'O3-CH3', freq: 5902 }],
    dji_o3_40_stock: [{ name: 'O3-CH1', freq: 5795 }],
    dji_o4_stock: [
      { name: 'O4-CH1', freq: 5769 }, { name: 'O4-CH2', freq: 5790 },
      { name: 'O4-CH3', freq: 5815 },
    ],
    dji_o4_fcc: [
      { name: 'O4-CH1', freq: 5669 }, { name: 'O4-CH2', freq: 5705 },
      { name: 'O4-CH3', freq: 5741 }, { name: 'O4-CH4', freq: 5769 },
      { name: 'O4-CH5', freq: 5790 }, { name: 'O4-CH6', freq: 5815 },
      { name: 'O4-CH7', freq: 5876 },
    ],
    dji_o4_40_fcc: [
      { name: 'O4-CH1', freq: 5735 }, { name: 'O4-CH2', freq: 5795 },
      { name: 'O4-CH3', freq: 5855 },
    ],
    dji_o4_40_stock: [{ name: 'O4-CH1', freq: 5795 }],
    dji_o4_60: [{ name: 'O4-CH1', freq: 5795 }],
    openipc: [{ name: 'WiFi-165', freq: 5825 }],
  };

  // Maps band code letters to CHANNELS keys
  const ANALOG_BAND_MAP = { R: 'raceband', F: 'fatshark', E: 'boscam_e', L: 'lowrace' };

  function mergeAnalogBands(bands) {
    if (!bands || bands.length === 0) return CHANNELS.raceband;
    var seen = {};
    var merged = [];
    bands.forEach(function (code) {
      var key = ANALOG_BAND_MAP[code];
      if (!key || !CHANNELS[key]) return;
      CHANNELS[key].forEach(function (ch) {
        if (!seen[ch.freq]) {
          seen[ch.freq] = true;
          merged.push(ch);
        }
      });
    });
    return merged.length > 0 ? merged : CHANNELS.raceband;
  }

  // ── DOM references ────────────────────────────────────────────
  const $ = (id) => document.getElementById(id);
  const screens = {
    landing: $('screen-landing'),
    setup: $('screen-setup'),
    session: $('screen-session'),
    feedback: $('screen-feedback'),
  };

  // ── Helpers ───────────────────────────────────────────────────
  function showScreen(name) {
    Object.values(screens).forEach((s) => s.classList.add('hidden'));
    screens[name].classList.remove('hidden');
  }

  // ── Feedback helpers ──────────────────────────────────────────
  const FEEDBACK_PLACEHOLDERS = {
    bug: 'FEEDBACK_PLACEHOLDER_BUG',
    feedback: 'FEEDBACK_PLACEHOLDER_FEEDBACK',
    translation: 'FEEDBACK_PLACEHOLDER_TRANSLATION',
  };

  function updateFeedbackPlaceholder() {
    var textarea = $('feedback-text');
    var key = FEEDBACK_PLACEHOLDERS[state.feedbackType];
    textarea.placeholder = t(key);
    textarea.setAttribute('data-i18n-placeholder', key);
  }

  function collectFeedbackContext() {
    var ctx = {
      page: state.feedbackReturnTo || 'landing',
      language: localStorage.getItem('skwad-lang') || navigator.language || 'en',
      user_agent: navigator.userAgent,
      timestamp: new Date().toISOString(),
    };
    if (state.sessionCode) {
      ctx.session_code = state.sessionCode;
      var pilotCards = document.querySelectorAll('.pilot-card');
      ctx.pilot_count = pilotCards.length;
      if (state.sessionPowerCeiling) {
        ctx.power_ceiling_mw = state.sessionPowerCeiling;
      }
      if (state.videoSystem) {
        ctx.video_system = state.videoSystem;
      }
    }
    return ctx;
  }

  function openFeedbackScreen() {
    // Reset form state
    state.feedbackType = 'feedback';
    $('feedback-text').value = '';
    $('btn-feedback-submit').disabled = true;
    $('btn-feedback-submit').textContent = t('FEEDBACK_BTN_SUBMIT');
    $('feedback-status').classList.add('hidden');
    $('feedback-status').classList.remove('success', 'error');

    // Reset category selection
    document.querySelectorAll('.feedback-cat').forEach(function (b) { b.classList.remove('selected'); });
    document.querySelector('.feedback-cat[data-type="feedback"]').classList.add('selected');
    updateFeedbackPlaceholder();

    showScreen('feedback');
  }

  function closeFeedbackScreen() {
    if (state.feedbackReturnTo === 'qr-overlay') {
      showScreen('session');
      $('qr-overlay').classList.remove('hidden');
    } else {
      showScreen('landing');
    }
    state.feedbackReturnTo = null;
  }

  function submitFeedback() {
    var msg = $('feedback-text').value.trim();
    if (!msg) return;

    var btn = $('btn-feedback-submit');
    btn.disabled = true;

    var payload = {
      type: state.feedbackType,
      message: msg,
      context: collectFeedbackContext(),
    };

    fetch('/api/feedback', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
      .then(function (resp) {
        if (resp.status === 429) {
          showFeedbackStatus(t('FEEDBACK_RATE_LIMITED'), 'error');
          btn.disabled = false;
          return;
        }
        if (!resp.ok) throw new Error('status ' + resp.status);
        showFeedbackStatus(t('FEEDBACK_SUCCESS'), 'success');
        setTimeout(closeFeedbackScreen, 2000);
      })
      .catch(function () {
        showFeedbackStatus(t('FEEDBACK_ERROR'), 'error');
        btn.disabled = false;
      });
  }

  function showFeedbackStatus(text, type) {
    var el = $('feedback-status');
    el.textContent = text;
    el.classList.remove('hidden', 'success', 'error');
    el.classList.add(type);
  }

  function showStep(stepId) {
    document.querySelectorAll('.setup-step').forEach((s) => s.classList.add('hidden'));
    $(stepId).classList.remove('hidden');
    if (stepId === 'step-power') {
      refreshPowerSliderThumb();
    }
  }

  // Called when step-power becomes visible so offsetWidth is valid.
  function refreshPowerSliderThumb() {
    var track = $('power-slider-track');
    if (!track) return;
    var thumbRadius = 23;
    var usable = track.offsetWidth - thumbRadius * 2;
    var pct = state.powerStepIndex / (POWER_STEPS.length - 1);
    $('power-slider-thumb').style.left = (thumbRadius - 23 + pct * usable) + 'px';
    var step = POWER_STEPS[state.powerStepIndex];
    $('power-mw-value').textContent = step.mw;
    $('power-guard-value').textContent = step.guard;
    $('power-tip').textContent = t(step.tipKey);
    renderPowerSpectrum(step.guard);
  }

  function renderPowerSpectrum(guard) {
    var bw = 20;
    var total = bw + guard;
    var bar = $('power-spectrum-bar');
    if (!bar) return;
    while (bar.firstChild) bar.removeChild(bar.firstChild);
    var halfBwPct = ((bw / 2) / total * 100);
    var guardPct = (guard / total * 100);
    var seg1 = document.createElement('div');
    seg1.className = 'power-spectrum-segment power-seg-bw';
    seg1.style.width = halfBwPct + '%';
    seg1.textContent = (bw / 2) + '';
    bar.appendChild(seg1);
    var seg2 = document.createElement('div');
    seg2.className = 'power-spectrum-segment power-seg-guard';
    seg2.style.width = guardPct + '%';
    seg2.textContent = guard + ' ' + t('UNIT_MHZ');
    bar.appendChild(seg2);
    var seg3 = document.createElement('div');
    seg3.className = 'power-spectrum-segment power-seg-bw';
    seg3.style.width = halfBwPct + '%';
    seg3.textContent = (bw / 2) + '';
    bar.appendChild(seg3);
  }

  function showError(elementId, msg) {
    const el = $(elementId);
    el.textContent = msg;
    el.classList.remove('hidden');
  }

  function hideError(elementId) {
    $(elementId).classList.add('hidden');
  }

  function setLoading(btn, loading) {
    if (loading) {
      btn.classList.add('loading');
      btn.disabled = true;
    } else {
      btn.classList.remove('loading');
      btn.disabled = false;
    }
  }

  // ── Safe DOM builder helpers ──────────────────────────────────
  function el(tag, attrs, children) {
    const node = document.createElement(tag);
    if (attrs) {
      Object.keys(attrs).forEach(function (key) {
        if (key === 'className') node.className = attrs[key];
        else if (key === 'textContent') node.textContent = attrs[key];
        else node.setAttribute(key, attrs[key]);
      });
    }
    if (children) {
      children.forEach(function (child) {
        if (typeof child === 'string') {
          node.appendChild(document.createTextNode(child));
        } else if (child) {
          node.appendChild(child);
        }
      });
    }
    return node;
  }

  function clearChildren(node) {
    while (node.firstChild) node.removeChild(node.firstChild);
  }

  // ── API calls ─────────────────────────────────────────────────
  function apiHeaders() {
    var headers = { 'Content-Type': 'application/json' };
    if (state.pilotId) {
      headers['X-Pilot-ID'] = String(state.pilotId);
    }
    return headers;
  }

  async function apiPost(path, body) {
    const res = await fetch(path, {
      method: 'POST',
      headers: apiHeaders(),
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text.trim() || t('ERR_HTTP', { status: res.status }));
    }
    return res.json();
  }

  async function apiGet(path) {
    var opts = {};
    if (state.pilotId) {
      opts.headers = { 'X-Pilot-ID': String(state.pilotId) };
    }
    const res = await fetch(path, opts);
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text.trim() || t('ERR_HTTP', { status: res.status }));
    }
    return res.json();
  }

  async function apiPut(path, body) {
    const res = await fetch(path, {
      method: 'PUT',
      headers: apiHeaders(),
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text.trim() || t('ERR_HTTP', { status: res.status }));
    }
  }

  async function apiDelete(path) {
    var headers = {};
    if (state.pilotId) {
      headers['X-Pilot-ID'] = String(state.pilotId);
    }
    const res = await fetch(path, { method: 'DELETE', headers: headers });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text.trim() || t('ERR_HTTP', { status: res.status }));
    }
  }

  // ── LocalStorage persistence ──────────────────────────────────
  function saveState() {
    localStorage.setItem('skwad_session', state.sessionCode || '');
    localStorage.setItem('skwad_pilot', state.pilotId ? String(state.pilotId) : '');
  }

  function loadState() {
    state.sessionCode = localStorage.getItem('skwad_session') || null;
    var pid = localStorage.getItem('skwad_pilot');
    state.pilotId = pid ? parseInt(pid, 10) : null;
  }

  function clearState() {
    state.sessionCode = null;
    state.pilotId = null;
    state.myChannel = null;
    state.myFreqMHz = null;
    state.isLeader = false;
    state.leaderPilotId = null;
    state.expectingAssignmentChange = false;
    localStorage.removeItem('skwad_session');
    localStorage.removeItem('skwad_pilot');
  }

  // ── Recent Sessions ────────────────────────────────────────────
  var RECENT_SESSIONS_KEY = 'skwad_recent_sessions';
  var MAX_RECENT_SESSIONS = 10;

  function getRecentSessions() {
    try {
      return JSON.parse(localStorage.getItem(RECENT_SESSIONS_KEY)) || [];
    } catch (e) {
      return [];
    }
  }

  function saveRecentSession(code, pilotId, callsign) {
    var sessions = getRecentSessions();
    // Remove existing entry for this session code (if rejoining)
    sessions = sessions.filter(function (s) { return s.code !== code; });
    // Add to front
    sessions.unshift({ code: code, pilotId: pilotId, callsign: callsign });
    // Cap at max
    if (sessions.length > MAX_RECENT_SESSIONS) {
      sessions = sessions.slice(0, MAX_RECENT_SESSIONS);
    }
    localStorage.setItem(RECENT_SESSIONS_KEY, JSON.stringify(sessions));
  }

  function setRecentSessions(sessions) {
    localStorage.setItem(RECENT_SESSIONS_KEY, JSON.stringify(sessions));
  }

  // ── Channel pool lookup (mirrors Go logic) ────────────────────
  // Uses getEffectiveVideoSystem() so it works during setup (when
  // state.videoSystem is still the raw UI value like 'walksnail').
  function getChannelPool() {
    var sys = getEffectiveVideoSystem();
    var fcc = state.fccUnlocked;
    var bw = state.bandwidthMHz;
    var rm = state.raceMode;
    var goggles = state.goggles;

    switch (sys) {
      case 'analog':
        return mergeAnalogBands(state.analogBands);
      case 'hdzero':
        return CHANNELS.raceband;
      case 'dji_v1':
        return fcc ? CHANNELS.dji_v1_fcc : CHANNELS.dji_v1_stock;
      case 'dji_o3':
        if (bw >= 40) return fcc ? CHANNELS.dji_o3_40_fcc : CHANNELS.dji_o3_40_stock;
        return fcc ? CHANNELS.dji_o3_fcc : CHANNELS.dji_o3_stock;
      case 'dji_o4':
        if (rm && (goggles === 'goggles_3' || goggles === 'goggles_n3'))
          return CHANNELS.raceband;
        if (bw >= 60) return CHANNELS.dji_o4_60;
        if (bw >= 40) return fcc ? CHANNELS.dji_o4_40_fcc : CHANNELS.dji_o4_40_stock;
        return fcc ? CHANNELS.dji_o4_fcc : CHANNELS.dji_o4_stock;
      case 'walksnail_std':
        return fcc ? CHANNELS.dji_v1_fcc : CHANNELS.dji_v1_stock;
      case 'walksnail_race':
        return CHANNELS.raceband;
      case 'openipc':
        return CHANNELS.openipc;
      default:
        return CHANNELS.raceband;
    }
  }

  // ── Determine effective video system for API ──────────────────
  function getEffectiveVideoSystem() {
    if (state.videoSystem === 'walksnail') {
      return state.walksnailMode === 'race' ? 'walksnail_race' : 'walksnail_std';
    }
    return state.videoSystem;
  }

  // ── Landing Page ──────────────────────────────────────────────
  function initLanding() {
    $('btn-start').addEventListener('click', handleStartSession);
    $('btn-join').addEventListener('click', function () {
      $('join-input-area').classList.toggle('hidden');
      $('input-code').focus();
    });
    $('btn-go').addEventListener('click', handleJoinByCode);
    $('input-code').addEventListener('keydown', function (e) {
      if (e.key === 'Enter') handleJoinByCode();
    });
    // Auto uppercase the code input
    $('input-code').addEventListener('input', function (e) {
      e.target.value = e.target.value.toUpperCase().replace(/O/g, '0').replace(/I/g, '1').replace(/[^A-F0-9]/g, '');
    });

    // Fetch config to show/hide FPVFC link
    apiGet('/api/config').then(function (config) {
      if (config.show_fpvfc_link) {
        $('fpvfc-link').classList.remove('hidden');
        $('fpvfc-link-qr').classList.remove('hidden');
      }
    }).catch(function () {
      // Silently ignore — link stays hidden
    });

    // QR scanner — always show on devices with cameras (mobile)
    if (navigator.mediaDevices && navigator.mediaDevices.getUserMedia) {
      $('btn-scan-qr').classList.remove('hidden');
    }
    $('btn-scan-qr').addEventListener('click', openQRScanner);
    $('btn-scanner-close').addEventListener('click', closeQRScanner);

    // Feedback link on landing page
    $('link-feedback').addEventListener('click', function (e) {
      e.preventDefault();
      state.feedbackReturnTo = 'landing';
      openFeedbackScreen();
    });
  }

  var scannerStream = null;
  var scannerInterval = null;

  function openQRScanner() {
    $('qr-scanner').classList.remove('hidden');
    var video = $('qr-video');
    navigator.mediaDevices.getUserMedia({
      video: { facingMode: 'environment' }
    })
      .then(function (stream) {
        scannerStream = stream;
        video.srcObject = stream;
        video.play();
        startScanning(video);
      })
      .catch(function () {
        closeQRScanner();
        showError('landing-error', t('ERR_CAMERA_DENIED'));
      });
  }

  function startScanning(video) {
    var canvas = document.createElement('canvas');
    var ctx = canvas.getContext('2d', { willReadFrequently: true });

    var useNative = 'BarcodeDetector' in window;
    var detector = useNative ? new BarcodeDetector({ formats: ['qr_code'] }) : null;
    var scanning = false;
    var hasJsQR = typeof jsQR === 'function';
    var matched = false;

    scannerInterval = setInterval(function () {
      if (video.readyState < 2 || scanning || matched) return;
      var w = video.videoWidth;
      var h = video.videoHeight;
      if (!w || !h) return;

      // Scale down large frames for jsQR performance
      var scale = 1;
      if (!useNative && w > 640) scale = 640 / w;
      var sw = Math.round(w * scale);
      var sh = Math.round(h * scale);
      canvas.width = sw;
      canvas.height = sh;
      ctx.drawImage(video, 0, 0, sw, sh);

      if (useNative) {
        scanning = true;
        detector.detect(video).then(function (barcodes) {
          scanning = false;
          handleResult(barcodes);
        }).catch(function () { scanning = false; });
      } else if (hasJsQR) {
        var imageData = ctx.getImageData(0, 0, sw, sh);
        var qr = jsQR(imageData.data, sw, sh, { inversionAttempts: 'dontInvert' });
        if (qr) handleResult([{ rawValue: qr.data }]);
      }
    }, 300);

    function handleResult(barcodes) {
      if (!barcodes || barcodes.length === 0 || matched) return;
      var raw = (barcodes[0].rawValue || '').trim();
      if (raw) {
        matched = true;
        closeQRScanner();
        // QR alphanumeric mode uppercases URLs — normalize the path
        try {
          var parsed = new URL(raw);
          parsed.pathname = parsed.pathname.toLowerCase();
          window.location.href = parsed.href;
        } catch (e) {
          window.location.href = raw;
        }
      }
    }
  }

  function closeQRScanner() {
    $('qr-scanner').classList.add('hidden');
    if (scannerInterval) { clearInterval(scannerInterval); scannerInterval = null; }
    if (scannerStream) {
      scannerStream.getTracks().forEach(function (t) { t.stop(); });
      scannerStream = null;
    }
    $('qr-video').srcObject = null;
  }

  async function handleStartSession() {
    var btn = $('btn-start');
    setLoading(btn, true);
    hideError('landing-error');
    try {
      // Defer session creation until after the power ceiling step.
      state.isCreator = true;
      state.sessionCode = null;
      state.powerCeilingMW = 0;
      state.powerStepIndex = 2;
      state.optPowerCeiling = false;
      state.optFixedChannels = false;
      state.fixedChannels = '';
      state.pendingPowerMW = 0;
      $('opt-power-ceiling').classList.remove('active');
      $('opt-power-ceiling').querySelector('.session-option-check').textContent = '\u2610';
      $('opt-fixed-channels').classList.remove('active');
      $('opt-fixed-channels').querySelector('.session-option-check').textContent = '\u2610';
      $('joining-session-hint').classList.add('hidden');
      showScreen('setup');
      showStep('step-callsign');
      $('input-callsign').focus();
    } finally {
      setLoading(btn, false);
    }
  }

  // Creates the session with the pending power ceiling and fixed channels, then shows video step.
  async function createSessionWithPower(powerCeilingMW) {
    var body = {};
    if (powerCeilingMW > 0) body.power_ceiling_mw = powerCeilingMW;
    if (state.fixedChannels) body.fixed_channels = state.fixedChannels;
    var sess = await apiPost('/api/sessions', body);
    state.sessionCode = sess.id;
    state.powerCeilingMW = powerCeilingMW;
    saveState();
  }

  // Creates the session (collecting all wizard options) then shows the video step.
  // Used when power ceiling and/or fixed channels steps have been configured.
  async function createSessionAndShowVideo(btn) {
    setLoading(btn, true);
    try {
      await createSessionWithPower(state.pendingPowerMW > 0 ? state.pendingPowerMW : 0);
      showStep('step-video');
    } catch (err) {
      alert(t('ERR_CREATE_FAILED'));
    } finally {
      setLoading(btn, false);
    }
  }

  async function handleJoinByCode() {
    var code = $('input-code').value.trim().toUpperCase();
    if (code.length !== 6) {
      showError('landing-error', t('ERR_CODE_LENGTH'));
      return;
    }
    var btn = $('btn-go');
    setLoading(btn, true);
    hideError('landing-error');
    try {
      // Verify the session exists and capture power ceiling
      var data = await apiGet('/api/sessions/' + code);
      state.sessionCode = code;
      state.sessionPowerCeiling = (data.session && data.session.power_ceiling_mw) || 0;
      state.sessionFixedChannels = (data.session && data.session.fixed_channels) || '';
      saveState();
      $('joining-session-code').textContent = code;
      $('joining-session-hint').classList.remove('hidden');
      showScreen('setup');
      showStep('step-callsign');
      $('input-callsign').focus();
    } catch (err) {
      showError('landing-error', t('ERR_SESSION_NOT_FOUND'));
    } finally {
      setLoading(btn, false);
    }
  }

  // ── Setup: Step 1 — Callsign ──────────────────────────────────
  function initCallsignStep() {
    $('input-callsign').addEventListener('input', function (e) {
      e.target.value = e.target.value.toUpperCase();
    });
    $('btn-callsign-next').addEventListener('click', function () {
      var cs = $('input-callsign').value.trim();
      if (!cs) {
        showError('callsign-error', t('ERR_CALLSIGN_EMPTY'));
        return;
      }
      hideError('callsign-error');
      state.callsign = cs;
      if (state.isCreator) {
        showStep('step-leader-info');
      } else if (state.sessionPowerCeiling > 0) {
        // Restore power ceiling elements in case they were hidden by a previous fixed-only display
        var alertTitle = $('power-alert-title');
        var alertValueRow = $('power-alert-value-row');
        var alertText = $('power-alert-text');
        if (alertTitle) alertTitle.textContent = t('POWER_ALERT_TITLE');
        if (alertValueRow) alertValueRow.classList.remove('hidden');
        if (alertText) alertText.classList.remove('hidden');
        $('power-alert-mw').textContent = state.sessionPowerCeiling;
        $('power-alert-mw-bold').textContent = state.sessionPowerCeiling;
        // 600 mW = raceband cliff where channels drop from 8 to 4; below that, 20 MHz BW matters
        $('power-alert-dji-hint').classList.toggle('hidden', state.sessionPowerCeiling >= 600);
        // Show fixed channels info if set
        var fixedHint = $('power-alert-fixed-hint');
        if (fixedHint) {
          if (state.sessionFixedChannels) {
            try {
              var fcList = JSON.parse(state.sessionFixedChannels);
              var fcNames = fcList.map(function (c) { return c.name; });
              fixedHint.textContent = t('FIXED_CHANNELS_HINT', { channels: fcNames.join(', ') });
              fixedHint.classList.remove('hidden');
            } catch (e) { fixedHint.classList.add('hidden'); }
          } else {
            fixedHint.classList.add('hidden');
          }
        }
        showStep('step-power-alert');
      } else if (state.sessionFixedChannels) {
        // Fixed channels but no power ceiling — reuse the alert step with only the fixed hint
        alertTitle = $('power-alert-title');
        alertValueRow = $('power-alert-value-row');
        alertText = $('power-alert-text');
        if (alertTitle) alertTitle.textContent = t('SESSION_CHANNELS_TITLE');
        if (alertValueRow) alertValueRow.classList.add('hidden');
        if (alertText) alertText.classList.add('hidden');
        $('power-alert-dji-hint').classList.add('hidden');
        fixedHint = $('power-alert-fixed-hint');
        if (fixedHint) {
          try {
            fcList = JSON.parse(state.sessionFixedChannels);
            fcNames = fcList.map(function (c) { return c.name; });
            fixedHint.textContent = t('FIXED_CHANNELS_HINT', { channels: fcNames.join(', ') });
            fixedHint.classList.remove('hidden');
          } catch (e) { fixedHint.classList.add('hidden'); }
        }
        showStep('step-power-alert');
      } else {
        showStep('step-video');
      }
    });
    $('input-callsign').addEventListener('keydown', function (e) {
      if (e.key === 'Enter') $('btn-callsign-next').click();
    });
    $('btn-callsign-cancel').addEventListener('click', function () {
      validateAndShowLanding();
    });
  }

  // ── Setup: Step 1.5 — Leader Info ──────────────────────────────
  function initLeaderInfoStep() {
    $('btn-leader-info-got-it').addEventListener('click', async function () {
      if (state.optPowerCeiling) {
        // Power step will handle session creation after ceiling is chosen
        showStep('step-power');
      } else if (state.optFixedChannels) {
        // Fixed channels step will create the session after set is chosen
        showStep('step-fixed-channels');
      } else {
        // No optional steps — create session immediately then go to video
        await createSessionAndShowVideo(this);
      }
    });
    $('opt-power-ceiling').addEventListener('click', function () {
      state.optPowerCeiling = !state.optPowerCeiling;
      this.classList.toggle('active', state.optPowerCeiling);
      this.querySelector('.session-option-check').textContent = state.optPowerCeiling ? '\u2611' : '\u2610';
    });
    $('opt-fixed-channels').addEventListener('click', function () {
      state.optFixedChannels = !state.optFixedChannels;
      this.classList.toggle('active', state.optFixedChannels);
      this.querySelector('.session-option-check').textContent = state.optFixedChannels ? '\u2611' : '\u2610';
    });
  }

  // ── Setup: Step 1.75 — Power Ceiling (creators only) ──────────
  function initPowerStep() {
    // Build notches
    var notchContainer = $('power-slider-notches');
    POWER_STEPS.forEach(function () {
      var n = document.createElement('div');
      n.className = 'power-slider-notch';
      notchContainer.appendChild(n);
    });

    function updatePowerDisplay(idx) {
      state.powerStepIndex = idx;
      var step = POWER_STEPS[idx];
      var track = $('power-slider-track');
      var thumbRadius = 23;
      var usable = track.offsetWidth - thumbRadius * 2;
      var pct = idx / (POWER_STEPS.length - 1);
      $('power-slider-thumb').style.left = (thumbRadius - 23 + pct * usable) + 'px';
      $('power-mw-value').textContent = step.mw;
      $('power-guard-value').textContent = step.guard;
      $('power-tip').textContent = t(step.tipKey);
      renderPowerSpectrum(step.guard);
    }

    function getStepFromX(clientX) {
      var rect = $('power-slider-track').getBoundingClientRect();
      var pct = (clientX - rect.left) / rect.width;
      pct = Math.max(0, Math.min(1, pct));
      return Math.round(pct * (POWER_STEPS.length - 1));
    }

    $('power-slider-track').addEventListener('pointerdown', function (e) {
      e.preventDefault();
      this.setPointerCapture(e.pointerId);
      updatePowerDisplay(getStepFromX(e.clientX));
    });

    $('power-slider-track').addEventListener('pointermove', function (e) {
      if (this.hasPointerCapture && this.hasPointerCapture(e.pointerId)) {
        updatePowerDisplay(getStepFromX(e.clientX));
      }
    });

    $('btn-power-next').addEventListener('click', async function () {
      state.pendingPowerMW = POWER_STEPS[state.powerStepIndex].mw;
      if (state.optFixedChannels) {
        showStep('step-fixed-channels');
      } else {
        await createSessionAndShowVideo(this);
      }
    });

    $('btn-power-skip').addEventListener('click', async function () {
      state.pendingPowerMW = 0;
      if (state.optFixedChannels) {
        showStep('step-fixed-channels');
      } else {
        await createSessionAndShowVideo(this);
      }
    });

    // Set initial display after DOM is fully ready
    // (offsetWidth is 0 before the step is visible, so we defer to when shown)
  }

  // ── Setup: Step 1.85 — Fixed Channels (creators only) ─────────
  function initFixedChannelsStep() {
    var FC_COUNT_MIN = 2;
    var FC_COUNT_MAX = 5;
    var fcCount = 4;
    var selectedFCSet = null;

    // Build notches (4 gaps between 2..5 = 4 positions)
    var notchContainer = $('fc-slider-notches');
    var notchCount = FC_COUNT_MAX - FC_COUNT_MIN + 1; // 4 notches
    for (var ni = 0; ni < notchCount; ni++) {
      var n = document.createElement('div');
      n.className = 'power-slider-notch';
      notchContainer.appendChild(n);
    }

    function getFCCountFromX(clientX) {
      var rect = $('fc-slider-track').getBoundingClientRect();
      var pct = (clientX - rect.left) / rect.width;
      pct = Math.max(0, Math.min(1, pct));
      return Math.round(pct * (FC_COUNT_MAX - FC_COUNT_MIN)) + FC_COUNT_MIN;
    }

    function updateFCSlider(count) {
      fcCount = count;
      selectedFCSet = null;
      $('btn-fc-use-set').disabled = true;
      $('fc-pilot-count').textContent = count;
      var track = $('fc-slider-track');
      var thumbRadius = 19;
      var usable = track.offsetWidth - thumbRadius * 2;
      var pct = (count - FC_COUNT_MIN) / (FC_COUNT_MAX - FC_COUNT_MIN);
      $('fc-slider-thumb').style.left = (thumbRadius - 19 + pct * usable) + 'px';
      renderFCSetList(count);
    }

    function drawFCSpectrum(canvas, channels) {
      var ctx = canvas.getContext('2d');
      var dpr = window.devicePixelRatio || 1;
      var rect = canvas.getBoundingClientRect();
      var cw = rect.width;
      var canvasH = rect.height;
      canvas.width = cw * dpr;
      canvas.height = canvasH * dpr;
      ctx.scale(dpr, dpr);

      var fMin = 5620;
      var fMax = 5950;
      var fSpan = fMax - fMin;
      var baseline = canvasH - 2;

      ctx.fillStyle = '#1a1a1a';
      roundRect(ctx, 0, 0, cw, canvasH, 4);
      ctx.fill();

      ctx.strokeStyle = '#2a2a2a';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(4, baseline);
      ctx.lineTo(cw - 4, baseline);
      ctx.stroke();

      channels.forEach(function (c) {
        var x = ((c.f - fMin) / fSpan) * cw;
        var halfW = (20 / fSpan * cw) / 2;
        if (halfW < 8) halfW = 8;
        var peakY = 4;
        var color = c.dji ? '#60a5fa' : (c.multi ? '#f59e0b' : '#888888');

        ctx.beginPath();
        ctx.moveTo(x - halfW, baseline);
        ctx.bezierCurveTo(x - halfW * 0.5, baseline, x - halfW * 0.4, peakY, x, peakY);
        ctx.bezierCurveTo(x + halfW * 0.4, peakY, x + halfW * 0.5, baseline, x + halfW, baseline);
        ctx.closePath();
        ctx.fillStyle = color + '40';
        ctx.fill();
        ctx.strokeStyle = color;
        ctx.lineWidth = 1;
        ctx.stroke();
      });
    }

    function renderFCSetList(count) {
      var list = $('fc-set-list');
      clearChildren(list);
      var sets = FIXED_CHANNEL_SETS[count] || [];
      sets.forEach(function (set) {
        var card = document.createElement('div');
        card.className = 'fc-set-card';

        // Header
        var header = document.createElement('div');
        header.className = 'fc-set-header';
        var nameEl = document.createElement('span');
        nameEl.className = 'fc-set-name';
        var FC_NAME_KEYS = { 'MAX SPREAD': 'FC_SET_MAX_SPREAD', 'DJI SPREAD': 'FC_SET_DJI_SPREAD', 'IMD CLEAN': 'FC_SET_IMD_CLEAN', 'MIXED CLEAN': 'FC_SET_MIXED_CLEAN', 'MIXED OPTIMAL': 'FC_SET_MIXED_OPTIMAL', 'RACEBAND 5': 'FC_SET_RACEBAND_5', 'DJI 5': 'FC_SET_DJI_5', 'ET5A': 'FC_SET_ET5A' };
        nameEl.textContent = FC_NAME_KEYS[set.name] ? t(FC_NAME_KEYS[set.name]) : set.name;
        header.appendChild(nameEl);
        var badges = document.createElement('div');
        badges.className = 'fc-set-badges';
        var imdBadge = document.createElement('span');
        imdBadge.className = 'fc-set-badge';
        imdBadge.textContent = t('FC_IMD_BADGE', { score: set.imd });
        imdBadge.style.borderColor = set.imd >= 90 ? '#4ade8066' : (set.imd >= 60 ? '#f59e0b66' : '#f9713166');
        imdBadge.style.color = set.imd >= 90 ? '#4ade80' : (set.imd >= 60 ? '#f59e0b' : '#f97316');
        badges.appendChild(imdBadge);
        var powerBadge = document.createElement('span');
        powerBadge.className = 'fc-set-badge';
        powerBadge.textContent = set.power === 'ANY POWER' ? t('FC_POWER_ANY') : set.power;
        powerBadge.style.borderColor = set.powerColor + '66';
        powerBadge.style.color = set.powerColor;
        badges.appendChild(powerBadge);
        header.appendChild(badges);
        card.appendChild(header);

        // System availability — count usable channels per system type
        // Raceband freqs are usable by analog, HDZero, DJI Race Mode, Walksnail Race
        var rbFreqs = { 5658:1, 5695:1, 5732:1, 5769:1, 5806:1, 5843:1, 5880:1, 5917:1 };
        // DJI FCC 20 MHz standard freqs (non-race mode)
        var djiStdFreqs = { 5669:1, 5705:1, 5741:1, 5769:1, 5805:1, 5840:1, 5876:1 };
        var rbCount = 0, djiStdCount = 0;
        set.channels.forEach(function (ch) {
          if (rbFreqs[ch.f]) rbCount++;
          if (djiStdFreqs[ch.f]) djiStdCount++;
        });
        var sysText = '';
        if (rbCount === set.channels.length) {
          sysText = t('FC_SYSTEMS_ALL_RACEBAND', { count: set.channels.length });
        } else if (djiStdCount === set.channels.length) {
          sysText = t('FC_SYSTEMS_DJI_STANDARD', { count: djiStdCount });
        } else {
          var parts = [];
          if (rbCount > 0) parts.push(t('FC_SYSTEMS_RACEBAND_PARTIAL', { rCount: rbCount }));
          if (djiStdCount > 0) parts.push(t('FC_SYSTEMS_DJI_PARTIAL', { dCount: djiStdCount }));
          sysText = parts.join(' \u00b7 ');
        }
        var systemsEl = document.createElement('div');
        systemsEl.className = 'fc-set-systems';
        systemsEl.textContent = sysText;
        card.appendChild(systemsEl);

        // Channel pills
        var pillsEl = document.createElement('div');
        pillsEl.className = 'fc-set-channels';
        set.channels.forEach(function (ch) {
          var pill = document.createElement('span');
          pill.className = 'fc-ch-pill' + (ch.dji ? ' ch-dji' : '') + (ch.multi ? ' ch-multi' : '');
          pill.textContent = ch.n;
          pillsEl.appendChild(pill);
        });
        card.appendChild(pillsEl);

        // Mini spectrum canvas
        var canvas = document.createElement('canvas');
        canvas.className = 'fc-set-spectrum';
        canvas.height = 32;
        card.appendChild(canvas);

        // Click to select
        card.addEventListener('click', function () {
          document.querySelectorAll('.fc-set-card').forEach(function (c) { c.classList.remove('selected'); });
          card.classList.add('selected');
          selectedFCSet = set;
          $('btn-fc-use-set').disabled = false;
        });

        list.appendChild(card);

        // Draw spectrum after insertion
        requestAnimationFrame(function () {
          canvas.width = canvas.offsetWidth || 300;
          drawFCSpectrum(canvas, set.channels);
        });
      });
    }

    $('fc-slider-track').addEventListener('pointerdown', function (e) {
      e.preventDefault();
      this.setPointerCapture(e.pointerId);
      updateFCSlider(getFCCountFromX(e.clientX));
    });

    $('fc-slider-track').addEventListener('pointermove', function (e) {
      if (this.hasPointerCapture && this.hasPointerCapture(e.pointerId)) {
        updateFCSlider(getFCCountFromX(e.clientX));
      }
    });

    $('btn-fc-use-set').addEventListener('click', async function () {
      if (!selectedFCSet) return;
      state.fixedChannels = JSON.stringify(selectedFCSet.channels.map(function (c) {
        return { name: c.n, freq: c.f };
      }));
      await createSessionAndShowVideo(this);
    });

    $('btn-fc-skip').addEventListener('click', async function () {
      state.fixedChannels = '';
      await createSessionAndShowVideo(this);
    });

    // showStep hook: initialize slider display when step becomes visible
    var origShowStep = showStep;
    showStep = function (stepId) {
      origShowStep(stepId);
      if (stepId === 'step-fixed-channels') {
        // Reset selection
        selectedFCSet = null;
        $('btn-fc-use-set').disabled = true;
        // Defer thumb positioning until track has layout
        requestAnimationFrame(function () {
          updateFCSlider(fcCount);
        });
      }
    };
  }

  // ── Setup: Step 1.25 — Power Ceiling Alert (joiners) ──────────
  function initPowerAlertStep() {
    $('btn-power-alert-ok').addEventListener('click', function () {
      showStep('step-video');
    });
  }

  // ── Setup: Step 2 — Video System ──────────────────────────────
  function initVideoStep() {
    document.querySelectorAll('.btn-system').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var sys = btn.dataset.system;
        state.videoSystem = sys;
        // Reset follow-up state
        state.fccUnlocked = false;
        state.goggles = '';
        state.bandwidthMHz = 0;
        state.raceMode = false;
        state.walksnailMode = '';
        startFollowUpFlow(sys);
      });
    });
    $('btn-video-cancel').addEventListener('click', function () {
      if (state._changingVideoSystem) { cancelVideoSystemChange(); return; }
      validateAndShowLanding();
    });
  }

  // ── Setup: Step 3 — Follow-ups (conditional) ──────────────────
  function startFollowUpFlow(system) {
    // Hide all follow-up groups
    document.querySelectorAll('.followup-group').forEach(function (g) { g.classList.add('hidden'); });
    $('btn-followup-next').classList.add('hidden');
    resetOptionButtons();

    // Systems with no follow-ups go straight to channel pref
    if (['hdzero', 'openipc', 'spotter'].includes(system)) {
      goToChannelStep();
      return;
    }

    showStep('step-followup');

    if (system === 'analog') {
      $('followup-title').textContent = t('FOLLOWUP_TITLE_ANALOG');
      $('followup-analog-bands').classList.remove('hidden');
      $('btn-followup-next').classList.remove('hidden');
      state.analogBands = ['R'];
      document.querySelectorAll('.analog-band-btn').forEach(function (b) {
        b.classList.toggle('selected', b.dataset.band === 'R');
      });
    } else if (system === 'walksnail') {
      $('followup-title').textContent = t('FOLLOWUP_TITLE_WALKSNAIL');
      $('followup-walksnail-mode').classList.remove('hidden');
    } else if (system === 'dji_v1') {
      $('followup-title').textContent = t('FOLLOWUP_TITLE_DJI_V1');
      $('followup-fcc').classList.remove('hidden');
    } else if (system === 'dji_o3') {
      $('followup-title').textContent = t('FOLLOWUP_TITLE_DJI_O3');
      $('followup-fcc').classList.remove('hidden');
    } else if (system === 'dji_o4') {
      $('followup-title').textContent = t('FOLLOWUP_TITLE_DJI_O4');
      $('followup-fcc').classList.remove('hidden');
    }
  }

  function resetOptionButtons() {
    document.querySelectorAll('.btn-option').forEach(function (b) { b.classList.remove('selected'); });
  }

  function initFollowUpStep() {
    // FCC buttons
    document.querySelectorAll('[data-fcc]').forEach(function (btn) {
      btn.addEventListener('click', function () {
        document.querySelectorAll('[data-fcc]').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        state.fccUnlocked = btn.dataset.fcc === 'true';
        handleFccSelected();
      });
    });

    // Goggles buttons
    document.querySelectorAll('[data-goggles]').forEach(function (btn) {
      btn.addEventListener('click', function () {
        document.querySelectorAll('[data-goggles]').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        state.goggles = btn.dataset.goggles;
        handleGogglesSelected();
      });
    });

    // Race mode buttons
    document.querySelectorAll('[data-racemode]').forEach(function (btn) {
      btn.addEventListener('click', function () {
        document.querySelectorAll('[data-racemode]').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        state.raceMode = btn.dataset.racemode === 'true';
        $('btn-followup-next').classList.remove('hidden');
      });
    });

    // Walksnail mode buttons
    document.querySelectorAll('[data-wsmode]').forEach(function (btn) {
      btn.addEventListener('click', function () {
        document.querySelectorAll('[data-wsmode]').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        state.walksnailMode = btn.dataset.wsmode;
        handleWalksnailModeSelected();
      });
    });

    // Analog band toggle buttons
    document.querySelectorAll('.analog-band-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        btn.classList.toggle('selected');
        var selected = [];
        document.querySelectorAll('.analog-band-btn.selected').forEach(function (b) {
          selected.push(b.dataset.band);
        });
        if (selected.length === 0) {
          btn.classList.add('selected');
          selected.push(btn.dataset.band);
        }
        state.analogBands = selected;
      });
    });

    // "Not sure? Just use Race Band" helper
    $('analog-bands-helper').addEventListener('click', function () {
      document.querySelectorAll('.analog-band-btn').forEach(function (b) {
        b.classList.toggle('selected', b.dataset.band === 'R');
      });
      state.analogBands = ['R'];
    });

    // Next button for follow-up step
    $('btn-followup-next').addEventListener('click', goToChannelStep);
    $('btn-followup-cancel').addEventListener('click', function () {
      if (state._changingVideoSystem) { cancelVideoSystemChange(); return; }
      validateAndShowLanding();
    });
  }

  function handleFccSelected() {
    if (state.videoSystem === 'dji_v1') {
      // DJI V1 just needs FCC — show next button
      $('btn-followup-next').classList.remove('hidden');
    } else if (state.videoSystem === 'dji_o3') {
      // DJI O3 needs bandwidth selection
      showBandwidthOptions([10, 20, 40]);
    } else if (state.videoSystem === 'dji_o4') {
      // DJI O4 needs goggles selection
      $('followup-goggles').classList.remove('hidden');
    }
  }

  function handleGogglesSelected() {
    // After goggles, ask bandwidth
    showBandwidthOptions([10, 20, 40, 60]);
  }

  function handleBandwidthSelected(bw) {
    state.bandwidthMHz = bw;

    if (state.videoSystem === 'dji_o4') {
      // Check if Race Mode is available (Goggles 3 or N3 + FCC)
      if (
        state.fccUnlocked &&
        (state.goggles === 'goggles_3' || state.goggles === 'goggles_n3')
      ) {
        $('followup-racemode').classList.remove('hidden');
      } else {
        $('btn-followup-next').classList.remove('hidden');
      }
    } else {
      // DJI O3 — done with follow-ups
      $('btn-followup-next').classList.remove('hidden');
    }
  }

  function shouldWarnBandwidth() {
    var ceiling = state.sessionPowerCeiling || state.powerCeilingMW;
    return ceiling > 0 && ceiling < 600;
  }

  function applyBandwidthHint(btn, bw) {
    if (!shouldWarnBandwidth()) return;
    if (bw <= 20) {
      btn.classList.add('bw-recommended');
      btn.setAttribute('data-label', t('BW_RECOMMENDED'));
    } else {
      btn.classList.add('bw-warn');
    }
  }

  function showBandwidthOptions(options) {
    $('followup-bandwidth').classList.remove('hidden');
    var container = $('bandwidth-buttons');
    clearChildren(container);
    options.forEach(function (bw) {
      var btn = document.createElement('button');
      btn.className = 'btn btn-option';
      btn.textContent = t('BW_MHZ', { bw: bw });
      applyBandwidthHint(btn, bw);
      btn.addEventListener('click', function () {
        container.querySelectorAll('.btn-option').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        handleBandwidthSelected(bw);
      });
      container.appendChild(btn);
    });
  }

  function handleWalksnailModeSelected() {
    if (state.walksnailMode === 'race') {
      // Walksnail race mode — no more questions
      $('btn-followup-next').classList.remove('hidden');
    } else {
      // Standard mode — ask FCC
      $('followup-fcc').classList.remove('hidden');
      // Show next button for walksnail standard after FCC selection
      $('btn-followup-next').classList.remove('hidden');
    }
  }

  // ── In-session video system change ──────────────────────────
  async function submitVideoSystemChange() {
    var btn = $('btn-join-session');
    setLoading(btn, true);
    state.expectingAssignmentChange = true;
    state._changingVideoSystem = false;
    try {
      await apiPut('/api/pilots/' + state.pilotId + '/video-system?session=' + state.sessionCode, {
        video_system: getEffectiveVideoSystem(),
        fcc_unlocked: state.fccUnlocked,
        goggles: state.goggles,
        bandwidth_mhz: state.bandwidthMHz,
        race_mode: state.raceMode,
        analog_bands: state.analogBands,
        preferred_frequency_mhz: state.preferredFreqMHz,
      });
    } catch (err) {
      showError('join-error', t('ERR_UPDATE_FAILED', { error: (err.message || '').toUpperCase() }));
      setLoading(btn, false);
      return;
    }
    setLoading(btn, false);
    enterSessionView();
  }

  function cancelVideoSystemChange() {
    state._changingVideoSystem = false;
    enterSessionView();
  }

  // ── Setup: Step 4 — Channel Preference ────────────────────────
  var previewPilots = []; // Cached pilots for spectrum preview on channel step

  function goToChannelStep() {
    if (state.videoSystem === 'spotter') {
      state.preferredFreqMHz = 0;
      showStep('step-channel');
      updateJoinButtonState();
      return;
    }
    state.preferredFreqMHz = 0;
    showStep('step-channel');
    $('btn-auto-assign').classList.add('active');
    $('btn-have-preference').classList.remove('active');
    $('channel-picker').classList.add('hidden');
    $('spectrum-preview').classList.add('hidden');
    var hint = $('preference-hint');
    if (hint) hint.style.display = 'none';
    $('btn-join-session').textContent = state._changingVideoSystem ? t('BTN_UPDATE') : t('BTN_JOIN');
    renderChannelPicker();
    updateJoinButtonState();
    // Pre-fetch existing pilots for spectrum preview
    fetchPreviewPilots();
  }

  function fetchPreviewPilots() {
    if (!state.sessionCode) { previewPilots = []; return; }
    apiGet('/api/sessions/' + state.sessionCode).then(function (data) {
      previewPilots = (data.pilots || []).filter(function (p) { return p.Active; });
    }).catch(function () { previewPilots = []; });
  }

  function renderSpectrumPreview() {
    var sys = getEffectiveVideoSystem();
    var bw = occupiedBandwidth(sys, state.bandwidthMHz);
    var freq = state.preferredFreqMHz || 0;
    // Include hypothetical self in pilot list for IMD preview
    var pilotsForIMD = previewPilots;
    if (freq > 0) {
      pilotsForIMD = previewPilots.concat([{ AssignedFreqMHz: freq, VideoSystem: sys, BandwidthMHz: bw, Callsign: t('SPECTRUM_YOU'), ID: -1 }]);
    }
    renderSpectrum(pilotsForIMD, 'spectrum-preview', freq, bw);
  }

  function updateJoinButtonState() {
    var btn = $('btn-join-session');
    var prefActive = $('btn-have-preference').classList.contains('active');
    btn.disabled = prefActive && !state.preferredFreqMHz;
  }

  function initChannelStep() {
    $('btn-auto-assign').addEventListener('click', function () {
      state.preferredFreqMHz = 0;
      $('btn-auto-assign').classList.add('active');
      $('btn-have-preference').classList.remove('active');
      $('channel-picker').classList.add('hidden');
      $('spectrum-preview').classList.add('hidden');
      $('preference-hint').style.display = 'none';
      // Deselect any selected channel
      document.querySelectorAll('.btn-channel').forEach(function (b) { b.classList.remove('selected'); });
      updateJoinButtonState();
    });

    $('btn-have-preference').addEventListener('click', function () {
      $('btn-have-preference').classList.add('active');
      $('btn-auto-assign').classList.remove('active');
      $('channel-picker').classList.remove('hidden');
      $('spectrum-preview').classList.remove('hidden');
      $('preference-hint').style.display = '';
      renderSpectrumPreview();
      updateJoinButtonState();
    });

    $('btn-join-session').addEventListener('click', handleJoinSession);
    $('btn-channel-back').addEventListener('click', function () {
      showStep('step-video');
    });
  }

  function fitText(elem, maxPx, minPx) {
    // After insertion, shrink font until text fits on one line.
    requestAnimationFrame(function () {
      var size = maxPx;
      elem.style.fontSize = size + 'px';
      while (elem.scrollWidth > elem.clientWidth && size > minPx) {
        size -= 1;
        elem.style.fontSize = size + 'px';
      }
    });
  }

  function adaptPickerGrid(picker) {
    var count = picker.children.length;
    var cols = count <= 3 ? count : 4;
    picker.style.gridTemplateColumns = 'repeat(' + cols + ', 1fr)';
  }

  function filterPoolToFixedChannels(pool) {
    var fcStr = state.sessionFixedChannels || state.fixedChannels;
    if (!fcStr) return pool;
    try {
      var fixedChannels = JSON.parse(fcStr);
      var fixedFreqs = {};
      fixedChannels.forEach(function (c) { fixedFreqs[c.freq] = true; });
      var filtered = pool.filter(function (ch) { return fixedFreqs[ch.freq]; });
      if (filtered.length > 0) return filtered;
      // Pilot's system has no overlap — show fixed channels directly
      return fixedChannels.map(function (c) { return { name: c.name, freq: c.freq }; });
    } catch (e) { return pool; }
  }

  function renderChannelPicker() {
    var pool = getChannelPool();
    pool = filterPoolToFixedChannels(pool);
    var picker = $('channel-picker');
    clearChildren(picker);

    // Count pilots per frequency for fixed session buddy info
    var isFixedSession = !!(state.sessionFixedChannels || state.fixedChannels);
    var freqPilotCount = {};
    if (isFixedSession && previewPilots) {
      previewPilots.forEach(function (p) {
        if (p.Active && p.AssignedFreqMHz) {
          freqPilotCount[p.AssignedFreqMHz] = (freqPilotCount[p.AssignedFreqMHz] || 0) + 1;
        }
      });
    }

    pool.forEach(function (ch) {
      var nameSpan = el('span', { className: 'ch-name', textContent: ch.name });
      var countOnChannel = freqPilotCount[ch.freq] || 0;
      var freqLabel = String(ch.freq);
      if (isFixedSession && countOnChannel > 0) {
        freqLabel += ' (' + countOnChannel + ')';
      }
      var freqSpan = el('span', { className: 'ch-freq', textContent: freqLabel });
      var btn = el('button', { className: 'btn-channel' }, [nameSpan, freqSpan]);
      btn.addEventListener('click', function () {
        picker.querySelectorAll('.btn-channel').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        state.preferredFreqMHz = ch.freq;
        renderSpectrumPreview();
        updateJoinButtonState();
      });
      picker.appendChild(btn);
    });
    adaptPickerGrid(picker);
  }

  function buildJoinBody() {
    return {
      callsign: state.callsign,
      video_system: getEffectiveVideoSystem(),
      fcc_unlocked: state.fccUnlocked,
      goggles: state.goggles,
      bandwidth_mhz: state.bandwidthMHz,
      race_mode: state.raceMode,
      preferred_frequency_mhz: state.preferredFreqMHz,
      analog_bands: state.analogBands,
    };
  }

  async function handleJoinSession() {
    hideError('join-error');
    if (state._changingVideoSystem) { submitVideoSystemChange(); return; }

    var btn = $('btn-join-session');
    setLoading(btn, true);

    var body = buildJoinBody();

    try {
      // Preview first to check for displacements.
      var preview = await apiPost('/api/sessions/' + state.sessionCode + '/preview-join', body);
      var level = preview.level || 0;

      if (level === 0 && preview.override_reason) {
        // Preference was overridden — show GOT IT dialog.
        setLoading(btn, false);
        showOverrideDialog(preview.override_reason, preview.assignment, function () {
          commitJoin(body);
        });
        return;
      }

      if (level === 1) {
        // No clean channel — show buddy/rebalance choice.
        setLoading(btn, false);
        showChoiceDialog(preview,
          function onBuddy(buddy) {
            body.preferred_frequency_mhz = buddy.freq_mhz;
            body.choice = 'buddy';
            commitJoin(body);
          },
          function onRebalance() {
            body.choice = 'rebalance';
            commitJoin(body);
          },
          function onCancel() {
            // Do nothing, user stays on wizard.
          }
        );
        return;
      }

      // Level 0, no override — join immediately.
      await commitJoin(body);
    } catch (err) {
      var msg = err.message || '';
      if (msg.includes('callsign already')) {
        showError('join-error', t('ERR_CALLSIGN_IN_SESSION'));
      } else {
        showError('join-error', t('ERR_JOIN_FAILED', { error: msg.toUpperCase() }));
      }
    } finally {
      setLoading(btn, false);
    }
  }

  async function commitJoin(body) {
    var btn = $('btn-join-session');
    setLoading(btn, true);
    try {
      var pilot = await apiPost('/api/sessions/' + state.sessionCode + '/join', body);
      state.pilotId = pilot.ID;
      saveState();
      saveRecentSession(state.sessionCode, pilot.ID, state.callsign);
      enterSessionView();
    } catch (err) {
      var msg = err.message || '';
      if (msg.includes('callsign already')) {
        showError('join-error', t('ERR_CALLSIGN_IN_SESSION'));
      } else {
        showError('join-error', t('ERR_JOIN_FAILED', { error: msg.toUpperCase() }));
      }
    } finally {
      setLoading(btn, false);
    }
  }

  // ── Override Dialog (preference overridden, GOT IT) ──────────
  function showOverrideDialog(overrideReason, assignment, onConfirm) {
    $('override-reason').textContent = overrideReason;
    $('override-dialog').style.display = '';

    if (assignment) {
      var bw = assignment.bandwidth_mhz || 20;
      renderSpectrum([], 'override-spectrum', assignment.freq_mhz, bw);
    }

    $('btn-override-ok').onclick = function () {
      $('override-dialog').style.display = 'none';
      if (onConfirm) onConfirm();
    };
  }

  // ── Choice Dialog (Level 1 — buddy or rebalance) ──────────
  function showChoiceDialog(preview, onBuddy, onRebalance, onCancel) {
    var options = $('choice-options');
    while (options.firstChild) {
      options.removeChild(options.firstChild);
    }

    if (preview.buddy_option) {
      var buddyBtn = document.createElement('button');
      buddyBtn.className = 'btn btn-secondary btn-large buddy-choice-btn';
      var strong1 = document.createElement('strong');
      strong1.textContent = t('CHOICE_BUDDY_UP');
      buddyBtn.appendChild(strong1);
      buddyBtn.appendChild(document.createElement('br'));
      buddyBtn.appendChild(document.createTextNode(
        t('CHOICE_BUDDY_SHARE', { channel: preview.buddy_option.channel, freq: preview.buddy_option.freq_mhz, callsign: preview.buddy_option.callsign })
      ));
      buddyBtn.onclick = function () {
        $('choice-dialog').style.display = 'none';
        onBuddy(preview.buddy_option);
      };
      options.appendChild(buddyBtn);
    }

    if (preview.rebalance_option) {
      var rebalBtn = document.createElement('button');
      rebalBtn.className = 'btn btn-secondary btn-large rebalance-choice-btn';
      var strong2 = document.createElement('strong');
      strong2.textContent = t('CHOICE_PARTIAL_REBALANCE');
      rebalBtn.appendChild(strong2);
      rebalBtn.appendChild(document.createElement('br'));
      var movedText = preview.rebalance_option.displaced.map(function (d) {
        return t('CHOICE_MOVE', { callsign: d.callsign, oldChannel: d.old_channel, newChannel: d.new_channel });
      }).join(', ');
      rebalBtn.appendChild(document.createTextNode(
        movedText + '. ' + t('CHOICE_YOU_GET', { channel: preview.rebalance_option.assignment.channel, freq: preview.rebalance_option.assignment.freq_mhz })
      ));
      rebalBtn.onclick = function () {
        $('choice-dialog').style.display = 'none';
        onRebalance();
      };
      options.appendChild(rebalBtn);
    }

    $('btn-choice-cancel').onclick = function () {
      $('choice-dialog').style.display = 'none';
      if (onCancel) onCancel();
    };

    $('choice-dialog').style.display = '';
  }




  // ── Session View ──────────────────────────────────────────────
  function enterSessionView() {
    showScreen('session');
    $('session-code-text').textContent = state.sessionCode;
    refreshSession();
    startPolling();
  }

  async function refreshSession() {
    try {
      var data = await apiGet('/api/sessions/' + state.sessionCode);
      state.knownVersion = data.session.version;

      // Track leader state.
      state.leaderPilotId = data.session.leader_pilot_id || null;
      state.isLeader = (state.pilotId && state.leaderPilotId === state.pilotId);

      // If session has no pilots or our pilot isn't in it, abandon it.
      if (!data.pilots || data.pilots.length === 0) {
        clearState();
        stopPolling();
        validateAndShowLanding();
        return;
      }

      var foundSelf = false;
      if (state.pilotId) {
        for (var j = 0; j < data.pilots.length; j++) {
          if (data.pilots[j].ID === state.pilotId) {
            foundSelf = true;

            // Detect externally-caused assignment change (partial rebalance moved us)
            if (state.myFreqMHz && state.myFreqMHz !== data.pilots[j].AssignedFreqMHz) {
              if (!state.expectingAssignmentChange) {
                showMovedDialog(
                  data.pilots[j].AssignedChannel,
                  data.pilots[j].AssignedFreqMHz,
                  data.session.leader_pilot_id,
                  data.pilots
                );
              }
            }
            state.expectingAssignmentChange = false;

            state.myChannel = data.pilots[j].AssignedChannel;
            state.myFreqMHz = data.pilots[j].AssignedFreqMHz;
            state.preferredFreqMHz = data.pilots[j].PreferredFreqMHz || 0;
            if (!state.callsign) state.callsign = data.pilots[j].Callsign;
            if (!state.videoSystem) state.videoSystem = data.pilots[j].VideoSystem;
            // Sync gear settings so channel picker works after page refresh.
            state.fccUnlocked = data.pilots[j].FCCUnlocked || false;
            state.bandwidthMHz = data.pilots[j].BandwidthMHz || 0;
            state.goggles = data.pilots[j].Goggles || '';
            state.raceMode = data.pilots[j].RaceMode || false;
            state.analogBands = data.pilots[j].AnalogBands ? data.pilots[j].AnalogBands.split(',') : ['R'];
            break;
          }
        }
      }

      if (!foundSelf) {
        clearState();
        stopPolling();
        validateAndShowLanding();
        return;
      }

      state.cachedPilots = data.pilots;
      renderPilotList(data.pilots);
      updateLeaderControls();

      // Rebalance recommended indicator (Task 20)
      var rebalHint = $('rebalance-hint');
      if (rebalHint) {
        if (data.rebalance_recommended && state.pilotId === data.session.leader_pilot_id) {
          rebalHint.style.display = '';
        } else {
          rebalHint.style.display = 'none';
        }
      }

      // Power ceiling badge
      state.sessionPowerCeiling = data.session.power_ceiling_mw || 0;
      state.sessionFixedChannels = data.session.fixed_channels || '';
      var badge = $('power-ceiling-badge');
      if (badge) {
        if (state.sessionPowerCeiling > 0) {
          badge.textContent = t('POWER_BADGE', { power: state.sessionPowerCeiling });
          badge.classList.remove('hidden');
        } else {
          badge.classList.add('hidden');
        }
      }

      // Fixed channels badge
      var fcBadge = $('fixed-channels-badge');
      if (fcBadge && state.sessionFixedChannels) {
        try {
          var fcParsed = JSON.parse(state.sessionFixedChannels);
          fcBadge.textContent = t('FIXED_BADGE', { count: fcParsed.length });
          fcBadge.classList.remove('hidden');
        } catch (e) { fcBadge.classList.add('hidden'); }
      } else if (fcBadge) {
        fcBadge.classList.add('hidden');
      }

      // IMD badge
      var imdBadge = $('imd-badge');
      if (imdBadge && data.pilots.length >= 2) {
        var imdScore = calcIMDScore(data.pilots);
        imdBadge.textContent = t('IMD_BADGE', { score: imdScore });
        imdBadge.className = 'imd-badge ' + (imdScore >= 80 ? 'imd-good' : imdScore >= 50 ? 'imd-fair' : 'imd-poor');
        imdBadge.classList.remove('hidden');
      } else if (imdBadge) {
        imdBadge.classList.add('hidden');
      }
    } catch (err) {
      // Session may have expired or been deleted
      if (err.message && err.message.includes('not found')) {
        clearState();
        stopPolling();
        validateAndShowLanding();
      }
    }
  }

  // ── Channel Change Banner ──────────────────────────────────
  function showChannelChangeBanner(oldChannel, oldFreq, newChannel, newFreq) {
    var msg = t('BANNER_CHANNEL_CHANGED', { oldChannel: oldChannel, oldFreq: oldFreq, newChannel: newChannel, newFreq: newFreq }) +
      '\n' + t('BANNER_COORDINATE');
    $('banner-message').textContent = msg;
    $('channel-change-banner').classList.remove('hidden');
  }

  function hideChannelChangeBanner() {
    $('channel-change-banner').classList.add('hidden');
  }

  function initChannelChangeBanner() {
    $('btn-banner-dismiss').addEventListener('click', hideChannelChangeBanner);
  }

  // ── Spectrum Visualization ──────────────────────────────────
  function occupiedBandwidth(videoSystem, bandwidthMHz) {
    if (videoSystem === 'dji_o3' || videoSystem === 'dji_o4') {
      if (bandwidthMHz === 40) return 40;
      if (bandwidthMHz === 60) return 60;
      return 20;
    }
    return 20;
  }

  function renderSpectrum(pilots, canvasOrId, highlightFreq, highlightBw) {
    var canvas = typeof canvasOrId === 'string' ? $(canvasOrId) : (canvasOrId || $('spectrum-canvas'));
    if (!canvas) return;
    var ctx = canvas.getContext('2d');
    var dpr = window.devicePixelRatio || 1;
    var rect = canvas.getBoundingClientRect();
    var w = rect.width * dpr;
    var ch = rect.height || 120;
    var h = ch * dpr;
    canvas.width = w;
    canvas.height = h;
    ctx.scale(dpr, dpr);
    var cw = rect.width;

    // Frequency range — dynamically expand beyond Race Band if any pilot
    // is assigned to Fatshark, Boscam E, or Low Race frequencies.
    var fMin = 5640;
    var fMax = 5930;
    if (pilots && pilots.length > 0) {
      pilots.forEach(function (p) {
        if (!p.AssignedFreqMHz) return;
        var bw = occupiedBandwidth(p.VideoSystem, p.BandwidthMHz);
        var lo = p.AssignedFreqMHz - bw / 2 - 20;
        var hi = p.AssignedFreqMHz + bw / 2 + 20;
        if (lo < fMin) fMin = lo;
        if (hi > fMax) fMax = hi;
      });
    }
    var fSpan = fMax - fMin;

    // Background track
    ctx.fillStyle = '#1a1a1a';
    roundRect(ctx, 0, 0, cw, ch, 8);
    ctx.fill();

    // Baseline — leave room below for channel labels
    var baseline = ch - 22;
    ctx.strokeStyle = '#2a2a2a';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(8, baseline);
    ctx.lineTo(cw - 8, baseline);
    ctx.stroke();

    // Race Band tick marks: names above baseline, frequencies below
    var rbFreqs = [5658, 5695, 5732, 5769, 5806, 5843, 5880, 5917];
    var rbNames = ['R1', 'R2', 'R3', 'R4', 'R5', 'R6', 'R7', 'R8'];
    for (var t = 0; t < rbFreqs.length; t++) {
      var tx = (rbFreqs[t] - fMin) / fSpan * cw;
      // Tick mark
      ctx.strokeStyle = '#888888';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(tx, baseline - 4);
      ctx.lineTo(tx, baseline + 4);
      ctx.stroke();
      // Channel name just above baseline (alphabetic baseline for even alignment)
      ctx.font = '800 11px -apple-system, BlinkMacSystemFont, sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'alphabetic';
      ctx.fillStyle = '#bbbbbb';
      ctx.fillText(rbNames[t], tx, baseline - 5);
      // Frequency below baseline (bigger)
      ctx.font = '600 10px -apple-system, BlinkMacSystemFont, sans-serif';
      ctx.textBaseline = 'top';
      ctx.fillStyle = '#666666';
      ctx.fillText(String(rbFreqs[t]), tx, baseline + 5);
    }

    if (!pilots || pilots.length === 0) {
      drawHighlightHump(ctx, highlightFreq, highlightBw, fMin, fSpan, cw, baseline, 44);
      return;
    }

    // Pre-compute each pilot's layout data
    var items = [];
    ctx.font = '700 10px -apple-system, BlinkMacSystemFont, sans-serif';
    pilots.forEach(function (p) {
      if (!p.AssignedFreqMHz) return;
      var bw = occupiedBandwidth(p.VideoSystem, p.BandwidthMHz);
      var centerX = (p.AssignedFreqMHz - fMin) / fSpan * cw;
      var halfW = (bw / fSpan * cw) / 2;
      if (halfW < 14) halfW = 14;

      var conflicts = p.Conflicts || p.conflicts || [];
      var worstLevel = null;
      conflicts.forEach(function (c) {
        if (c.level === 'danger' || c.Level === 'danger') worstLevel = 'danger';
        else if (!worstLevel && (c.level === 'warning' || c.Level === 'warning')) worstLevel = 'warning';
      });

      var isMe = p.ID === state.pilotId;
      var color;
      if (isMe) color = '#33ff33';
      else if (worstLevel === 'danger') color = '#ff3333';
      else if (worstLevel === 'warning') color = '#ffaa00';
      else color = '#888888';

      var label = p.Callsign || '';
      var labelW = ctx.measureText(label).width;

      items.push({
        centerX: centerX,
        halfW: halfW,
        color: color,
        isMe: isMe,
        label: label,
        labelW: labelW,
        labelLeft: centerX - labelW / 2,
        labelRight: centerX + labelW / 2
      });
    });

    // Sort by frequency for stagger calculation
    items.sort(function (a, b) { return a.centerX - b.centerX; });

    // Assign vertical tiers to labels to avoid overlap
    var labelH = 13;
    var labelPad = 4;
    var humpPeakH = 44;
    var humpPeakY = baseline - humpPeakH;
    var tierBaseY = humpPeakY - 3; // Y for tier 0 (closest to hump)
    items.forEach(function (item, i) {
      var tier = 0;
      // Check against all previous items for horizontal overlap
      for (var j = 0; j < i; j++) {
        var prev = items[j];
        // Do the labels overlap horizontally?
        if (item.labelLeft - labelPad < prev.labelRight + labelPad &&
            item.labelRight + labelPad > prev.labelLeft - labelPad) {
          // Need to be on a different tier than prev
          if (tier <= prev.tier) tier = prev.tier + 1;
        }
      }
      item.tier = tier;
      item.labelY = Math.max(labelH, tierBaseY - tier * (labelH + 2));
    });

    // Draw waveform humps
    items.forEach(function (item) {
      ctx.beginPath();
      ctx.moveTo(item.centerX - item.halfW, baseline);
      ctx.bezierCurveTo(
        item.centerX - item.halfW * 0.5, baseline,
        item.centerX - item.halfW * 0.4, humpPeakY,
        item.centerX, humpPeakY
      );
      ctx.bezierCurveTo(
        item.centerX + item.halfW * 0.4, humpPeakY,
        item.centerX + item.halfW * 0.5, baseline,
        item.centerX + item.halfW, baseline
      );
      ctx.closePath();
      ctx.fillStyle = item.color + '40';
      ctx.fill();
      ctx.strokeStyle = item.color;
      ctx.lineWidth = 1.5;
      ctx.stroke();
    });

    // Draw IMD indicators
    var imdProducts = calcIMDProducts(pilots);
    var imdHitPilots = {};
    imdProducts.forEach(function(p) {
      if (p.hitIdx >= 0) imdHitPilots[p.hitIdx] = true;
    });

    // IMD ticks on baseline
    var drawnIMD = {};
    imdProducts.forEach(function(p) {
      var key = Math.round(p.freq);
      if (drawnIMD[key]) return;
      drawnIMD[key] = true;
      var x = (p.freq - fMin) / fSpan * cw;
      if (x < 4 || x > cw - 4) return;
      var isHit = p.hitIdx >= 0;
      ctx.strokeStyle = isHit ? 'rgba(239,68,68,0.7)' : 'rgba(239,68,68,0.2)';
      ctx.lineWidth = isHit ? 2 : 1;
      var tickH = isHit ? 24 : 10;
      ctx.beginPath();
      ctx.moveTo(x, baseline);
      ctx.lineTo(x, baseline - tickH);
      ctx.stroke();
      if (isHit) {
        ctx.fillStyle = 'rgba(239,68,68,0.8)';
        ctx.beginPath();
        ctx.moveTo(x, baseline - tickH - 5);
        ctx.lineTo(x + 4, baseline - tickH);
        ctx.lineTo(x, baseline - tickH + 5);
        ctx.lineTo(x - 4, baseline - tickH);
        ctx.closePath();
        ctx.fill();
      }
    });

    // Draw callsign labels
    ctx.font = '700 10px -apple-system, BlinkMacSystemFont, sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'bottom';
    items.forEach(function (item) {
      ctx.fillStyle = item.color;
      ctx.fillText(item.label, item.centerX, item.labelY);
    });

    drawHighlightHump(ctx, highlightFreq, highlightBw, fMin, fSpan, cw, baseline, humpPeakH);
  }

  function drawHighlightHump(ctx, freq, bw, fMin, fSpan, cw, baseline, peakH) {
    if (!freq || freq < fMin || freq > fMin + fSpan) return;
    var centerX = (freq - fMin) / fSpan * cw;
    var hBw = bw || 20;
    var halfW = (hBw / fSpan * cw) / 2;
    if (halfW < 14) halfW = 14;
    var peakY = baseline - peakH;

    // Dashed hump outline
    ctx.setLineDash([5, 4]);
    ctx.beginPath();
    ctx.moveTo(centerX - halfW, baseline);
    ctx.bezierCurveTo(
      centerX - halfW * 0.5, baseline,
      centerX - halfW * 0.4, peakY,
      centerX, peakY
    );
    ctx.bezierCurveTo(
      centerX + halfW * 0.4, peakY,
      centerX + halfW * 0.5, baseline,
      centerX + halfW, baseline
    );
    ctx.closePath();
    ctx.fillStyle = 'rgba(255, 255, 255, 0.12)';
    ctx.fill();
    ctx.strokeStyle = '#ffffff';
    ctx.lineWidth = 2;
    ctx.stroke();
    ctx.setLineDash([]);

    // "YOU" label
    ctx.font = '700 11px -apple-system, BlinkMacSystemFont, sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'bottom';
    ctx.fillStyle = '#ffffff';
    ctx.fillText(t('SPECTRUM_YOU'), centerX, peakY - 3);
  }

  function roundRect(ctx, x, y, w, h, r) {
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.lineTo(x + w - r, y);
    ctx.arcTo(x + w, y, x + w, y + r, r);
    ctx.lineTo(x + w, y + h - r);
    ctx.arcTo(x + w, y + h, x + w - r, y + h, r);
    ctx.lineTo(x + r, y + h);
    ctx.arcTo(x, y + h, x, y + h - r, r);
    ctx.lineTo(x, y + r);
    ctx.arcTo(x, y, x + r, y, r);
    ctx.closePath();
  }

  function renderPilotList(pilots) {
    var container = $('pilot-list');
    clearChildren(container);

    if (!pilots || pilots.length === 0) {
      var emptyDiv = el('div', { className: 'empty-state' }, [
        el('div', { className: 'empty-state-text', textContent: t('WAITING_FOR_PILOTS') })
      ]);
      container.appendChild(emptyDiv);
      $('pilot-count').textContent = t('PILOT_COUNT_ZERO');
      renderSpectrum([]);
      return;
    }

    // Sort flyers by frequency, spotters alphabetically at end
    var flyers = pilots.filter(function (p) { return p.VideoSystem !== 'spotter'; });
    var spotters = pilots.filter(function (p) { return p.VideoSystem === 'spotter'; });
    flyers.sort(function (a, b) { return (a.AssignedFreqMHz || 0) - (b.AssignedFreqMHz || 0); });
    spotters.sort(function (a, b) { return a.Callsign.localeCompare(b.Callsign); });
    pilots = flyers.concat(spotters);

    // Build buddy group map for "sharing with" labels (exclude spotters)
    var buddyGroups = {};
    pilots.forEach(function (p) {
      if (p.VideoSystem !== 'spotter' && p.BuddyGroup && p.BuddyGroup > 0) {
        if (!buddyGroups[p.BuddyGroup]) buddyGroups[p.BuddyGroup] = [];
        buddyGroups[p.BuddyGroup].push(p);
      }
    });

    // Compute IMD hit pilots (after sort, so indices match the sorted order)
    var imdHitSet = getIMDHitPilots(pilots);

    var spotterDividerAdded = false;
    pilots.forEach(function (p, pilotIdx) {
      if (p.VideoSystem === 'spotter' && !spotterDividerAdded && flyers.length > 0) {
        spotterDividerAdded = true;
        container.appendChild(el('div', { className: 'spotter-divider' }));
      }
      var card = document.createElement('div');
      card.className = 'pilot-card';
      if (p.VideoSystem === 'spotter') {
        card.classList.add('is-spotter');
      }

      var isMe = p.ID === state.pilotId;
      var buddyIdx = (p.VideoSystem !== 'spotter' && p.BuddyGroup > 0) ? ((p.BuddyGroup - 1) % 8) + 1 : 0;

      if (isMe) {
        card.classList.add('is-me');
        card.addEventListener('click', function () {
          showPilotActions();
        });
      } else {
        card.classList.add('is-other');
        if (state.isLeader) {
          card.addEventListener('click', function () {
            showOtherPilotActions(p);
          });
        }
      }

      if (buddyIdx > 0) {
        card.classList.add('buddy-group', 'buddy-' + buddyIdx);
      }

      // Determine worst conflict level for card styling
      var conflicts = [];
      var worstLevel = null;
      if (p.VideoSystem !== 'spotter') {
        conflicts = p.Conflicts || p.conflicts || [];
        conflicts.forEach(function (c) {
          if (c.level === 'danger' || c.Level === 'danger') worstLevel = 'danger';
          else if (!worstLevel && (c.level === 'warning' || c.Level === 'warning')) worstLevel = 'warning';
        });
        if (worstLevel === 'danger') {
          card.classList.add('has-conflict-danger');
        } else if (worstLevel === 'warning') {
          card.classList.add('has-conflict-warning');
        }
      }

      // Frequency block — frequency prominent, channel name + bandwidth below
      var freqBlock;
      if (p.VideoSystem === 'spotter') {
        freqBlock = el('div', { className: 'pilot-freq-block spotter-freq-block' }, [
          el('div', { className: 'pilot-freq spotter-label', textContent: t('SYS_SPOTTER') })
        ]);
      } else {
        var freqEl = el('div', { className: 'pilot-freq' });
        if (p.AssignedFreqMHz) {
          freqEl.appendChild(document.createTextNode(String(p.AssignedFreqMHz)));
          freqEl.appendChild(el('span', { className: 'pilot-freq-unit', textContent: ' ' + t('UNIT_MHZ') }));
        } else {
          freqEl.textContent = '\u2014';
        }
        var channelLabel = p.AssignedChannel || '';
        var bw = p.BandwidthMHz || 0;
        if ((p.VideoSystem === 'dji_o3' || p.VideoSystem === 'dji_o4') && bw > 0) {
          channelLabel += ' (' + bw + 'M)';
        }
        var chEl = el('div', { className: 'pilot-channel', textContent: channelLabel });
        if (channelLabel) fitText(chEl, 15, 10);
        var freqBlockChildren = [freqEl, chEl];
        if (buddyIdx > 0) {
          freqBlockChildren.push(el('span', { className: 'pilot-buddy-badge buddy-badge-' + buddyIdx, textContent: t('BADGE_BUDDIES') }));
        }
        freqBlock = el('div', { className: 'pilot-freq-block' }, freqBlockChildren);
      }
      card.appendChild(freqBlock);

      // Info block
      var info = document.createElement('div');
      info.className = 'pilot-info';

      var nameEl = el('div', { className: 'pilot-callsign', textContent: p.Callsign });
      fitText(nameEl, 28, 14);
      info.appendChild(nameEl);

      // Badge row: system badge + YOU badge
      var badgeRow = el('div', { className: 'pilot-badge-row' });
      var sysLabel = SYSTEM_LABEL_KEYS[p.VideoSystem] ? t(SYSTEM_LABEL_KEYS[p.VideoSystem]) : p.VideoSystem.toUpperCase();
      var badge = el('span', { className: 'pilot-system-badge', textContent: sysLabel });
      badgeRow.appendChild(badge);

      if (isMe) {
        var youBadge = el('span', { className: 'pilot-you-badge', textContent: t('BADGE_YOU') });
        badgeRow.appendChild(youBadge);
      }

      if (p.ID === state.leaderPilotId) {
        var leaderBadge = el('span', { className: 'pilot-leader-badge', textContent: t('BADGE_LEADER') });
        badgeRow.appendChild(leaderBadge);
      }

      info.appendChild(badgeRow);

      card.appendChild(info);

      // Bottom row: IMD badge (left), status text (right), leader dot (far right)
      var hasIMD = imdHitSet[pilotIdx];
      var hasBuddy = buddyIdx > 0 && buddyGroups[p.BuddyGroup] && buddyGroups[p.BuddyGroup].length > 1;
      var hasConflicts = conflicts.length > 0;
      var hasLeaderDot = p.AddedByLeader;

      if (hasIMD || hasBuddy || hasConflicts || hasLeaderDot) {
        var bottomRow = el('div', { className: 'pilot-bottom-row' });

        if (hasIMD) {
          bottomRow.appendChild(el('span', { className: 'pilot-imd-flag', textContent: t('BADGE_IMD') }));
        }

        var statusText = el('div', { className: 'pilot-status-text' });
        if (hasBuddy) {
          var buddies = buddyGroups[p.BuddyGroup]
            .filter(function (b) { return b.ID !== p.ID; })
            .map(function (b) { return b.Callsign; });
          if (buddies.length > 0) {
            statusText.appendChild(el('span', {
              className: 'pilot-buddy-info buddy-text-' + buddyIdx,
              textContent: t('SHARING_WITH', { callsigns: buddies.join(', ') })
            }));
          }
        }
        conflicts.forEach(function (c) {
          var level = c.level || c.Level;
          var otherName = c.other_callsign || c.OtherCallsign || '?';
          var sep = c.separation_mhz || c.SeparationMHz || 0;
          var req = c.required_mhz || c.RequiredMHz || 0;
          statusText.appendChild(el('span', {
            className: 'pilot-conflict conflict-' + level,
            textContent: level === 'danger' ? t('CONFLICT_OVERLAP', { callsign: otherName, sep: sep, req: req }) : t('CONFLICT_CLOSE_TO', { callsign: otherName, sep: sep, req: req })
          }));
        });
        bottomRow.appendChild(statusText);

        if (hasLeaderDot) {
          bottomRow.appendChild(el('div', { className: 'pilot-added-dot' }));
        }

        card.appendChild(bottomRow);
      }

      container.appendChild(card);
    });

    var count = pilots.length;
    $('pilot-count').textContent = tPlural('PILOT_COUNT', count);
    renderSpectrum(pilots);
  }

  // ── Polling ───────────────────────────────────────────────────
  function startPolling() {
    stopPolling();
    state.pollTimer = setInterval(pollVersion, 5000);
  }

  function stopPolling() {
    if (state.pollTimer) {
      clearInterval(state.pollTimer);
      state.pollTimer = null;
    }
  }

  async function pollVersion() {
    try {
      var data = await apiGet('/api/sessions/' + state.sessionCode + '/poll');
      if (data.version !== state.knownVersion) {
        refreshSession();
      }
    } catch (err) {
      // Silently ignore poll failures
    }
  }

  // ── Leave Session ─────────────────────────────────────────────
  function initSessionView() {
    $('session-code-display').addEventListener('click', showQROverlay);
    $('btn-qr-close').addEventListener('click', hideQROverlay);
  }

  async function handleLeave() {
    if (!state.pilotId || !state.sessionCode) return;

    // If leader, show leader-leave dialog instead of leaving directly.
    if (state.isLeader) {
      showLeaderLeaveDialog();
      return;
    }

    await doLeave();
  }

  async function doLeave() {
    if (!state.pilotId || !state.sessionCode) return;

    try {
      await apiDelete('/api/pilots/' + state.pilotId + '?session=' + state.sessionCode);
    } catch (err) {
      // Even if delete fails, leave the session locally
    }

    stopPolling();
    clearState();

    // Reset setup state
    state.callsign = '';
    state.videoSystem = '';
    state.isLeader = false;
    state.leaderPilotId = null;
    $('input-callsign').value = '';

    validateAndShowLanding();
  }

  // ── Pilot Action Sheet ───────────────────────────────────────
  function showPilotActions() {
    var imdInfo = $('self-imd-info');
    if (imdInfo) {
      var pilots = state.cachedPilots || [];
      var myIdx = -1;
      for (var i = 0; i < pilots.length; i++) {
        if (pilots[i].ID === state.pilotId) { myIdx = i; break; }
      }
      var msg = myIdx >= 0 ? formatIMDSources(pilots, myIdx) : '';
      if (msg) {
        imdInfo.textContent = msg;
        imdInfo.classList.remove('hidden');
      } else {
        imdInfo.classList.add('hidden');
      }
    }
    $('pilot-actions').classList.remove('hidden');
  }

  function hidePilotActions() {
    $('pilot-actions').classList.add('hidden');
  }

  function initPilotActions() {
    $('btn-action-cancel').addEventListener('click', hidePilotActions);
    $('btn-leave-rotation').addEventListener('click', function () {
      hidePilotActions();
      handleLeave();
    });
    $('btn-change-channel').addEventListener('click', function () {
      hidePilotActions();
      showChannelChangeOptions();
    });
    $('btn-change-callsign').addEventListener('click', function () {
      hidePilotActions();
      showCallsignChange();
    });

    // Close on backdrop tap
    $('pilot-actions').addEventListener('click', function (e) {
      if (e.target === $('pilot-actions')) hidePilotActions();
    });
  }

  // ── Channel Change Options (self) ──────────────────────────
  function showChannelChangeOptions() {
    // Spotters go straight to video system change
    if (state.videoSystem === 'spotter') {
      state._changingVideoSystem = true;
      state.videoSystem = '';
      state.fccUnlocked = false;
      state.goggles = '';
      state.bandwidthMHz = 0;
      state.raceMode = false;
      state.walksnailMode = '';
      showScreen('setup');
      showStep('step-video');
      return;
    }
    $('channel-change-options').style.display = '';
  }

  function hideChannelChangeOptions() {
    $('channel-change-options').style.display = 'none';
  }

  function initChannelChangeOptions() {
    $('btn-cco-auto-assign').addEventListener('click', function () {
      hideChannelChangeOptions();
      submitChannelChange(0, state.myFreqMHz);
    });

    $('btn-cco-preference').addEventListener('click', function () {
      hideChannelChangeOptions();
      showChannelChange();
    });

    $('btn-cco-video-system').addEventListener('click', function () {
      hideChannelChangeOptions();
      state._changingVideoSystem = true;
      state.videoSystem = '';
      state.fccUnlocked = false;
      state.goggles = '';
      state.bandwidthMHz = 0;
      state.raceMode = false;
      state.walksnailMode = '';
      showScreen('setup');
      showStep('step-video');
    });

    $('btn-cco-cancel').addEventListener('click', hideChannelChangeOptions);

    $('channel-change-options').addEventListener('click', function (e) {
      if (e.target === $('channel-change-options')) hideChannelChangeOptions();
    });
  }

  // ── Moved-by-rebalance Dialog ──────────────────────────────
  function showMovedDialog(channel, freqMHz, leaderPilotId, pilots) {
    var leaderName = '';
    for (var i = 0; i < pilots.length; i++) {
      if (pilots[i].ID === leaderPilotId) {
        leaderName = pilots[i].Callsign;
        break;
      }
    }

    var msg = $('moved-dialog-message');
    while (msg.firstChild) msg.removeChild(msg.firstChild);

    var text = t('MOVED_TO_TEXT', { channel: channel, freq: freqMHz });
    if (leaderName) {
      text += ' ' + t('MOVED_TALK_TO_LEADER', { leaderName: leaderName });
    }
    msg.appendChild(document.createTextNode(text));

    $('moved-dialog').style.display = '';
  }

  function dismissMovedDialog() {
    $('moved-dialog').style.display = 'none';
  }

  function initMovedDialog() {
    $('moved-dialog-ok').addEventListener('click', dismissMovedDialog);
    $('moved-dialog').addEventListener('click', function (e) {
      if (e.target === $('moved-dialog')) dismissMovedDialog();
    });
  }

  // ── Channel Change ──────────────────────────────────────────
  var channelChangeSelectedFreq = 0;

  function renderChannelChangeSpectrum(freq) {
    var sys = getEffectiveVideoSystem();
    var bw = occupiedBandwidth(sys, state.bandwidthMHz);
    // Show other pilots (exclude self) so you see where you'd land
    var others = (state.cachedPilots || []).filter(function (p) { return p.ID !== state.pilotId; });
    // Include hypothetical self at candidate freq for IMD preview
    var pilotsForIMD = others;
    if (freq > 0) {
      pilotsForIMD = others.concat([{ AssignedFreqMHz: freq, VideoSystem: sys, BandwidthMHz: bw, Callsign: state.callsign || t('SPECTRUM_YOU'), ID: state.pilotId }]);
    }
    renderSpectrum(pilotsForIMD, 'spectrum-change', freq, bw);
  }

  function showChannelChange() {
    var picker = $('channel-change-picker');
    clearChildren(picker);
    channelChangeSelectedFreq = 0;

    var pool = filterPoolToFixedChannels(getChannelPool());
    var myVideoSystem = getEffectiveVideoSystem();
    var myBw = state.bandwidthMHz || 0;

    // In fixed-channel sessions, show all channels (buddying is expected).
    var isFixedSession = !!(state.sessionFixedChannels || state.fixedChannels);

    // Count pilots per frequency for buddy info
    var freqPilotCount = {};
    if (isFixedSession && state.cachedPilots) {
      state.cachedPilots.forEach(function (p) {
        if (p.Active && p.ID !== state.pilotId && p.AssignedFreqMHz) {
          freqPilotCount[p.AssignedFreqMHz] = (freqPilotCount[p.AssignedFreqMHz] || 0) + 1;
        }
      });
    }

    pool.forEach(function (ch) {
      // Non-leader self-service: hide conflicting channels unless fixed session.
      if (!isFixedSession) {
        var conflicts = findConflicts(ch.freq, state.pilotId, myVideoSystem, myBw);
        if (conflicts.length > 0) return;
      }

      var nameSpan = el('span', { className: 'ch-name', textContent: ch.name });
      var countOnChannel = freqPilotCount[ch.freq] || 0;
      var freqLabel = String(ch.freq);
      if (isFixedSession && countOnChannel > 0) {
        freqLabel += ' (' + countOnChannel + ')';
      }
      var freqSpan = el('span', { className: 'ch-freq', textContent: freqLabel });
      var btn = el('button', { className: 'btn-channel' }, [nameSpan, freqSpan]);
      btn.addEventListener('click', function () {
        picker.querySelectorAll('.btn-channel').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        channelChangeSelectedFreq = ch.freq;
        renderChannelChangeSpectrum(ch.freq);
        $('btn-confirm-channel-change').classList.remove('hidden');
        // Promote lock button to primary, demote auto-assign
        $('btn-confirm-channel-change').className = 'btn btn-primary btn-large';
        $('btn-auto-reassign').className = 'btn btn-secondary btn-large';
        $('btn-confirm-channel-change').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      });
      picker.appendChild(btn);
    });
    adaptPickerGrid(picker);

    $('channel-change-title').textContent = t('SELECT_CHANNEL');
    $('btn-auto-reassign').className = 'btn btn-primary btn-large';
    $('btn-auto-reassign').classList.remove('hidden');
    // Video system change is now in the options dialog, hide it from picker
    $('btn-change-video-system').classList.add('hidden');
    $('btn-confirm-channel-change').classList.add('hidden');
    state._channelChangeForPilot = null;
    state._changeVideoSystemPilot = null;
    $('channel-change').classList.remove('hidden');
    renderChannelChangeSpectrum(0);
  }

  function hideChannelChange() {
    $('channel-change').classList.add('hidden');
  }

  function initChannelChange() {
    $('btn-confirm-channel-change').addEventListener('click', function () {
      if (state._channelChangeForPilot) {
        submitChannelChangeForPilot(state._channelChangeForPilot, channelChangeSelectedFreq);
      } else {
        submitChannelChange(channelChangeSelectedFreq);
      }
    });
    $('btn-auto-reassign').addEventListener('click', function () {
      submitChannelChange(0, state.myFreqMHz);
    });
    $('btn-change-video-system').addEventListener('click', async function () {
      hideChannelChange();
      // Leader changing video system for a leader-added pilot
      if (state._changeVideoSystemPilot) {
        var pilot = state._changeVideoSystemPilot;
        state._changeVideoSystemPilot = null;
        try {
          await apiDelete('/api/pilots/' + pilot.ID + '?session=' + state.sessionCode);
        } catch (err) {
          // Continue even if delete fails
        }
        // Open add-pilot dialog pre-filled with their callsign
        showAddPilotDialog();
        $('input-add-callsign').value = pilot.Callsign;
        return;
      }
      // Self: update video system in place
      hideChannelChange();
      state._changingVideoSystem = true;
      state.videoSystem = '';
      state.fccUnlocked = false;
      state.goggles = '';
      state.bandwidthMHz = 0;
      state.raceMode = false;
      state.walksnailMode = '';
      showScreen('setup');
      showStep('step-video');
    });
    $('btn-channel-change-cancel').addEventListener('click', hideChannelChange);
    $('channel-change').addEventListener('click', function (e) {
      if (e.target === $('channel-change')) hideChannelChange();
    });
  }

  async function submitChannelChange(freqMHz, excludeFreqMHz) {
    hideChannelChange();
    var body = { preferred_frequency_mhz: freqMHz };
    if (excludeFreqMHz && freqMHz === 0) {
      body.exclude_freq_mhz = excludeFreqMHz;
    }
    try {
      // Preview first to check for conflicts.
      var preview = await apiPost(
        '/api/pilots/' + state.pilotId + '/preview-channel?session=' + state.sessionCode,
        body
      );
      var level = preview.level || 0;

      if (level === 0 && preview.override_reason) {
        showOverrideDialog(preview.override_reason, preview.assignment, function () {
          commitChannelChange(body);
        });
        return;
      }

      if (level === 1) {
        showChoiceDialog(preview,
          function onBuddy(buddy) {
            commitChannelChange({ preferred_frequency_mhz: buddy.freq_mhz, choice: 'buddy' });
          },
          function onRebalance() {
            body.choice = 'rebalance';
            commitChannelChange(body);
          },
          null
        );
        return;
      }

      // Level 0, no override — apply immediately.
      await commitChannelChange(body);
    } catch (err) {
      refreshSession();
    }
  }

  async function commitChannelChange(body) {
    try {
      state.expectingAssignmentChange = true;
      await apiPut(
        '/api/pilots/' + state.pilotId + '/channel?session=' + state.sessionCode,
        body
      );
      state.preferredFreqMHz = body.preferred_frequency_mhz;
      refreshSession();
    } catch (err) {
      state.expectingAssignmentChange = false;
      refreshSession();
    }
  }

  // ── Channel Change for Other Pilot (Leader) ────────────────
  function getChannelPoolForPilot(pilot) {
    var sys = pilot.VideoSystem;
    var fcc = pilot.FCCUnlocked || false;
    var bw = pilot.BandwidthMHz || 0;
    var rm = pilot.RaceMode || false;
    var goggles = pilot.Goggles || '';

    switch (sys) {
      case 'analog':
        return mergeAnalogBands(pilot.AnalogBands ? pilot.AnalogBands.split(',') : ['R']);
      case 'hdzero':
        return CHANNELS.raceband;
      case 'dji_v1':
        return fcc ? CHANNELS.dji_v1_fcc : CHANNELS.dji_v1_stock;
      case 'dji_o3':
        if (bw >= 40) return fcc ? CHANNELS.dji_o3_40_fcc : CHANNELS.dji_o3_40_stock;
        return fcc ? CHANNELS.dji_o3_fcc : CHANNELS.dji_o3_stock;
      case 'dji_o4':
        if (rm && (goggles === 'goggles_3' || goggles === 'goggles_n3'))
          return CHANNELS.raceband;
        if (bw >= 60) return CHANNELS.dji_o4_60;
        if (bw >= 40) return fcc ? CHANNELS.dji_o4_40_fcc : CHANNELS.dji_o4_40_stock;
        return fcc ? CHANNELS.dji_o4_fcc : CHANNELS.dji_o4_stock;
      case 'walksnail_std':
        return fcc ? CHANNELS.dji_v1_fcc : CHANNELS.dji_v1_stock;
      case 'walksnail_race':
        return CHANNELS.raceband;
      case 'openipc':
        return CHANNELS.openipc;
      default:
        return CHANNELS.raceband;
    }
  }

  // Check if a frequency conflicts with existing pilots (excluding targetPilotId)
  function findConflicts(freqMHz, targetPilotId, targetVideoSystem, targetBwMHz) {
    var conflicts = [];
    var targetBw = occupiedBandwidth(targetVideoSystem, targetBwMHz);
    var others = (state.cachedPilots || []).filter(function (p) {
      return p.ID !== targetPilotId && p.Active !== false;
    });
    others.forEach(function (p) {
      if (!p.AssignedFreqMHz) return;
      var otherBw = occupiedBandwidth(p.VideoSystem, p.BandwidthMHz);
      var separation = Math.abs(freqMHz - p.AssignedFreqMHz);
      var required = (targetBw + otherBw) / 2;
      if (separation < required) {
        conflicts.push({
          pilotId: p.ID,
          callsign: p.Callsign,
          freq: p.AssignedFreqMHz,
          separation: separation,
          required: required,
          exact: separation === 0
        });
      }
    });
    return conflicts;
  }

  function showChannelChangeForPilot(pilot) {
    var picker = $('channel-change-picker');
    clearChildren(picker);
    channelChangeSelectedFreq = 0;

    if (pilot.VideoSystem === 'spotter') {
      $('channel-change-title').textContent = t('CHANGE_CHANNEL_FOR', { callsign: pilot.Callsign });
      $('btn-auto-reassign').classList.add('hidden');
      $('btn-change-video-system').classList.remove('hidden');
      state._changeVideoSystemPilot = pilot;
      $('btn-confirm-channel-change').classList.add('hidden');
      state._channelChangeForPilot = pilot.ID;
      $('channel-change').classList.remove('hidden');
      return;
    }

    var pool = filterPoolToFixedChannels(getChannelPoolForPilot(pilot));
    var pilotBw = pilot.BandwidthMHz || 0;

    pool.forEach(function (ch) {
      var nameSpan = el('span', { className: 'ch-name', textContent: ch.name });
      var freqSpan = el('span', { className: 'ch-freq', textContent: String(ch.freq) });
      var conflicts = findConflicts(ch.freq, pilot.ID, pilot.VideoSystem, pilotBw);
      var isConflict = conflicts.length > 0;
      var btnClass = 'btn-channel' + (isConflict ? ' channel-conflict' : '');
      var btn = el('button', { className: btnClass }, [nameSpan, freqSpan]);
      btn.addEventListener('click', function () {
        picker.querySelectorAll('.btn-channel').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        channelChangeSelectedFreq = ch.freq;
        // Show spectrum with this pilot excluded, highlight the selection
        var others = (state.cachedPilots || []).filter(function (p) { return p.ID !== pilot.ID; });
        var bw = occupiedBandwidth(pilot.VideoSystem, pilot.BandwidthMHz);
        // Include hypothetical pilot at candidate freq for IMD preview
        var withCandidate = others.concat([{ AssignedFreqMHz: ch.freq, VideoSystem: pilot.VideoSystem, BandwidthMHz: pilot.BandwidthMHz, Callsign: pilot.Callsign, ID: pilot.ID }]);
        renderSpectrum(withCandidate, 'spectrum-change', ch.freq, bw);

        if (isConflict) {
          // Leader tapped a conflicting channel — hide picker, show confirmation
          hideChannelChange();
          showLeaderConflictConfirm(pilot, ch, conflicts);
        } else {
          $('btn-confirm-channel-change').classList.remove('hidden');
          $('btn-confirm-channel-change').className = 'btn btn-primary btn-large';
          $('btn-confirm-channel-change').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        }
      });
      picker.appendChild(btn);
    });
    adaptPickerGrid(picker);

    // Update the title and hide self-only buttons.
    $('channel-change-title').textContent = t('CHANGE_CHANNEL_FOR', { callsign: pilot.Callsign });
    $('btn-auto-reassign').classList.add('hidden');
    // Show video system change for leader-added pilots or spotters
    if ((pilot.VideoSystem === 'spotter' || pilot.AddedByLeader) && state.isLeader) {
      $('btn-change-video-system').classList.remove('hidden');
      state._changeVideoSystemPilot = pilot;
    } else {
      $('btn-change-video-system').classList.add('hidden');
    }
    $('btn-confirm-channel-change').classList.add('hidden');
    state._channelChangeForPilot = pilot.ID;
    $('channel-change').classList.remove('hidden');
    // Show spectrum without the target pilot
    var others = (state.cachedPilots || []).filter(function (p) { return p.ID !== pilot.ID; });
    var bw = occupiedBandwidth(pilot.VideoSystem, pilot.BandwidthMHz);
    renderSpectrum(others, 'spectrum-change', 0, bw);
  }

  function showLeaderConflictConfirm(pilot, channel, conflicts) {
    // Determine if any conflict is exact-frequency (buddy) vs adjacent overlap
    var hasExact = conflicts.some(function (c) { return c.exact; });
    var conflictNames = conflicts.map(function (c) { return c.callsign; }).join(', ');

    // Reuse the buddy-suggestion dialog for conflict confirmation
    var textEl = $('buddy-suggestion-text');
    while (textEl.firstChild) textEl.removeChild(textEl.firstChild);

    if (hasExact) {
      textEl.appendChild(document.createTextNode(
        t('BUDDY_CONFIRM_TEXT', { pilotCallsign: pilot.Callsign, channelName: channel.name, freq: channel.freq, conflictNames: conflictNames })
      ));
      $('buddy-suggestion').querySelector('.action-sheet-title').textContent = t('BUDDY_CONFIRMATION');
    } else {
      textEl.appendChild(document.createTextNode(
        t('OVERLAP_WARNING_TEXT', { channelName: channel.name, freq: channel.freq, conflictNames: conflictNames, pilotCallsign: pilot.Callsign })
      ));
      $('buddy-suggestion').querySelector('.action-sheet-title').textContent = t('OVERLAP_WARNING');
    }

    $('btn-buddy-up').textContent = t('BTN_FORCE');
    $('btn-buddy-up').onclick = function () {
      $('buddy-suggestion').classList.add('hidden');
      $('btn-buddy-up').textContent = t('BTN_BUDDY_UP');
      submitChannelChangeForPilot(pilot.ID, channel.freq);
    };
    $('btn-buddy-cancel').onclick = function () {
      $('buddy-suggestion').classList.add('hidden');
      $('btn-buddy-up').textContent = t('BTN_BUDDY_UP');
      // Re-show the channel picker so leader can pick a different channel
      showChannelChangeForPilot(pilot);
    };
    $('buddy-suggestion').classList.remove('hidden');
  }

  async function submitChannelChangeForPilot(pilotId, freqMHz) {
    hideChannelChange();
    // Leader moving another pilot — always force (leader's choice is authoritative)
    // and commit directly (no preview dialogs).
    var body = { preferred_frequency_mhz: freqMHz, force: true };
    await commitChannelChangeForPilot(pilotId, body);
  }

  async function commitChannelChangeForPilot(pilotId, body) {
    try {
      await apiPut(
        '/api/pilots/' + pilotId + '/channel?session=' + state.sessionCode,
        body
      );
      refreshSession();
    } catch (err) {
      refreshSession();
    }
  }

  // ── Callsign Change ─────────────────────────────────────────
  function showCallsignChange() {
    $('input-new-callsign').value = state.callsign;
    hideError('callsign-change-error');
    $('callsign-change').classList.remove('hidden');
    $('input-new-callsign').focus();
    $('input-new-callsign').select();
  }

  function hideCallsignChange() {
    $('callsign-change').classList.add('hidden');
  }

  function initCallsignChange() {
    $('input-new-callsign').addEventListener('input', function (e) {
      e.target.value = e.target.value.toUpperCase();
    });
    $('btn-callsign-save').addEventListener('click', submitCallsignChange);
    $('btn-callsign-change-cancel').addEventListener('click', hideCallsignChange);
    $('callsign-change').addEventListener('click', function (e) {
      if (e.target === $('callsign-change')) hideCallsignChange();
    });
    $('input-new-callsign').addEventListener('keydown', function (e) {
      if (e.key === 'Enter') submitCallsignChange();
    });
  }

  async function submitCallsignChange() {
    var newCallsign = $('input-new-callsign').value.trim();
    if (!newCallsign) {
      showError('callsign-change-error', t('ERR_CALLSIGN_EMPTY'));
      return;
    }
    hideError('callsign-change-error');

    var btn = $('btn-callsign-save');
    setLoading(btn, true);

    try {
      await apiPut(
        '/api/pilots/' + state.pilotId + '/callsign?session=' + state.sessionCode,
        { callsign: newCallsign }
      );
      state.callsign = newCallsign;
      hideCallsignChange();
      refreshSession();
    } catch (err) {
      var msg = err.message || '';
      if (msg.includes('callsign already') || msg.includes('409')) {
        showError('callsign-change-error', t('ERR_CALLSIGN_IN_USE'));
      } else {
        showError('callsign-change-error', t('ERR_FAILED', { error: msg.toUpperCase() }));
      }
    } finally {
      setLoading(btn, false);
    }
  }

  // ── Other Pilot Actions (with slide to remove) ─────────────
  var otherPilotTarget = null;

  function showOtherPilotActions(pilot) {
    otherPilotTarget = pilot;
    $('other-pilot-name').textContent = pilot.Callsign;
    resetSlideHandle();
    // IMD source info
    var imdInfo = $('other-imd-info');
    if (imdInfo) {
      var pilots = state.cachedPilots || [];
      var targetIdx = -1;
      for (var i = 0; i < pilots.length; i++) {
        if (pilots[i].ID === pilot.ID) { targetIdx = i; break; }
      }
      var msg = targetIdx >= 0 ? formatIMDSources(pilots, targetIdx) : '';
      if (msg) {
        imdInfo.textContent = msg;
        imdInfo.classList.remove('hidden');
      } else {
        imdInfo.classList.add('hidden');
      }
    }
    // Show/hide leader-only controls
    if (state.isLeader) {
      $('btn-change-other-channel').classList.remove('hidden');
      $('btn-transfer-leader').classList.remove('hidden');
      $('slide-remove-track').classList.remove('hidden');
    } else {
      $('btn-change-other-channel').classList.add('hidden');
      $('btn-transfer-leader').classList.add('hidden');
      $('slide-remove-track').classList.add('hidden');
    }
    $('other-pilot-actions').classList.remove('hidden');
  }

  function hideOtherPilotActions() {
    $('other-pilot-actions').classList.add('hidden');
    otherPilotTarget = null;
  }

  function resetSlideHandle() {
    var handle = $('slide-remove-handle');
    handle.classList.add('snapping');
    handle.style.left = '4px';
    setTimeout(function () { handle.classList.remove('snapping'); }, 300);
  }

  function initOtherPilotActions() {
    $('btn-other-cancel').addEventListener('click', hideOtherPilotActions);
    $('btn-change-other-channel').addEventListener('click', function () {
      if (!otherPilotTarget) return;
      var pilot = otherPilotTarget;
      hideOtherPilotActions();
      showChannelChangeForPilot(pilot);
    });
    $('other-pilot-actions').addEventListener('click', function (e) {
      if (e.target === $('other-pilot-actions')) hideOtherPilotActions();
    });

    // Slide-to-remove touch/mouse handling
    var handle = $('slide-remove-handle');
    var track = $('slide-remove-track');
    var dragging = false;
    var startX = 0;
    var handleStartLeft = 0;

    function getTrackWidth() {
      return track.getBoundingClientRect().width;
    }

    function onStart(clientX) {
      dragging = true;
      startX = clientX;
      handleStartLeft = handle.offsetLeft;
      handle.classList.remove('snapping');
    }

    function onMove(clientX) {
      if (!dragging) return;
      var dx = clientX - startX;
      var newLeft = handleStartLeft + dx;
      var maxLeft = getTrackWidth() - handle.offsetWidth - 4;
      if (newLeft < 4) newLeft = 4;
      if (newLeft > maxLeft) newLeft = maxLeft;
      handle.style.left = newLeft + 'px';
    }

    function onEnd() {
      if (!dragging) return;
      dragging = false;

      var maxLeft = getTrackWidth() - handle.offsetWidth - 4;
      var currentLeft = handle.offsetLeft;
      var pct = currentLeft / maxLeft;

      if (pct > 0.85) {
        // Triggered — remove the pilot
        removeOtherPilot();
      } else {
        // Snap back
        resetSlideHandle();
      }
    }

    // Touch events
    handle.addEventListener('touchstart', function (e) {
      e.preventDefault();
      onStart(e.touches[0].clientX);
    }, { passive: false });

    document.addEventListener('touchmove', function (e) {
      if (dragging) {
        e.preventDefault();
        onMove(e.touches[0].clientX);
      }
    }, { passive: false });

    document.addEventListener('touchend', function () {
      onEnd();
    });

    // Mouse events
    handle.addEventListener('mousedown', function (e) {
      e.preventDefault();
      onStart(e.clientX);
    });

    document.addEventListener('mousemove', function (e) {
      if (dragging) onMove(e.clientX);
    });

    document.addEventListener('mouseup', function () {
      onEnd();
    });
  }

  async function removeOtherPilot() {
    if (!otherPilotTarget) return;
    var pilotId = otherPilotTarget.ID;

    try {
      await apiDelete('/api/pilots/' + pilotId + '?session=' + state.sessionCode);
    } catch (err) {
      // Refresh anyway
    }

    hideOtherPilotActions();
    refreshSession();
  }

  // ── Leader Controls ──────────────────────────────────────────
  function updateLeaderControls() {
    var leaderControls = $('leader-controls');
    if (state.isLeader) {
      leaderControls.classList.remove('hidden');
    } else {
      leaderControls.classList.add('hidden');
    }
  }

  function initLeaderControls() {
    // Rebalance power slider state
    var rebalancePowerIndex = 2;
    var rebalancePowerInitialized = false;

    function initRebalanceSlider() {
      var notches = $('rebalance-slider-notches');
      if (rebalancePowerInitialized) return;
      rebalancePowerInitialized = true;
      for (var i = 0; i < REBALANCE_POSITIONS; i++) {
        var n = document.createElement('div');
        n.className = 'power-slider-notch';
        notches.appendChild(n);
      }
    }

    // Rebalance slider has POWER_STEPS.length + 1 positions (last = NO LIMIT)
    var REBALANCE_POSITIONS = POWER_STEPS.length + 1;

    function rebalanceMW() {
      if (rebalancePowerIndex >= POWER_STEPS.length) return 0; // NO LIMIT
      return POWER_STEPS[rebalancePowerIndex].mw;
    }

    function updateRebalancePower(idx) {
      rebalancePowerIndex = idx;
      var label = idx >= POWER_STEPS.length ? t('NO_LIMIT') : t('REBALANCE_POWER_VALUE', { mw: POWER_STEPS[idx].mw });
      $('rebalance-power-value').textContent = label;
      var track = $('rebalance-slider-track');
      var thumbRadius = 18;
      var usable = track.offsetWidth - thumbRadius * 2;
      var pct = idx / (REBALANCE_POSITIONS - 1);
      $('rebalance-slider-thumb').style.left = (thumbRadius - 18 + pct * usable) + 'px';
    }

    function getRebalanceStepFromX(clientX) {
      var rect = $('rebalance-slider-track').getBoundingClientRect();
      var pct = (clientX - rect.left) / rect.width;
      pct = Math.max(0, Math.min(1, pct));
      return Math.round(pct * (REBALANCE_POSITIONS - 1));
    }

    (function () {
      var track = $('rebalance-slider-track');
      if (!track) return;
      var dragging = false;
      track.addEventListener('pointerdown', function (e) {
        dragging = true;
        track.setPointerCapture(e.pointerId);
        var idx = getRebalanceStepFromX(e.clientX);
        updateRebalancePower(idx);
        fetchRebalancePreview();
      });
      track.addEventListener('pointermove', function (e) {
        if (!dragging) return;
        var idx = getRebalanceStepFromX(e.clientX);
        if (idx !== rebalancePowerIndex) {
          updateRebalancePower(idx);
        }
      });
      track.addEventListener('pointerup', function () {
        if (dragging) {
          dragging = false;
          fetchRebalancePreview();
        }
      });
      track.addEventListener('pointercancel', function () { dragging = false; });
    })();

    async function fetchRebalancePreview() {
      try {
        var body = { power_ceiling_mw: rebalanceMW() };
        var proposed = await apiPost('/api/sessions/' + state.sessionCode + '/preview-rebalance', body);
        renderSpectrum(proposed, 'spectrum-rebalance-after', 0, 0);
      } catch (err) {
        renderSpectrum([], 'spectrum-rebalance-after', 0, 0);
      }
    }

    $('btn-rebalance-all').addEventListener('click', async function () {
      // Show dialog first so canvases have layout dimensions
      $('rebalance-confirm').classList.remove('hidden');

      // Set up the power slider
      initRebalanceSlider();
      var currentCeiling = state.sessionPowerCeiling || 0;
      if (currentCeiling > 0) {
        var matchIdx = POWER_STEPS.length; // default to NO LIMIT
        for (var i = 0; i < POWER_STEPS.length; i++) {
          if (POWER_STEPS[i].mw === currentCeiling) { matchIdx = i; break; }
        }
        updateRebalancePower(matchIdx);
      } else {
        // No ceiling — start at NO LIMIT (rightmost)
        updateRebalancePower(POWER_STEPS.length);
      }

      // Render current spectrum
      var pilots = state.cachedPilots || [];
      renderSpectrum(pilots, 'spectrum-rebalance-before', 0, 0);

      // Fetch proposed assignments
      fetchRebalancePreview();
    });

    $('btn-add-pilot').addEventListener('click', function () {
      showAddPilotDialog();
    });

    $('btn-transfer-leader').addEventListener('click', function () {
      if (!otherPilotTarget) return;
      transferLeadership(otherPilotTarget.ID);
    });

    // Rebalance confirmation
    $('btn-rebalance-confirm').addEventListener('click', async function () {
      $('rebalance-confirm').classList.add('hidden');
      var btn = $('btn-rebalance-all');
      setLoading(btn, true);
      try {
        state.expectingAssignmentChange = true;
        var body = { power_ceiling_mw: rebalanceMW() };
        var result = await apiPost('/api/sessions/' + state.sessionCode + '/rebalance', body);
        if ($('rebalance-hint')) $('rebalance-hint').style.display = 'none';
        await refreshSession();
        showRebalanceResult(result);
      } catch (err) {
        state.expectingAssignmentChange = false;
        // Silently ignore
      } finally {
        setLoading(btn, false);
      }
    });
    $('btn-rebalance-cancel').addEventListener('click', function () {
      $('rebalance-confirm').classList.add('hidden');
    });
    $('rebalance-confirm').addEventListener('click', function (e) {
      if (e.target === $('rebalance-confirm')) {
        $('rebalance-confirm').classList.add('hidden');
      }
    });

    // Rebalance result dismiss
    $('btn-rebalance-result-ok').addEventListener('click', function () {
      $('rebalance-result').classList.add('hidden');
    });
    $('rebalance-result').addEventListener('click', function (e) {
      if (e.target === $('rebalance-result')) {
        $('rebalance-result').classList.add('hidden');
      }
    });
  }

  function showRebalanceResult(result) {
    var moved = result.moved || [];
    var unresolved = result.unresolved || [];
    var list = $('rebalance-result-list');
    clearChildren(list);

    if (moved.length > 0) {
      var movedTitle = el('div', { className: 'rebalance-section-title', textContent: t('REBALANCE_MOVED') });
      list.appendChild(movedTitle);
      moved.forEach(function (d) {
        var nameEl = el('div', { className: 'displacement-name', textContent: d.callsign });
        var moveText = d.old_channel + ' (' + d.old_freq_mhz + ') \u2192 ' +
          d.new_channel + ' (' + d.new_freq_mhz + ')';
        var moveEl = el('div', { className: 'displacement-move', textContent: moveText });
        var item = el('div', { className: 'displacement-item' }, [nameEl, moveEl]);
        list.appendChild(item);
      });
    } else {
      var noChanges = el('p', { className: 'rebalance-result-text', textContent: t('REBALANCE_NO_CHANGES') });
      list.appendChild(noChanges);
    }

    if (unresolved.length > 0) {
      var conflictTitle = el('div', { className: 'rebalance-section-title rebalance-conflict-title', textContent: t('REBALANCE_UNRESOLVED') });
      list.appendChild(conflictTitle);
      unresolved.forEach(function (c) {
        var reasonEl = el('div', { className: 'rebalance-conflict-reason', textContent: c.reason });
        list.appendChild(reasonEl);
      });
    }

    $('rebalance-result').classList.remove('hidden');
  }

  async function transferLeadership(pilotId) {
    hideOtherPilotActions();
    try {
      await apiPost('/api/sessions/' + state.sessionCode + '/transfer-leader', { pilot_id: pilotId });
      refreshSession();
    } catch (err) {
      refreshSession();
    }
  }

  // ── Add Pilot Dialog ────────────────────────────────────────
  // Systems that need no follow-up options.
  var SIMPLE_SYSTEMS = ['hdzero', 'openipc', 'walksnail_race', 'spotter'];
  // Systems that need FCC toggle only.
  var FCC_SYSTEMS = ['dji_v1', 'walksnail_std'];
  // Systems that need FCC + bandwidth.
  var BW_SYSTEMS = { dji_o3: [20, 40], dji_o4: [20, 40, 60] };

  var addPilotState = { system: '', fccUnlocked: false, goggles: '', bandwidthMHz: 0, raceMode: false, analogBands: ['R'] };

  function showAddPilotDialog() {
    $('input-add-callsign').value = '';
    hideError('add-pilot-error');
    document.querySelectorAll('.btn-add-system').forEach(function (b) {
      b.classList.remove('selected');
      b.classList.remove('hidden');
    });
    $('add-pilot-options').classList.add('hidden');
    addPilotState = { system: '', fccUnlocked: false, goggles: '', bandwidthMHz: 0, raceMode: false, analogBands: ['R'] };

    // Show fixed channels hint
    var fcHint = $('add-pilot-fixed-hint');
    if (fcHint && state.sessionFixedChannels) {
      try {
        var channels = JSON.parse(state.sessionFixedChannels);
        var names = channels.map(function (c) { return c.name; });
        fcHint.textContent = t('ADD_PILOT_FIXED_HINT', { channels: names.join(' \u00b7 ') });
        fcHint.classList.remove('hidden');
      } catch (e) { fcHint.classList.add('hidden'); }
    } else if (fcHint) {
      fcHint.classList.add('hidden');
    }

    $('add-pilot').classList.remove('hidden');
    $('input-add-callsign').focus();
  }

  function hideAddPilotDialog() {
    $('add-pilot').classList.add('hidden');
  }

  function showAddPilotOptions(system) {
    addPilotState.system = system;
    addPilotState.fccUnlocked = false;
    addPilotState.goggles = '';
    addPilotState.bandwidthMHz = 20;
    addPilotState.raceMode = false;

    // FCC toggle
    var needsFCC = FCC_SYSTEMS.indexOf(system) !== -1 || BW_SYSTEMS[system];
    if (needsFCC) {
      $('add-pilot-fcc').classList.remove('hidden');
      document.querySelectorAll('.btn-add-fcc').forEach(function (b) { b.classList.remove('selected'); });
    } else {
      $('add-pilot-fcc').classList.add('hidden');
    }

    // Goggles selector (DJI O4 only — shown after FCC selected)
    $('add-pilot-goggles').classList.add('hidden');
    document.querySelectorAll('.btn-add-goggles').forEach(function (b) { b.classList.remove('selected'); });

    // Bandwidth buttons
    var bwOptions = BW_SYSTEMS[system];
    if (bwOptions) {
      var bwContainer = $('add-pilot-bw-buttons');
      clearChildren(bwContainer);
      bwOptions.forEach(function (bw) {
        var btn = el('button', {
          className: 'btn btn-toggle' + (bw === 20 ? ' active' : ''),
          textContent: t('BW_MHZ', { bw: bw })
        });
        applyBandwidthHint(btn, bw);
        btn.addEventListener('click', function () {
          addPilotState.bandwidthMHz = bw;
          bwContainer.querySelectorAll('.btn-toggle').forEach(function (b) { b.classList.remove('active'); });
          btn.classList.add('active');
          updateAddPilotRaceMode();
        });
        bwContainer.appendChild(btn);
      });
      // DJI O4: bandwidth hidden until goggles selected
      if (system === 'dji_o4') {
        $('add-pilot-bw').classList.add('hidden');
      } else {
        $('add-pilot-bw').classList.remove('hidden');
      }
    } else {
      $('add-pilot-bw').classList.add('hidden');
    }

    // Race mode (hidden until conditionally shown)
    $('add-pilot-racemode').classList.add('hidden');
    document.querySelectorAll('.btn-add-racemode').forEach(function (b) { b.classList.remove('selected'); });

    // Analog band buttons
    if (system === 'analog') {
      addPilotState.analogBands = ['R'];
      document.querySelectorAll('.add-band-btn').forEach(function (b) {
        b.classList.toggle('active', b.dataset.addBand === 'R');
      });
      $('add-pilot-bands').classList.remove('hidden');
    } else {
      $('add-pilot-bands').classList.add('hidden');
    }

    $('add-pilot-options').classList.remove('hidden');
  }

  function updateAddPilotRaceMode() {
    if (addPilotState.system !== 'dji_o4') return;
    // Race Mode requires FCC + Goggles 3 or N3
    if (addPilotState.fccUnlocked &&
        (addPilotState.goggles === 'goggles_3' || addPilotState.goggles === 'goggles_n3')) {
      $('add-pilot-racemode').classList.remove('hidden');
    } else {
      $('add-pilot-racemode').classList.add('hidden');
      addPilotState.raceMode = false;
      document.querySelectorAll('.btn-add-racemode').forEach(function (b) { b.classList.remove('selected'); });
    }
  }

  function initAddPilotDialog() {
    $('input-add-callsign').addEventListener('input', function (e) {
      e.target.value = e.target.value.toUpperCase();
    });

    document.querySelectorAll('.btn-add-system').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var callsign = $('input-add-callsign').value.trim();
        if (!callsign) {
          showError('add-pilot-error', t('ERR_CALLSIGN_EMPTY'));
          return;
        }
        hideError('add-pilot-error');
        var system = btn.dataset.addSystem;

        // Highlight selected system.
        document.querySelectorAll('.btn-add-system').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');

        // Hide other system buttons to reduce clutter.
        document.querySelectorAll('.btn-add-system').forEach(function (b) {
          if (b !== btn) b.classList.add('hidden');
        });

        if (SIMPLE_SYSTEMS.indexOf(system) !== -1) {
          // No follow-up needed — add immediately.
          addPilot(callsign, system, false, '', 0, false, ['R']);
        } else {
          // Show follow-up options below the selected system.
          showAddPilotOptions(system);
        }
      });
    });

    // FCC buttons
    document.querySelectorAll('.btn-add-fcc').forEach(function (btn) {
      btn.addEventListener('click', function () {
        document.querySelectorAll('.btn-add-fcc').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        addPilotState.fccUnlocked = btn.dataset.addFcc === 'true';
        // DJI O4: show goggles after FCC
        if (addPilotState.system === 'dji_o4') {
          $('add-pilot-goggles').classList.remove('hidden');
        }
        updateAddPilotRaceMode();
      });
    });

    // Goggles buttons (DJI O4)
    document.querySelectorAll('.btn-add-goggles').forEach(function (btn) {
      btn.addEventListener('click', function () {
        document.querySelectorAll('.btn-add-goggles').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        addPilotState.goggles = btn.dataset.addGoggles;
        // Show bandwidth after goggles
        $('add-pilot-bw').classList.remove('hidden');
        updateAddPilotRaceMode();
      });
    });

    // Race mode buttons (DJI O4)
    document.querySelectorAll('.btn-add-racemode').forEach(function (btn) {
      btn.addEventListener('click', function () {
        document.querySelectorAll('.btn-add-racemode').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        addPilotState.raceMode = btn.dataset.addRacemode === 'true';
      });
    });

    // Analog band toggles for add-pilot
    document.querySelectorAll('.add-band-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        btn.classList.toggle('active');
        var selected = [];
        document.querySelectorAll('.add-band-btn.active').forEach(function (b) {
          selected.push(b.dataset.addBand);
        });
        if (selected.length === 0) {
          btn.classList.add('active');
          selected.push(btn.dataset.addBand);
        }
        addPilotState.analogBands = selected;
      });
    });

    // Confirm button
    $('btn-add-pilot-confirm').addEventListener('click', function () {
      var callsign = $('input-add-callsign').value.trim();
      if (!callsign) {
        showError('add-pilot-error', t('ERR_CALLSIGN_EMPTY'));
        return;
      }
      hideError('add-pilot-error');
      addPilot(callsign, addPilotState.system, addPilotState.fccUnlocked, addPilotState.goggles, addPilotState.bandwidthMHz, addPilotState.raceMode, addPilotState.analogBands);
    });

    $('btn-add-pilot-cancel').addEventListener('click', hideAddPilotDialog);
    $('add-pilot').addEventListener('click', function (e) {
      if (e.target === $('add-pilot')) hideAddPilotDialog();
    });
  }

  async function addPilot(callsign, videoSystem, fccUnlocked, goggles, bandwidthMHz, raceMode, analogBands) {
    try {
      await apiPost('/api/sessions/' + state.sessionCode + '/add-pilot', {
        callsign: callsign,
        video_system: videoSystem,
        fcc_unlocked: fccUnlocked || false,
        goggles: goggles || '',
        bandwidth_mhz: bandwidthMHz || 0,
        race_mode: raceMode || false,
        analog_bands: analogBands || ['R'],
      });
      hideAddPilotDialog();
      refreshSession();
    } catch (err) {
      var msg = err.message || '';
      if (msg.includes('callsign already') || msg.includes('409')) {
        showError('add-pilot-error', t('ERR_CALLSIGN_IN_SESSION'));
      } else {
        showError('add-pilot-error', t('ERR_FAILED', { error: msg.toUpperCase() }));
      }
    }
  }

  // ── Leader Leave Dialog ──────────────────────────────────────
  var cachedPilotsForLeave = null;

  function showLeaderLeaveDialog() {
    // Fetch pilot list for transfer options
    var pilotListEl = $('leader-leave-pilots');
    clearChildren(pilotListEl);

    apiGet('/api/sessions/' + state.sessionCode).then(function (data) {
      var pilots = (data.pilots || []).filter(function (p) { return p.ID !== state.pilotId; });
      cachedPilotsForLeave = pilots;

      if (pilots.length === 0) {
        // No one to transfer to
        var note = el('p', { className: 'leader-leave-text', textContent: t('LEADER_LEAVE_NO_PILOTS') });
        pilotListEl.appendChild(note);
      } else {
        pilots.forEach(function (p) {
          var btn = el('button', {
            className: 'btn btn-secondary btn-large',
            textContent: t('TRANSFER_TO', { callsign: p.Callsign })
          });
          btn.addEventListener('click', function () {
            transferAndLeave(p.ID);
          });
          pilotListEl.appendChild(btn);
        });
      }

      $('leader-leave').classList.remove('hidden');
    }).catch(function () {
      // If fetch fails, just show leave anyway
      $('leader-leave').classList.remove('hidden');
    });
  }

  function hideLeaderLeaveDialog() {
    $('leader-leave').classList.add('hidden');
    cachedPilotsForLeave = null;
  }

  async function transferAndLeave(newLeaderId) {
    hideLeaderLeaveDialog();
    try {
      await apiPost('/api/sessions/' + state.sessionCode + '/transfer-leader', { pilot_id: newLeaderId });
    } catch (err) {
      // Continue with leave even if transfer fails
    }
    await doLeave();
  }

  function initLeaderLeaveDialog() {
    $('btn-leave-anyway').addEventListener('click', function () {
      hideLeaderLeaveDialog();
      doLeave();
    });
    $('btn-leader-leave-cancel').addEventListener('click', hideLeaderLeaveDialog);
    $('leader-leave').addEventListener('click', function (e) {
      if (e.target === $('leader-leave')) hideLeaderLeaveDialog();
    });
  }

  // ── QR Code ───────────────────────────────────────────────────
  function showQROverlay() {
    var overlay = $('qr-overlay');
    overlay.classList.remove('hidden');

    var url = window.location.origin + '/s/' + state.sessionCode;
    $('qr-url').textContent = url;

    generateQR(url);
  }

  function hideQROverlay() {
    $('qr-overlay').classList.add('hidden');
  }

  // Minimal QR Code generator using canvas
  // Falls back to displaying the code in large text if generation fails
  function generateQR(text) {
    var canvas = $('qr-canvas');
    var ctx = canvas.getContext('2d');

    try {
      var matrix = QRCode.generate(text);
      var size = matrix.length;
      var padding = 4;
      var totalModules = size + padding * 2;
      var canvasSize = 280;
      canvas.width = canvasSize;
      canvas.height = canvasSize;
      var moduleSize = canvasSize / totalModules;

      // White background
      ctx.fillStyle = '#ffffff';
      ctx.fillRect(0, 0, canvasSize, canvasSize);

      // Draw modules
      ctx.fillStyle = '#000000';
      for (var y = 0; y < size; y++) {
        for (var x = 0; x < size; x++) {
          if (matrix[y][x]) {
            ctx.fillRect(
              (x + padding) * moduleSize,
              (y + padding) * moduleSize,
              moduleSize + 0.5,
              moduleSize + 0.5
            );
          }
        }
      }
    } catch (err) {
      // Fallback: show the code as large text on canvas
      canvas.width = 280;
      canvas.height = 280;
      ctx.fillStyle = '#ffffff';
      ctx.fillRect(0, 0, 280, 280);
      ctx.fillStyle = '#000000';
      ctx.font = '900 48px -apple-system, BlinkMacSystemFont, sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(state.sessionCode, 140, 130);
      ctx.font = '700 16px -apple-system, BlinkMacSystemFont, sans-serif';
      ctx.fillText(t('QR_SHARE_CODE'), 140, 180);
    }
  }

  // ── Minimal QR Code Library ───────────────────────────────────
  // A compact QR code generator supporting alphanumeric and byte modes
  // Generates Version 1-6 QR codes with error correction level L
  var QRCode = (function () {
    // Galois Field GF(256) arithmetic for Reed-Solomon
    var GF_EXP = new Uint8Array(512);
    var GF_LOG = new Uint8Array(256);
    (function initGF() {
      var x = 1;
      for (var i = 0; i < 255; i++) {
        GF_EXP[i] = x;
        GF_LOG[x] = i;
        x = x * 2;
        if (x >= 256) x ^= 0x11d;
      }
      for (var i = 255; i < 512; i++) GF_EXP[i] = GF_EXP[i - 255];
    })();

    function gfMul(a, b) {
      if (a === 0 || b === 0) return 0;
      return GF_EXP[GF_LOG[a] + GF_LOG[b]];
    }

    function rsGenPoly(nsym) {
      var g = [1];
      for (var i = 0; i < nsym; i++) {
        var ng = new Array(g.length + 1).fill(0);
        for (var j = 0; j < g.length; j++) {
          ng[j] ^= g[j];
          ng[j + 1] ^= gfMul(g[j], GF_EXP[i]);
        }
        g = ng;
      }
      return g;
    }

    function rsEncode(data, nsym) {
      var gen = rsGenPoly(nsym);
      var out = new Uint8Array(data.length + nsym);
      out.set(data);
      for (var i = 0; i < data.length; i++) {
        var coef = out[i];
        if (coef !== 0) {
          for (var j = 0; j < gen.length; j++) {
            out[i + j] ^= gfMul(gen[j], coef);
          }
        }
      }
      return out.slice(data.length);
    }

    // QR code version parameters for error correction level L
    var VERSION_INFO = {
      1: { total: 26, ecPerBlock: 7, blocks: 1, dataCW: 19 },
      2: { total: 44, ecPerBlock: 10, blocks: 1, dataCW: 34 },
      3: { total: 70, ecPerBlock: 15, blocks: 1, dataCW: 55 },
      4: { total: 100, ecPerBlock: 20, blocks: 1, dataCW: 80 },
      5: { total: 134, ecPerBlock: 26, blocks: 1, dataCW: 108 },
      6: { total: 172, ecPerBlock: 18, blocks: 2, dataCW: 136 },
    };

    // Alphanumeric encoding table
    var ALPHANUM = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:';

    function encodeAlphanumeric(str) {
      str = str.toUpperCase();
      var bits = [];
      for (var i = 0; i < str.length; i += 2) {
        if (i + 1 < str.length) {
          var val = ALPHANUM.indexOf(str[i]) * 45 + ALPHANUM.indexOf(str[i + 1]);
          bits.push({ val: val, len: 11 });
        } else {
          bits.push({ val: ALPHANUM.indexOf(str[i]), len: 6 });
        }
      }
      return bits;
    }

    function encodeByte(str) {
      var bits = [];
      var encoder = new TextEncoder();
      var bytes = encoder.encode(str);
      for (var k = 0; k < bytes.length; k++) {
        bits.push({ val: bytes[k], len: 8 });
      }
      return bits;
    }

    function canAlphanumeric(str) {
      return str.toUpperCase().split('').every(function (c) { return ALPHANUM.includes(c); });
    }

    function chooseVersion(dataLen, isAlphanumeric) {
      for (var v = 1; v <= 6; v++) {
        var info = VERSION_INFO[v];
        var dataBits = info.dataCW * 8;
        var ccBits = v <= 1 ? (isAlphanumeric ? 9 : 8) : (isAlphanumeric ? 9 : 16);
        var needed = 4 + ccBits;
        if (isAlphanumeric) {
          needed += Math.floor(dataLen / 2) * 11 + (dataLen % 2 ? 6 : 0);
        } else {
          needed += dataLen * 8;
        }
        if (needed <= dataBits) return v;
      }
      return 6;
    }

    function buildDataCodewords(text, version) {
      var info = VERSION_INFO[version];
      var totalBits = info.dataCW * 8;
      var isAlpha = canAlphanumeric(text);
      var mode = isAlpha ? 0x02 : 0x04;

      var bitStream = [];

      function push(val, len) {
        for (var i = len - 1; i >= 0; i--) {
          bitStream.push((val >> i) & 1);
        }
      }

      // Mode indicator
      push(mode, 4);

      // Character count
      var ccBits = version <= 1 ? (isAlpha ? 9 : 8) : (isAlpha ? 9 : 16);
      var charCount = isAlpha ? text.length : new TextEncoder().encode(text).length;
      push(charCount, ccBits);

      // Data
      if (isAlpha) {
        encodeAlphanumeric(text).forEach(function (e) { push(e.val, e.len); });
      } else {
        encodeByte(text).forEach(function (e) { push(e.val, e.len); });
      }

      // Terminator
      var remaining = totalBits - bitStream.length;
      var termLen = Math.min(4, remaining);
      push(0, termLen);

      // Pad to byte boundary
      while (bitStream.length % 8 !== 0) bitStream.push(0);

      // Pad codewords
      var padWords = [0xec, 0x11];
      var padIdx = 0;
      while (bitStream.length < totalBits) {
        push(padWords[padIdx % 2], 8);
        padIdx++;
      }

      // Convert to bytes
      var codewords = new Uint8Array(info.dataCW);
      for (var i = 0; i < info.dataCW; i++) {
        var byte = 0;
        for (var b = 0; b < 8; b++) {
          byte = (byte << 1) | (bitStream[i * 8 + b] || 0);
        }
        codewords[i] = byte;
      }

      return codewords;
    }

    function buildFinalMessage(dataCW, version) {
      var info = VERSION_INFO[version];

      if (info.blocks === 1) {
        var ec = rsEncode(dataCW, info.ecPerBlock);
        var result = new Uint8Array(dataCW.length + ec.length);
        result.set(dataCW);
        result.set(ec, dataCW.length);
        return result;
      }

      // Multi-block: split data and interleave
      var blockSize = Math.floor(info.dataCW / info.blocks);
      var extraBytes = info.dataCW % info.blocks;
      var dataBlocks = [];
      var ecBlocks = [];
      var offset = 0;

      for (var i = 0; i < info.blocks; i++) {
        var sz = blockSize + (i >= info.blocks - extraBytes ? 1 : 0);
        var block = dataCW.slice(offset, offset + sz);
        dataBlocks.push(block);
        ecBlocks.push(rsEncode(block, info.ecPerBlock));
        offset += sz;
      }

      // Interleave data
      var resultArr = [];
      var maxDataLen = Math.max.apply(null, dataBlocks.map(function (b) { return b.length; }));
      for (var i = 0; i < maxDataLen; i++) {
        for (var j = 0; j < info.blocks; j++) {
          if (i < dataBlocks[j].length) resultArr.push(dataBlocks[j][i]);
        }
      }
      // Interleave EC
      for (var i = 0; i < info.ecPerBlock; i++) {
        for (var j = 0; j < info.blocks; j++) {
          resultArr.push(ecBlocks[j][i]);
        }
      }

      return new Uint8Array(resultArr);
    }

    // Matrix construction
    function createMatrix(version) {
      var size = 17 + version * 4;
      var matrix = [];
      var reserved = [];
      for (var i = 0; i < size; i++) {
        matrix.push(new Uint8Array(size));
        reserved.push(new Uint8Array(size));
      }
      return { matrix: matrix, reserved: reserved, size: size };
    }

    function addFinderPattern(m, row, col) {
      for (var r = -1; r <= 7; r++) {
        for (var c = -1; c <= 7; c++) {
          var rr = row + r, cc = col + c;
          if (rr < 0 || rr >= m.size || cc < 0 || cc >= m.size) continue;
          m.reserved[rr][cc] = 1;
          if (r >= 0 && r <= 6 && c >= 0 && c <= 6) {
            if (
              r === 0 || r === 6 || c === 0 || c === 6 ||
              (r >= 2 && r <= 4 && c >= 2 && c <= 4)
            ) {
              m.matrix[rr][cc] = 1;
            } else {
              m.matrix[rr][cc] = 0;
            }
          } else {
            m.matrix[rr][cc] = 0;
          }
        }
      }
    }

    function addAlignmentPattern(m, row, col) {
      for (var r = -2; r <= 2; r++) {
        for (var c = -2; c <= 2; c++) {
          var rr = row + r, cc = col + c;
          if (rr < 0 || rr >= m.size || cc < 0 || cc >= m.size) continue;
          if (m.reserved[rr][cc]) continue;
          m.reserved[rr][cc] = 1;
          if (Math.abs(r) === 2 || Math.abs(c) === 2 || (r === 0 && c === 0)) {
            m.matrix[rr][cc] = 1;
          } else {
            m.matrix[rr][cc] = 0;
          }
        }
      }
    }

    var ALIGNMENT_POSITIONS = {
      2: [6, 18],
      3: [6, 22],
      4: [6, 26],
      5: [6, 30],
      6: [6, 34],
    };

    function addTimingPatterns(m) {
      for (var i = 8; i < m.size - 8; i++) {
        if (!m.reserved[6][i]) {
          m.reserved[6][i] = 1;
          m.matrix[6][i] = i % 2 === 0 ? 1 : 0;
        }
        if (!m.reserved[i][6]) {
          m.reserved[i][6] = 1;
          m.matrix[i][6] = i % 2 === 0 ? 1 : 0;
        }
      }
    }

    function reserveFormatArea(m) {
      for (var i = 0; i <= 8; i++) {
        if (i < m.size) m.reserved[8][i] = 1;
        if (i < m.size) m.reserved[i][8] = 1;
      }
      for (var i = 0; i <= 7; i++) {
        m.reserved[8][m.size - 1 - i] = 1;
      }
      for (var i = 0; i <= 7; i++) {
        m.reserved[m.size - 1 - i][8] = 1;
      }
      // Dark module
      m.matrix[m.size - 8][8] = 1;
      m.reserved[m.size - 8][8] = 1;
    }

    function placeData(m, data) {
      var totalBits = data.length * 8;
      var bitIdx = 0;
      var x = m.size - 1;
      var upward = true;

      while (x >= 0) {
        if (x === 6) x--;
        if (x < 0) break;

        var rows;
        if (upward) {
          rows = [];
          for (var i = m.size - 1; i >= 0; i--) rows.push(i);
        } else {
          rows = [];
          for (var i = 0; i < m.size; i++) rows.push(i);
        }

        for (var ri = 0; ri < rows.length; ri++) {
          var y = rows[ri];
          for (var dx = 0; dx <= 1; dx++) {
            var col = x - dx;
            if (col < 0) continue;
            if (m.reserved[y][col]) continue;
            if (bitIdx < totalBits) {
              var byteIdx = Math.floor(bitIdx / 8);
              var bitPos = 7 - (bitIdx % 8);
              m.matrix[y][col] = (data[byteIdx] >> bitPos) & 1;
              bitIdx++;
            }
          }
        }

        x -= 2;
        upward = !upward;
      }
    }

    // Masking
    var MASK_FUNCTIONS = [
      function (r, c) { return (r + c) % 2 === 0; },
      function (r, c) { return r % 2 === 0; },
      function (r, c) { return c % 3 === 0; },
      function (r, c) { return (r + c) % 3 === 0; },
      function (r, c) { return (Math.floor(r / 2) + Math.floor(c / 3)) % 2 === 0; },
      function (r, c) { return ((r * c) % 2 + (r * c) % 3) === 0; },
      function (r, c) { return ((r * c) % 2 + (r * c) % 3) % 2 === 0; },
      function (r, c) { return ((r + c) % 2 + (r * c) % 3) % 2 === 0; },
    ];

    function applyMask(m, maskIdx) {
      var fn = MASK_FUNCTIONS[maskIdx];
      for (var r = 0; r < m.size; r++) {
        for (var c = 0; c < m.size; c++) {
          if (!m.reserved[r][c] && fn(r, c)) {
            m.matrix[r][c] ^= 1;
          }
        }
      }
    }

    function calcPenalty(m) {
      var penalty = 0;
      var s = m.size;

      // Rule 1: Runs of same color
      for (var r = 0; r < s; r++) {
        var count = 1;
        for (var c = 1; c < s; c++) {
          if (m.matrix[r][c] === m.matrix[r][c - 1]) {
            count++;
            if (count === 5) penalty += 3;
            else if (count > 5) penalty++;
          } else {
            count = 1;
          }
        }
      }
      for (var c = 0; c < s; c++) {
        var count = 1;
        for (var r = 1; r < s; r++) {
          if (m.matrix[r][c] === m.matrix[r - 1][c]) {
            count++;
            if (count === 5) penalty += 3;
            else if (count > 5) penalty++;
          } else {
            count = 1;
          }
        }
      }

      // Rule 2: 2x2 blocks
      for (var r = 0; r < s - 1; r++) {
        for (var c = 0; c < s - 1; c++) {
          var v = m.matrix[r][c];
          if (v === m.matrix[r][c + 1] && v === m.matrix[r + 1][c] && v === m.matrix[r + 1][c + 1]) {
            penalty += 3;
          }
        }
      }

      return penalty;
    }

    // Format info (L error correction)
    var FORMAT_STRINGS = [
      0x77c4, 0x72f3, 0x7daa, 0x789d, 0x662f, 0x6318, 0x6c41, 0x6976,
    ];

    function writeFormatInfo(m, maskIdx) {
      var bits = FORMAT_STRINGS[maskIdx];

      var hPositions = [0, 1, 2, 3, 4, 5, 7, 8, m.size - 8, m.size - 7, m.size - 6, m.size - 5, m.size - 4, m.size - 3, m.size - 2];
      for (var i = 0; i < 15; i++) {
        m.matrix[8][hPositions[i]] = (bits >> (14 - i)) & 1;
      }

      var vPositions = [0, 1, 2, 3, 4, 5, 7, 8].map(function (p) { return m.size - 1 - p; }).concat([7, 5, 4, 3, 2, 1, 0]);
      for (var i = 0; i < 15; i++) {
        m.matrix[vPositions[i]][8] = (bits >> (14 - i)) & 1;
      }
    }

    return {
      generate: function (text) {
        var isAlpha = canAlphanumeric(text);
        var version = chooseVersion(
          isAlpha ? text.length : new TextEncoder().encode(text).length,
          isAlpha
        );
        var dataCW = buildDataCodewords(text, version);
        var finalMsg = buildFinalMessage(dataCW, version);

        var m = createMatrix(version);

        // Add finder patterns
        addFinderPattern(m, 0, 0);
        addFinderPattern(m, 0, m.size - 7);
        addFinderPattern(m, m.size - 7, 0);

        // Add alignment patterns (version >= 2)
        if (ALIGNMENT_POSITIONS[version]) {
          var pos = ALIGNMENT_POSITIONS[version];
          for (var i = 0; i < pos.length; i++) {
            for (var j = 0; j < pos.length; j++) {
              if (i === 0 && j === 0) continue;
              if (i === 0 && j === pos.length - 1) continue;
              if (i === pos.length - 1 && j === 0) continue;
              addAlignmentPattern(m, pos[i], pos[j]);
            }
          }
        }

        addTimingPatterns(m);
        reserveFormatArea(m);
        placeData(m, finalMsg);

        // Try all masks and pick the best
        var bestMask = 0;
        var bestPenalty = Infinity;

        for (var mask = 0; mask < 8; mask++) {
          var clone = createMatrix(version);
          for (var r = 0; r < m.size; r++) {
            for (var c = 0; c < m.size; c++) {
              clone.matrix[r][c] = m.matrix[r][c];
              clone.reserved[r][c] = m.reserved[r][c];
            }
          }
          applyMask(clone, mask);
          writeFormatInfo(clone, mask);
          var p = calcPenalty(clone);
          if (p < bestPenalty) {
            bestPenalty = p;
            bestMask = mask;
          }
        }

        applyMask(m, bestMask);
        writeFormatInfo(m, bestMask);

        // Convert to boolean matrix for rendering
        var result = [];
        for (var r = 0; r < m.size; r++) {
          var row = [];
          for (var c = 0; c < m.size; c++) {
            row.push(m.matrix[r][c] === 1);
          }
          result.push(row);
        }
        return result;
      },
    };
  })();

  // ── Client-side routing ───────────────────────────────────────
  function route() {
    var path = window.location.pathname;
    var match = path.match(/^\/[sS]\/([A-Fa-f0-9]{6})$/);

    if (match) {
      var code = match[1].toUpperCase();
      loadState();

      if (state.sessionCode === code && state.pilotId) {
        // Already joined this session, go to session view
        enterSessionView();
      } else {
        // Need to join this session
        state.sessionCode = code;
        state.sessionPowerCeiling = 0;
        state.sessionFixedChannels = '';
        saveState();
        $('joining-session-code').textContent = code;
        $('joining-session-hint').classList.remove('hidden');
        showScreen('setup');
        showStep('step-callsign');
        $('input-callsign').focus();
        // Fetch session in background to get power ceiling and fixed channels for alert steps
        apiGet('/api/sessions/' + code).then(function (data) {
          state.sessionPowerCeiling = (data.session && data.session.power_ceiling_mw) || 0;
          state.sessionFixedChannels = (data.session && data.session.fixed_channels) || '';
        }).catch(function () {
          state.sessionPowerCeiling = 0;
          state.sessionFixedChannels = '';
        });
      }
      return;
    }

    // Root path
    loadState();
    if (state.sessionCode && state.pilotId) {
      // Returning user with active session
      enterSessionView();
    } else {
      // Validate recent sessions before showing landing
      validateAndShowLanding();
    }
  }

  async function validateAndShowLanding() {
    var recent = getRecentSessions();
    if (recent.length === 0) {
      renderRecentSessions([]);
      showScreen('landing');
      return;
    }

    // Show landing with buttons hidden while validating
    showScreen('landing');
    document.querySelector('.landing-buttons').style.display = 'none';
    $('recent-sessions').classList.add('hidden');

    // Validate all sessions in parallel
    try {
      var validated = [];
      var results = await Promise.allSettled(recent.map(function (entry) {
        return apiGet('/api/sessions/' + entry.code).then(function () {
          return entry;
        });
      }));
      results.forEach(function (result) {
        if (result.status === 'fulfilled') {
          validated.push(result.value);
        }
      });

      // Update localStorage with only valid sessions
      setRecentSessions(validated);
      renderRecentSessions(validated);
    } catch (e) {
      renderRecentSessions([]);
    } finally {
      document.querySelector('.landing-buttons').style.display = '';
    }
  }

  function renderRecentSessions(sessions) {
    var container = $('recent-sessions');
    var list = $('recent-sessions-list');
    clearChildren(list);

    if (sessions.length === 0) {
      container.classList.add('hidden');
      return;
    }

    sessions.forEach(function (entry) {
      var btn = document.createElement('button');
      btn.className = 'btn-recent-session';

      var codeSpan = document.createElement('span');
      codeSpan.className = 'recent-session-code';
      codeSpan.textContent = entry.code;

      var sepSpan = document.createElement('span');
      sepSpan.className = 'recent-session-sep';
      sepSpan.textContent = '\u2014';

      var callsignSpan = document.createElement('span');
      callsignSpan.className = 'recent-session-callsign';
      callsignSpan.textContent = entry.callsign;

      btn.appendChild(codeSpan);
      btn.appendChild(sepSpan);
      btn.appendChild(callsignSpan);

      btn.addEventListener('click', function () {
        state.sessionCode = entry.code;
        state.pilotId = null;
        saveState();
        state.callsign = entry.callsign;
        $('input-callsign').value = entry.callsign;
        showScreen('setup');
        showStep('step-callsign');
      });
      list.appendChild(btn);
    });

    container.classList.remove('hidden');
  }

  // ── Service Worker & Install Prompt ─────────────────────────
  var deferredInstallPrompt = null;

  function initServiceWorker() {
    if (!('serviceWorker' in navigator)) return;
    navigator.serviceWorker.register('/sw.js').catch(function () {});

    // Capture the install prompt (Android Chrome).
    window.addEventListener('beforeinstallprompt', function (e) {
      e.preventDefault();
      deferredInstallPrompt = e;
      showInstallBanner();
    });

    // Hide banner if already installed as PWA.
    window.addEventListener('appinstalled', function () {
      hideInstallBanner();
      deferredInstallPrompt = null;
    });
  }

  function showInstallBanner() {
    var banner = $('install-banner');
    if (banner) banner.classList.remove('hidden');
  }

  function hideInstallBanner() {
    var banner = $('install-banner');
    if (banner) banner.classList.add('hidden');
  }

  function initInstallBanner() {
    var btnInstall = $('btn-install');
    var btnDismiss = $('btn-install-dismiss');
    if (btnInstall) {
      btnInstall.addEventListener('click', function () {
        if (deferredInstallPrompt) {
          deferredInstallPrompt.prompt();
          deferredInstallPrompt.userChoice.then(function () {
            deferredInstallPrompt = null;
            hideInstallBanner();
          });
        }
      });
    }
    if (btnDismiss) {
      btnDismiss.addEventListener('click', hideInstallBanner);
    }

    // iOS detection: show a hint since iOS has no beforeinstallprompt.
    var isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent) ||
      (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
    var isStandalone = window.matchMedia('(display-mode: standalone)').matches ||
      window.navigator.standalone;
    if (isIOS && !isStandalone) {
      var banner = $('install-banner');
      var btnInstall = $('btn-install');
      var iosHint = $('ios-install-hint');
      if (banner) banner.classList.remove('hidden');
      if (btnInstall) btnInstall.classList.add('hidden');
      if (iosHint) iosHint.classList.remove('hidden');
    }
  }

  function initFeedbackScreen() {
    // Feedback link on QR overlay
    $('link-feedback-qr').addEventListener('click', function (e) {
      e.preventDefault();
      state.feedbackReturnTo = 'qr-overlay';
      $('qr-overlay').classList.add('hidden');
      openFeedbackScreen();
    });

    // Feedback back button
    $('btn-feedback-back').addEventListener('click', function () {
      closeFeedbackScreen();
    });

    // Feedback category buttons
    document.querySelectorAll('.feedback-cat').forEach(function (btn) {
      btn.addEventListener('click', function () {
        document.querySelectorAll('.feedback-cat').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        state.feedbackType = btn.getAttribute('data-type');
        updateFeedbackPlaceholder();
      });
    });

    // Feedback textarea — enable/disable submit
    $('feedback-text').addEventListener('input', function () {
      $('btn-feedback-submit').disabled = !this.value.trim();
    });

    // Feedback submit
    $('btn-feedback-submit').addEventListener('click', submitFeedback);
  }

  function init() {
    initLanding();
    initCallsignStep();
    initLeaderInfoStep();
    initPowerStep();
    initFixedChannelsStep();
    initPowerAlertStep();
    initVideoStep();
    initFollowUpStep();
    initChannelStep();
    initSessionView();
    initPilotActions();
    initChannelChangeOptions();
    initChannelChange();
    initMovedDialog();
    initCallsignChange();
    initOtherPilotActions();
    initChannelChangeBanner();
    initLeaderControls();
    initAddPilotDialog();
    initLeaderLeaveDialog();
    initInstallBanner();
    initFeedbackScreen();
    initServiceWorker();
    route();
  }

  // ── Language Picker ─────────────────────────────────────────────
  onI18nReady(function () {
    var selectors = document.querySelectorAll('.lang-select');
    selectors.forEach(function (sel) {
      getSupportedLanguages().forEach(function (code) {
        var opt = document.createElement('option');
        opt.value = code;
        opt.textContent = getLanguageName(code);
        if (code === getCurrentLanguage()) opt.selected = true;
        sel.appendChild(opt);
      });
      sel.addEventListener('change', function () { setLanguage(sel.value); });
    });
  });

  // Re-render dynamic content when language changes
  window.addEventListener('skwad-languagechange', function () {
    if (state.sessionCode) refreshSession();
  });

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
