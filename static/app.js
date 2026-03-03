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
    walksnailMode: '', // 'standard' or 'race'
    channelLocked: false,
    lockedFreqMHz: 0,
    // Tracked assignment for change detection
    myChannel: null,
    myFreqMHz: null,
  };

  // ── Buddy group colors ────────────────────────────────────────
  const BUDDY_COLORS = [
    '', '#ff3333', '#33ff33', '#3399ff', '#ffcc00',
    '#ff66ff', '#00ffcc', '#ff9900', '#cc66ff'
  ];

  // ── Video system display names ────────────────────────────────
  const SYSTEM_LABELS = {
    analog: 'ANALOG',
    dji_v1: 'DJI V1',
    dji_o3: 'DJI O3',
    dji_o4: 'DJI O4',
    hdzero: 'HDZERO',
    walksnail_std: 'WALKSNAIL',
    walksnail_race: 'WALKSNAIL RACE',
    openipc: 'OPENIPC',
  };

  // ── Channel tables (mirrors Go freq/tables.go) ────────────────
  const CHANNELS = {
    raceband: [
      { name: 'R1', freq: 5658 }, { name: 'R2', freq: 5695 },
      { name: 'R3', freq: 5732 }, { name: 'R4', freq: 5769 },
      { name: 'R5', freq: 5806 }, { name: 'R6', freq: 5843 },
      { name: 'R7', freq: 5880 }, { name: 'R8', freq: 5917 },
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
      { name: 'O3-CH3', freq: 5741 }, { name: 'O3-CH4', freq: 5769 },
      { name: 'O3-CH5', freq: 5805 }, { name: 'O3-CH6', freq: 5840 },
      { name: 'O3-CH7', freq: 5876 },
    ],
    dji_o3_40: [{ name: 'O3-CH1', freq: 5795 }],
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

  // ── DOM references ────────────────────────────────────────────
  const $ = (id) => document.getElementById(id);
  const screens = {
    landing: $('screen-landing'),
    setup: $('screen-setup'),
    session: $('screen-session'),
  };

  // ── Helpers ───────────────────────────────────────────────────
  function showScreen(name) {
    Object.values(screens).forEach((s) => s.classList.add('hidden'));
    screens[name].classList.remove('hidden');
  }

  function showStep(stepId) {
    document.querySelectorAll('.setup-step').forEach((s) => s.classList.add('hidden'));
    $(stepId).classList.remove('hidden');
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
  async function apiPost(path, body) {
    const res = await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text.trim() || ('HTTP ' + res.status));
    }
    return res.json();
  }

  async function apiGet(path) {
    const res = await fetch(path);
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text.trim() || ('HTTP ' + res.status));
    }
    return res.json();
  }

  async function apiPut(path, body) {
    const res = await fetch(path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text.trim() || ('HTTP ' + res.status));
    }
  }

  async function apiDelete(path) {
    const res = await fetch(path, { method: 'DELETE' });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text.trim() || ('HTTP ' + res.status));
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
    localStorage.removeItem('skwad_session');
    localStorage.removeItem('skwad_pilot');
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
      case 'hdzero':
        return CHANNELS.raceband;
      case 'dji_v1':
        return fcc ? CHANNELS.dji_v1_fcc : CHANNELS.dji_v1_stock;
      case 'dji_o3':
        if (bw >= 40) return CHANNELS.dji_o3_40;
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
      e.target.value = e.target.value.toUpperCase().replace(/[^A-F0-9]/g, '');
    });
  }

  async function handleStartSession() {
    var btn = $('btn-start');
    setLoading(btn, true);
    hideError('landing-error');
    try {
      var sess = await apiPost('/api/sessions');
      state.sessionCode = sess.ID;
      saveState();
      showScreen('setup');
      showStep('step-callsign');
      $('input-callsign').focus();
    } catch (err) {
      showError('landing-error', 'FAILED TO CREATE SESSION');
    } finally {
      setLoading(btn, false);
    }
  }

  async function handleJoinByCode() {
    var code = $('input-code').value.trim().toUpperCase();
    if (code.length !== 6) {
      showError('landing-error', 'CODE MUST BE 6 CHARACTERS');
      return;
    }
    var btn = $('btn-go');
    setLoading(btn, true);
    hideError('landing-error');
    try {
      // Verify the session exists
      await apiGet('/api/sessions/' + code);
      state.sessionCode = code;
      saveState();
      showScreen('setup');
      showStep('step-callsign');
      $('input-callsign').focus();
    } catch (err) {
      showError('landing-error', 'SESSION NOT FOUND');
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
        showError('callsign-error', 'ENTER YOUR CALLSIGN');
        return;
      }
      hideError('callsign-error');
      state.callsign = cs;
      showStep('step-video');
    });
    $('input-callsign').addEventListener('keydown', function (e) {
      if (e.key === 'Enter') $('btn-callsign-next').click();
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
  }

  // ── Setup: Step 3 — Follow-ups (conditional) ──────────────────
  function startFollowUpFlow(system) {
    // Hide all follow-up groups
    document.querySelectorAll('.followup-group').forEach(function (g) { g.classList.add('hidden'); });
    $('btn-followup-next').classList.add('hidden');
    resetOptionButtons();

    // Systems with no follow-ups go straight to channel pref
    if (['analog', 'hdzero', 'openipc'].includes(system)) {
      goToChannelStep();
      return;
    }

    showStep('step-followup');

    if (system === 'walksnail') {
      $('followup-title').textContent = 'WALKSNAIL SETTINGS';
      $('followup-walksnail-mode').classList.remove('hidden');
    } else if (system === 'dji_v1') {
      $('followup-title').textContent = 'DJI V1 SETTINGS';
      $('followup-fcc').classList.remove('hidden');
    } else if (system === 'dji_o3') {
      $('followup-title').textContent = 'DJI O3 SETTINGS';
      $('followup-fcc').classList.remove('hidden');
    } else if (system === 'dji_o4') {
      $('followup-title').textContent = 'DJI O4 SETTINGS';
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

    // Next button for follow-up step
    $('btn-followup-next').addEventListener('click', goToChannelStep);
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

  function showBandwidthOptions(options) {
    $('followup-bandwidth').classList.remove('hidden');
    var container = $('bandwidth-buttons');
    clearChildren(container);
    options.forEach(function (bw) {
      var btn = document.createElement('button');
      btn.className = 'btn btn-option';
      btn.textContent = bw + ' MHz';
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

  // ── Setup: Step 4 — Channel Preference ────────────────────────
  function goToChannelStep() {
    state.channelLocked = false;
    state.lockedFreqMHz = 0;
    showStep('step-channel');
    $('btn-auto-channel').classList.add('active');
    $('btn-lock-channel').classList.remove('active');
    $('channel-picker').classList.add('hidden');
    renderChannelPicker();
  }

  function initChannelStep() {
    $('btn-auto-channel').addEventListener('click', function () {
      state.channelLocked = false;
      state.lockedFreqMHz = 0;
      $('btn-auto-channel').classList.add('active');
      $('btn-lock-channel').classList.remove('active');
      $('channel-picker').classList.add('hidden');
      // Deselect any selected channel
      document.querySelectorAll('.btn-channel').forEach(function (b) { b.classList.remove('selected'); });
    });

    $('btn-lock-channel').addEventListener('click', function () {
      $('btn-lock-channel').classList.add('active');
      $('btn-auto-channel').classList.remove('active');
      $('channel-picker').classList.remove('hidden');
      state.channelLocked = true;
    });

    $('btn-join-session').addEventListener('click', handleJoinSession);
    $('btn-channel-back').addEventListener('click', function () {
      showStep('step-video');
    });
  }

  function renderChannelPicker() {
    var pool = getChannelPool();
    var picker = $('channel-picker');
    clearChildren(picker);
    pool.forEach(function (ch) {
      var nameSpan = el('span', { className: 'ch-name', textContent: ch.name });
      var freqSpan = el('span', { className: 'ch-freq', textContent: String(ch.freq) });
      var btn = el('button', { className: 'btn-channel' }, [nameSpan, freqSpan]);
      btn.addEventListener('click', function () {
        picker.querySelectorAll('.btn-channel').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        state.lockedFreqMHz = ch.freq;
      });
      picker.appendChild(btn);
    });
  }

  function buildJoinBody() {
    return {
      callsign: state.callsign,
      video_system: getEffectiveVideoSystem(),
      fcc_unlocked: state.fccUnlocked,
      goggles: state.goggles,
      bandwidth_mhz: state.bandwidthMHz,
      race_mode: state.raceMode,
      channel_locked: state.channelLocked,
      locked_frequency_mhz: state.lockedFreqMHz,
    };
  }

  async function handleJoinSession() {
    if (state.channelLocked && !state.lockedFreqMHz) {
      showError('join-error', 'SELECT A CHANNEL');
      return;
    }
    hideError('join-error');

    var btn = $('btn-join-session');
    setLoading(btn, true);

    var body = buildJoinBody();

    try {
      // Preview first to check for displacements.
      var preview = await apiPost('/api/sessions/' + state.sessionCode + '/preview-join', body);
      var displaced = preview.displaced || [];

      if (displaced.length > 0) {
        // Show confirmation dialog — actual join happens on confirm.
        setLoading(btn, false);
        showDisplacementConfirm(displaced, preview.has_danger);
        return;
      }

      // No displacements — join immediately.
      await commitJoin(body);
    } catch (err) {
      var msg = err.message || '';
      if (msg.includes('callsign already')) {
        showError('join-error', 'CALLSIGN ALREADY IN SESSION');
      } else {
        showError('join-error', 'FAILED TO JOIN: ' + msg.toUpperCase());
      }
    } finally {
      setLoading(btn, false);
    }
  }

  async function commitJoin(body, rebalance) {
    var btn = $('btn-join-session');
    setLoading(btn, true);
    var rebalParam = (rebalance === false) ? '?rebalance=false' : '';
    try {
      var pilot = await apiPost('/api/sessions/' + state.sessionCode + '/join' + rebalParam, body);
      state.pilotId = pilot.ID;
      saveState();
      enterSessionView();
    } catch (err) {
      var msg = err.message || '';
      if (msg.includes('callsign already')) {
        showError('join-error', 'CALLSIGN ALREADY IN SESSION');
      } else {
        showError('join-error', 'FAILED TO JOIN: ' + msg.toUpperCase());
      }
    } finally {
      setLoading(btn, false);
    }
  }

  // ── Displacement Confirmation ──────────────────────────────
  function showDisplacementConfirm(displaced, hasDanger) {
    var list = $('displacement-list');
    clearChildren(list);

    displaced.forEach(function (d) {
      var nameEl = el('div', { className: 'displacement-name', textContent: d.callsign });
      var moveText = d.old_channel + ' (' + d.old_freq_mhz + ') \u2192 ' +
        d.new_channel + ' (' + d.new_freq_mhz + ')';
      var moveEl = el('div', { className: 'displacement-move', textContent: moveText });
      var item = el('div', { className: 'displacement-item' }, [nameEl, moveEl]);
      list.appendChild(item);
    });

    // Show/hide "JUST MOVE ME" — allowed unless it would cause overlap (danger)
    var justMeBtn = $('btn-displacement-just-me');
    if (hasDanger) {
      justMeBtn.classList.add('hidden');
    } else {
      justMeBtn.classList.remove('hidden');
    }

    $('displacement-confirm').classList.remove('hidden');
  }

  function hideDisplacementConfirm() {
    $('displacement-confirm').classList.add('hidden');
  }

  function initDisplacementConfirm() {
    // "MOVE EVERYONE" — full rebalance
    $('btn-displacement-confirm').addEventListener('click', function () {
      hideDisplacementConfirm();
      if (state.pendingChannelChange) {
        var body = state.pendingChannelChange;
        state.pendingChannelChange = null;
        commitChannelChange(body, true);
      } else {
        commitJoin(buildJoinBody(), true);
      }
    });
    // "JUST MOVE ME" — only apply my assignment, leave others
    $('btn-displacement-just-me').addEventListener('click', function () {
      hideDisplacementConfirm();
      if (state.pendingChannelChange) {
        var body = state.pendingChannelChange;
        state.pendingChannelChange = null;
        commitChannelChange(body, false);
      } else {
        commitJoin(buildJoinBody(), false);
      }
    });
    $('btn-displacement-cancel').addEventListener('click', function () {
      state.pendingChannelChange = null;
      hideDisplacementConfirm();
    });
    $('displacement-confirm').addEventListener('click', function (e) {
      if (e.target === $('displacement-confirm')) {
        state.pendingChannelChange = null;
        hideDisplacementConfirm();
      }
    });
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
      state.knownVersion = data.session.Version;

      // Detect if our channel was changed by the optimizer.
      if (state.pilotId && state.myChannel !== null) {
        var me = null;
        if (data.pilots) {
          for (var i = 0; i < data.pilots.length; i++) {
            if (data.pilots[i].ID === state.pilotId) {
              me = data.pilots[i];
              break;
            }
          }
        }
        if (me && me.AssignedChannel && me.AssignedChannel !== state.myChannel) {
          showChannelChangeBanner(state.myChannel, state.myFreqMHz, me.AssignedChannel, me.AssignedFreqMHz);
        }
      }

      // Update tracked assignment and sync callsign/video from server.
      if (state.pilotId && data.pilots) {
        for (var j = 0; j < data.pilots.length; j++) {
          if (data.pilots[j].ID === state.pilotId) {
            state.myChannel = data.pilots[j].AssignedChannel;
            state.myFreqMHz = data.pilots[j].AssignedFreqMHz;
            if (!state.callsign) state.callsign = data.pilots[j].Callsign;
            if (!state.videoSystem) state.videoSystem = data.pilots[j].VideoSystem;
            // Sync gear settings so channel picker works after page refresh.
            state.fccUnlocked = data.pilots[j].FCCUnlocked || false;
            state.bandwidthMHz = data.pilots[j].BandwidthMHz || 0;
            state.goggles = data.pilots[j].Goggles || '';
            state.raceMode = data.pilots[j].RaceMode || false;
            break;
          }
        }
      }

      renderPilotList(data.pilots);
    } catch (err) {
      // Session may have expired
      if (err.message && err.message.includes('not found')) {
        clearState();
        stopPolling();
        showScreen('landing');
      }
    }
  }

  // ── Channel Change Banner ──────────────────────────────────
  function showChannelChangeBanner(oldChannel, oldFreq, newChannel, newFreq) {
    var msg = 'YOUR CHANNEL CHANGED: ' + oldChannel + ' (' + oldFreq + ') \u2192 ' +
      newChannel + ' (' + newFreq + ')\nCOORDINATE WITH YOUR GROUP BEFORE SWITCHING';
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

  function renderSpectrum(pilots) {
    var canvas = $('spectrum-canvas');
    if (!canvas) return;
    var ctx = canvas.getContext('2d');
    var dpr = window.devicePixelRatio || 1;
    var rect = canvas.getBoundingClientRect();
    var w = rect.width * dpr;
    var h = 96 * dpr;
    canvas.width = w;
    canvas.height = h;
    ctx.scale(dpr, dpr);
    var cw = rect.width;
    var ch = 96;

    // Frequency range
    var fMin = 5640;
    var fMax = 5930;
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

    if (!pilots || pilots.length === 0) return;

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
      item.labelY = tierBaseY - tier * (labelH + 2);
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

    // Draw callsign labels
    ctx.font = '700 10px -apple-system, BlinkMacSystemFont, sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'bottom';
    items.forEach(function (item) {
      ctx.fillStyle = item.color;
      ctx.fillText(item.label, item.centerX, item.labelY);
    });
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
        el('div', { className: 'empty-state-text', textContent: 'WAITING FOR PILOTS...' })
      ]);
      container.appendChild(emptyDiv);
      $('pilot-count').textContent = '0 PILOTS';
      renderSpectrum([]);
      return;
    }

    // Sort by frequency (lowest first)
    pilots.sort(function (a, b) { return (a.AssignedFreqMHz || 0) - (b.AssignedFreqMHz || 0); });

    // Build buddy group map for "sharing with" labels
    var buddyGroups = {};
    pilots.forEach(function (p) {
      if (p.BuddyGroup && p.BuddyGroup > 0) {
        if (!buddyGroups[p.BuddyGroup]) buddyGroups[p.BuddyGroup] = [];
        buddyGroups[p.BuddyGroup].push(p);
      }
    });

    pilots.forEach(function (p) {
      var card = document.createElement('div');
      card.className = 'pilot-card';

      var isMe = p.ID === state.pilotId;
      var buddyIdx = p.BuddyGroup > 0 ? ((p.BuddyGroup - 1) % 8) + 1 : 0;

      if (isMe) {
        card.classList.add('is-me');
        card.addEventListener('click', function () {
          showPilotActions();
        });
      } else {
        card.classList.add('is-other');
        card.addEventListener('click', function () {
          showOtherPilotActions(p);
        });
      }

      if (buddyIdx > 0) {
        card.classList.add('buddy-group', 'buddy-' + buddyIdx);
      }

      // Determine worst conflict level for card styling
      var conflicts = p.Conflicts || p.conflicts || [];
      var worstLevel = null;
      conflicts.forEach(function (c) {
        if (c.level === 'danger' || c.Level === 'danger') worstLevel = 'danger';
        else if (!worstLevel && (c.level === 'warning' || c.Level === 'warning')) worstLevel = 'warning';
      });
      if (worstLevel === 'danger') {
        card.classList.add('has-conflict-danger');
      } else if (worstLevel === 'warning') {
        card.classList.add('has-conflict-warning');
      }

      // Frequency block
      var chEl = el('div', { className: 'pilot-channel', textContent: p.AssignedChannel || '\u2014' });
      var freqText = p.AssignedFreqMHz ? (p.AssignedFreqMHz + ' MHz') : '';
      var freqEl = el('div', { className: 'pilot-freq', textContent: freqText });
      var freqBlock = el('div', { className: 'pilot-freq-block' }, [chEl, freqEl]);
      card.appendChild(freqBlock);

      // Info block
      var info = document.createElement('div');
      info.className = 'pilot-info';

      var nameRow = document.createElement('div');
      var nameEl = el('span', { className: 'pilot-callsign', textContent: p.Callsign });
      nameRow.appendChild(nameEl);

      if (isMe) {
        var youBadge = el('span', { className: 'pilot-you-badge', textContent: 'YOU' });
        nameRow.appendChild(youBadge);
      }

      info.appendChild(nameRow);

      // System badge
      var sysLabel = SYSTEM_LABELS[p.VideoSystem] || p.VideoSystem.toUpperCase();
      var badge = el('span', { className: 'pilot-system-badge', textContent: sysLabel });
      info.appendChild(badge);

      // Buddy info
      if (buddyIdx > 0 && buddyGroups[p.BuddyGroup] && buddyGroups[p.BuddyGroup].length > 1) {
        var buddies = buddyGroups[p.BuddyGroup]
          .filter(function (b) { return b.ID !== p.ID; })
          .map(function (b) { return b.Callsign; });
        if (buddies.length > 0) {
          var buddyInfo = el('div', {
            className: 'pilot-buddy-info buddy-text-' + buddyIdx,
            textContent: 'SHARING WITH: ' + buddies.join(', ')
          });
          info.appendChild(buddyInfo);
        }
      }

      // Conflict warnings
      conflicts.forEach(function (c) {
        var level = c.level || c.Level;
        var otherName = c.other_callsign || c.OtherCallsign || '?';
        var sep = c.separation_mhz || c.SeparationMHz || 0;
        var req = c.required_mhz || c.RequiredMHz || 0;
        var conflictEl = el('div', {
          className: 'pilot-conflict conflict-' + level,
          textContent: (level === 'danger' ? 'OVERLAP' : 'CLOSE TO') + ' ' + otherName + ' (' + sep + '/' + req + ' MHz)'
        });
        info.appendChild(conflictEl);
      });

      card.appendChild(info);
      container.appendChild(card);
    });

    var count = pilots.length;
    $('pilot-count').textContent = count + ' PILOT' + (count !== 1 ? 'S' : '');
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
    $('input-callsign').value = '';

    showScreen('landing');
  }

  // ── Pilot Action Sheet ───────────────────────────────────────
  function showPilotActions() {
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
      showChannelChange();
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

  // ── Channel Change ──────────────────────────────────────────
  function showChannelChange() {
    var picker = $('channel-change-picker');
    clearChildren(picker);

    var pool = getChannelPool();
    pool.forEach(function (ch) {
      var nameSpan = el('span', { className: 'ch-name', textContent: ch.name });
      var freqSpan = el('span', { className: 'ch-freq', textContent: String(ch.freq) });
      var btn = el('button', { className: 'btn-channel' }, [nameSpan, freqSpan]);
      btn.addEventListener('click', function () {
        submitChannelChange(true, ch.freq);
      });
      picker.appendChild(btn);
    });

    $('channel-change').classList.remove('hidden');
  }

  function hideChannelChange() {
    $('channel-change').classList.add('hidden');
  }

  function initChannelChange() {
    $('btn-auto-reassign').addEventListener('click', function () {
      submitChannelChange(false, 0);
    });
    $('btn-change-video-system').addEventListener('click', async function () {
      hideChannelChange();
      // Remove from session, keep callsign + session code, go to wizard
      var savedCallsign = state.callsign;
      var savedCode = state.sessionCode;
      try {
        await apiDelete('/api/pilots/' + state.pilotId + '?session=' + state.sessionCode);
      } catch (err) {
        // Continue even if delete fails
      }
      stopPolling();
      clearState();
      state.sessionCode = savedCode;
      state.callsign = savedCallsign;
      state.videoSystem = '';
      $('input-callsign').value = savedCallsign;
      showScreen('setup');
      showStep('step-video');
    });
    $('btn-channel-change-cancel').addEventListener('click', hideChannelChange);
    $('channel-change').addEventListener('click', function (e) {
      if (e.target === $('channel-change')) hideChannelChange();
    });
  }

  async function submitChannelChange(locked, freqMHz) {
    hideChannelChange();
    var body = { channel_locked: locked, locked_frequency_mhz: freqMHz };
    try {
      // Preview first to check for displacements.
      var preview = await apiPost(
        '/api/pilots/' + state.pilotId + '/preview-channel?session=' + state.sessionCode,
        body
      );
      var displaced = preview.displaced || [];
      if (displaced.length > 0) {
        // Stash pending change so displacement confirm can commit it.
        state.pendingChannelChange = body;
        showDisplacementConfirm(displaced, preview.has_danger);
        return;
      }
      // No displacements — apply immediately.
      await commitChannelChange(body);
    } catch (err) {
      refreshSession();
    }
  }

  async function commitChannelChange(body, rebalance) {
    var rebalParam = (rebalance === false) ? '&rebalance=false' : '';
    try {
      await apiPut(
        '/api/pilots/' + state.pilotId + '/channel?session=' + state.sessionCode + rebalParam,
        body
      );
      state.channelLocked = body.channel_locked;
      state.lockedFreqMHz = body.locked_frequency_mhz;
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
    $('btn-callsign-cancel').addEventListener('click', hideCallsignChange);
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
      showError('callsign-change-error', 'ENTER A CALLSIGN');
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
        showError('callsign-change-error', 'CALLSIGN ALREADY IN USE');
      } else {
        showError('callsign-change-error', 'FAILED: ' + msg.toUpperCase());
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
      ctx.fillText('SHARE THIS CODE', 140, 180);
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
    var match = path.match(/^\/s\/([A-Fa-f0-9]{6})$/);

    if (match) {
      var code = match[1].toUpperCase();
      loadState();

      if (state.sessionCode === code && state.pilotId) {
        // Already joined this session, go to session view
        enterSessionView();
      } else {
        // Need to join this session
        state.sessionCode = code;
        saveState();
        showScreen('setup');
        showStep('step-callsign');
        $('input-callsign').focus();
      }
      return;
    }

    // Root path
    loadState();
    if (state.sessionCode && state.pilotId) {
      // Returning user with active session
      enterSessionView();
    } else {
      showScreen('landing');
    }
  }

  // ── Init ──────────────────────────────────────────────────────
  function init() {
    initLanding();
    initCallsignStep();
    initVideoStep();
    initFollowUpStep();
    initChannelStep();
    initSessionView();
    initPilotActions();
    initChannelChange();
    initCallsignChange();
    initOtherPilotActions();
    initChannelChangeBanner();
    initDisplacementConfirm();
    route();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
