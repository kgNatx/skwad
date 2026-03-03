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

  async function handleJoinSession() {
    if (state.channelLocked && !state.lockedFreqMHz) {
      showError('join-error', 'SELECT A CHANNEL');
      return;
    }
    hideError('join-error');

    var btn = $('btn-join-session');
    setLoading(btn, true);

    var effectiveSystem = getEffectiveVideoSystem();
    var body = {
      callsign: state.callsign,
      video_system: effectiveSystem,
      fcc_unlocked: state.fccUnlocked,
      goggles: state.goggles,
      bandwidth_mhz: state.bandwidthMHz,
      race_mode: state.raceMode,
      channel_locked: state.channelLocked,
      locked_frequency_mhz: state.lockedFreqMHz,
    };

    try {
      var pilot = await apiPost('/api/sessions/' + state.sessionCode + '/join', body);
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

  function renderPilotList(pilots) {
    var container = $('pilot-list');
    clearChildren(container);

    if (!pilots || pilots.length === 0) {
      var emptyDiv = el('div', { className: 'empty-state' }, [
        el('div', { className: 'empty-state-text', textContent: 'WAITING FOR PILOTS...' })
      ]);
      container.appendChild(emptyDiv);
      $('pilot-count').textContent = '0 PILOTS ACTIVE';
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

      if (buddyIdx > 0) {
        card.classList.add('buddy-group', 'buddy-' + buddyIdx);
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

      card.appendChild(info);
      container.appendChild(card);
    });

    var count = pilots.length;
    $('pilot-count').textContent = count + ' PILOT' + (count !== 1 ? 'S' : '') + ' ACTIVE';
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
    $('btn-leave').addEventListener('click', handleLeave);
  }

  async function handleLeave() {
    if (!state.pilotId || !state.sessionCode) return;

    var btn = $('btn-leave');
    setLoading(btn, true);

    try {
      await apiDelete('/api/pilots/' + state.pilotId + '?session=' + state.sessionCode);
    } catch (err) {
      // Even if delete fails, leave the session locally
    }

    stopPolling();
    clearState();
    setLoading(btn, false);

    // Reset setup state
    state.callsign = '';
    state.videoSystem = '';
    $('input-callsign').value = '';

    showScreen('landing');
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
    route();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
