# DJI Power Ceiling Guidance Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give DJI pilots actionable guidance when a session has a power ceiling, since they can't manually control TX power.

**Architecture:** Frontend-only. Add a DJI bandwidth tip to the existing power ceiling joiner interstitial. Add recommended/warning styling to bandwidth buttons in both the join wizard and leader add-pilot dialog. Add a DJI Dynamic Power Control section to the frequency reference doc. No backend or optimizer changes.

**Tech Stack:** Vanilla JS, CSS, HTML, Markdown

**Spec:** `docs/superpowers/specs/2026-03-14-dji-power-guidance-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `static/index.html` | Modify | Add DJI guidance paragraph to `step-power-alert` (line 99) |
| `static/app.js` | Modify | Conditionally show/hide DJI guidance; apply bandwidth button styling in `showBandwidthOptions()` (line 870) and `showAddPilotOptions()` (line 2564) |
| `static/style.css` | Modify | Styles for DJI guidance text, recommended badge, amber bandwidth warning |
| `frequency-reference.md` | Modify | New `## DJI Dynamic Power Control` section after line 321 (`---` separator) |
| `static/sw.js` | Modify | Bump `CACHE_NAME` from `skwad-v17` to `skwad-v18` (line 5) |
| `CHANGELOG.md` | Modify | Developer changelog entry |
| `static/changelog.html` | Modify | User-facing changelog entry |

---

## Chunk 1: Interstitial DJI Guidance + CSS

### Task 1: Add DJI guidance HTML to power ceiling interstitial

**Files:**
- Modify: `static/index.html:94-102`

- [ ] **Step 1: Add DJI guidance paragraph**

In `static/index.html`, after line 99 (the `<p class="power-alert-text">` paragraph), add a new paragraph. The full `step-power-alert` block should become:

```html
    <!-- Step 1.25: Power Ceiling Alert (joining a session with a ceiling) -->
    <div id="step-power-alert" class="setup-step hidden">
      <h2 class="step-title power-alert-title">POWER CEILING</h2>
      <div class="power-alert-body">
        <div class="power-alert-value"><span id="power-alert-mw">200</span> mW</div>
        <p class="power-alert-text">The session leader has set a transmit power ceiling. Please set your VTX to <strong id="power-alert-mw-bold">200</strong> mW or below for the best experience.</p>
        <p id="power-alert-dji-hint" class="power-alert-dji-hint hidden">DJI pilots: use 20 MHz bandwidth for best channel compatibility.</p>
      </div>
      <button id="btn-power-alert-ok" class="btn btn-primary btn-large">GOT IT</button>
    </div>
```

The new `<p>` has `id="power-alert-dji-hint"` and starts `hidden`. JS will show it conditionally.

- [ ] **Step 2: Commit**

```bash
git add static/index.html
git commit -m "feat: add DJI bandwidth guidance element to power ceiling interstitial"
```

### Task 2: Add CSS for DJI guidance and bandwidth button styling

**Files:**
- Modify: `static/style.css` (after line 1453, the `.power-alert-text strong` rule)

- [ ] **Step 1: Add styles**

After the existing `.power-alert-text strong { color: #fff; }` rule (line 1453), add:

```css
.power-alert-dji-hint {
  font-size: 13px;
  font-weight: 600;
  color: #60a5fa;
  margin-top: 12px;
  padding: 8px 12px;
  background: rgba(96,165,250,0.08);
  border: 1px solid rgba(96,165,250,0.25);
  border-radius: 8px;
  max-width: 300px;
  margin-left: auto;
  margin-right: auto;
  line-height: 1.4;
}

/* Bandwidth button recommendation/warning */
.bw-recommended {
  border-color: #4ade80 !important;
  box-shadow: 0 0 0 1px rgba(74,222,128,0.3);
  position: relative;
}
.bw-recommended::after {
  content: 'RECOMMENDED';
  display: block;
  font-size: 9px;
  font-weight: 800;
  color: #4ade80;
  letter-spacing: 0.5px;
  margin-top: 2px;
}
.bw-warn {
  border-color: #f59e0b !important;
  box-shadow: 0 0 0 1px rgba(245,158,11,0.3);
  color: #f59e0b !important;
}
```

- [ ] **Step 2: Commit**

```bash
git add static/style.css
git commit -m "feat: add CSS for DJI power guidance and bandwidth button styling"
```

### Task 3: Wire up DJI guidance visibility in JS

**Files:**
- Modify: `static/app.js:593-596`

- [ ] **Step 1: Show/hide the DJI hint when the power alert step is displayed**

In `static/app.js`, find the block at lines 593-596 where the power alert is shown:

```javascript
      } else if (state.sessionPowerCeiling > 0) {
        $('power-alert-mw').textContent = state.sessionPowerCeiling;
        $('power-alert-mw-bold').textContent = state.sessionPowerCeiling;
        showStep('step-power-alert');
```

Replace with:

```javascript
      } else if (state.sessionPowerCeiling > 0) {
        $('power-alert-mw').textContent = state.sessionPowerCeiling;
        $('power-alert-mw-bold').textContent = state.sessionPowerCeiling;
        var djiHint = $('power-alert-dji-hint');
        if (state.sessionPowerCeiling < 600) {
          djiHint.classList.remove('hidden');
        } else {
          djiHint.classList.add('hidden');
        }
        showStep('step-power-alert');
```

This shows the DJI hint when the ceiling is set and below 600 mW, hides it at 600+ mW.

- [ ] **Step 2: Commit**

```bash
git add static/app.js
git commit -m "feat: conditionally show DJI bandwidth hint on power ceiling interstitial"
```

---

## Chunk 2: Bandwidth Button Highlighting

### Task 4: Add bandwidth styling to join wizard

**Files:**
- Modify: `static/app.js:870-885` (the `showBandwidthOptions` function)

- [ ] **Step 1: Add a helper function for bandwidth button styling**

In `static/app.js`, just before the `showBandwidthOptions` function (line 870), add:

```javascript
  function shouldWarnBandwidth() {
    return state.sessionPowerCeiling > 0 && state.sessionPowerCeiling < 600;
  }

  function applyBandwidthHint(btn, bw) {
    if (!shouldWarnBandwidth()) return;
    if (bw <= 20) {
      btn.classList.add('bw-recommended');
    } else {
      btn.classList.add('bw-warn');
    }
  }
```

- [ ] **Step 2: Apply styling in `showBandwidthOptions()`**

In the `showBandwidthOptions` function (line 870), after the button is created and before it's appended to the container, add the hint call. The function should become:

```javascript
  function showBandwidthOptions(options) {
    $('followup-bandwidth').classList.remove('hidden');
    var container = $('bandwidth-buttons');
    clearChildren(container);
    options.forEach(function (bw) {
      var btn = document.createElement('button');
      btn.className = 'btn btn-option';
      btn.textContent = bw + ' MHz';
      applyBandwidthHint(btn, bw);
      btn.addEventListener('click', function () {
        container.querySelectorAll('.btn-option').forEach(function (b) { b.classList.remove('selected'); });
        btn.classList.add('selected');
        handleBandwidthSelected(bw);
      });
      container.appendChild(btn);
    });
  }
```

The only new line is `applyBandwidthHint(btn, bw);` after setting `textContent`.

- [ ] **Step 3: Commit**

```bash
git add static/app.js
git commit -m "feat: highlight bandwidth buttons in join wizard when power ceiling is set"
```

### Task 5: Add bandwidth styling to leader add-pilot dialog

**Files:**
- Modify: `static/app.js:2564-2580` (bandwidth buttons in `showAddPilotOptions`)

- [ ] **Step 1: Apply styling in `showAddPilotOptions()`**

In the `showAddPilotOptions` function, find the bandwidth button creation loop (lines 2569-2579). Add `applyBandwidthHint(btn, bw);` after the button is created. The loop should become:

```javascript
      bwOptions.forEach(function (bw) {
        var btn = el('button', {
          className: 'btn btn-toggle' + (bw === 20 ? ' active' : ''),
          textContent: bw + ' MHz'
        });
        applyBandwidthHint(btn, bw);
        btn.addEventListener('click', function () {
          addPilotState.bandwidthMHz = bw;
          bwContainer.querySelectorAll('.btn-toggle').forEach(function (b) { b.classList.remove('active'); });
          btn.classList.add('active');
        });
        bwContainer.appendChild(btn);
      });
```

The only new line is `applyBandwidthHint(btn, bw);` after `el()`.

- [ ] **Step 2: Commit**

```bash
git add static/app.js
git commit -m "feat: highlight bandwidth buttons in add-pilot dialog when power ceiling is set"
```

---

## Chunk 3: Documentation and Release

### Task 6: Add DJI Dynamic Power Control section to frequency reference

**Files:**
- Modify: `frequency-reference.md` (after the `---` separator on line 321)

- [ ] **Step 1: Add new section**

After the `---` separator on line 321 (which follows the `### Impact Summary` subsection) and before `## Intermodulation Distortion (IMD)` on line 323, insert a new h2 peer section:

```markdown

## DJI Dynamic Power Control

DJI O3 and O4 video systems use automatic power control — the VTX adjusts transmit power dynamically based on link quality. Pilots have no manual mW setting. This has several implications for session power ceilings:

**How DJI dynamic power behaves:**

- Power scales with link quality, which depends on distance, obstacles, and antenna orientation — not a single variable
- At typical meetup distances (~100m line-of-sight), DJI tends to stay at low power (community estimates suggest under 200 mW)
- Power ramps up progressively at range, approaching the maximum (~1W / 30 dBm in FCC mode) beyond ~500m or through obstacles
- These are community observations from spectrum analyzer measurements, not manufacturer specifications — DJI does not publish their power control curves
- The exact power at a given distance varies significantly with environment, antenna choice, and interference

**Why bandwidth matters more than power for DJI:**

Bandwidth is the lever DJI pilots actually control. Switching from 40 MHz to 20 MHz mode saves 10 MHz of required spacing per neighbor — a bigger impact than most power ceiling steps. The math:

| DJI Bandwidth | vs 20 MHz neighbor (14 MHz guard*) | vs 40 MHz neighbor (14 MHz guard*) |
|--------------|-----------------------------------|-----------------------------------|
| 20 MHz | 34 MHz spacing | 44 MHz spacing |
| 40 MHz | 44 MHz spacing | 54 MHz spacing |
| 60 MHz | 54 MHz spacing | 64 MHz spacing |

*14 MHz guard band = 200 mW power ceiling step. Each 20 MHz step in DJI bandwidth adds 10 MHz to required spacing — equivalent to going from a 200 mW guard band to a 600 mW guard band.

**Practical experience:**

Groups of 6-8 mixed DJI/analog pilots fit comfortably on raceband channels when DJI pilots use 20 MHz bandwidth and fly within a confined area. The dynamic power at these distances stays within a range where the default guard band provides adequate separation.

**Skwad's approach:**

Skwad treats DJI pilots identically to analog in the optimizer — same guard band for the same power ceiling. The app provides guidance rather than enforcement:
- The power ceiling joiner interstitial includes a DJI-specific bandwidth recommendation (when ceiling < 600 mW)
- Bandwidth buttons show visual recommended/warning indicators based on the session's power ceiling
- The optimizer already accounts for DJI's wider bandwidth through the `RequiredSpacing()` formula

This matches Skwad's philosophy: it's a coordinator, not a controller.

---
```

Note: The trailing `---` separator keeps the document structure consistent — each h2 section is delimited by horizontal rules.

- [ ] **Step 2: Commit**

```bash
git add frequency-reference.md
git commit -m "docs: add DJI dynamic power control section to frequency reference"
```

### Task 7: Bump service worker and update changelogs

**Files:**
- Modify: `static/sw.js:5`
- Modify: `CHANGELOG.md`
- Modify: `static/changelog.html`

- [ ] **Step 1: Bump service worker cache**

In `static/sw.js`, change line 5:

```javascript
const CACHE_NAME = 'skwad-v18';
```

- [ ] **Step 2: Update CHANGELOG.md**

Add at the top of the version list:

```markdown
## [0.4.1] - 2026-03-14

### Added
- **DJI bandwidth guidance.** Power ceiling interstitial now includes a tip for DJI pilots to use 20 MHz bandwidth for best channel compatibility (shown when ceiling < 600 mW).
- **Bandwidth button indicators.** DJI O3/O4 bandwidth buttons show "RECOMMENDED" on 10/20 MHz and amber warning on 40/60 MHz when a power ceiling is set below 600 mW. Applies in join wizard, video system change, and leader add-pilot dialog.
- **DJI Dynamic Power Control section** in `frequency-reference.md` documenting DJI's automatic power behavior, bandwidth impact on spacing, and practical group flying experience.
```

- [ ] **Step 3: Update static/changelog.html**

Add new version block at the top of the version list:

```html
<div class="version">
  <div class="version-header">v0.4.1</div>
  <div class="version-date">March 14, 2026</div>

  <h3>Improvements</h3>
  <ul>
    <li><strong>Better guidance for DJI pilots.</strong> When a session has a power ceiling, the join screen now reminds DJI pilots to use 20 MHz bandwidth for best channel compatibility. Bandwidth buttons also show visual indicators — green for recommended, amber for wider modes that reduce channel density.</li>
  </ul>
</div>

<hr class="divider">
```

- [ ] **Step 4: Commit**

```bash
git add static/sw.js CHANGELOG.md static/changelog.html
git commit -m "release: v0.4.1 — DJI power ceiling guidance"
```

---

## Execution Notes

**Build and test after implementation:**

```bash
cd /home/kyleg/containers/atxfpv.org && docker compose build skwad && docker compose up -d skwad
```

**Manual testing checklist:**
1. Create session with 200 mW ceiling → join from another browser → verify DJI hint shows on interstitial
2. Create session with 800 mW ceiling → join → verify DJI hint is hidden
3. Create session with no ceiling → join → verify no interstitial at all
4. Join as DJI O3 with 200 mW ceiling → verify 10/20 MHz buttons show "RECOMMENDED", 40 MHz shows amber
5. Join as DJI O4 with 200 mW ceiling → verify 10/20 MHz "RECOMMENDED", 40/60 MHz amber
6. Join as DJI O4 with 800 mW ceiling → verify no button highlighting
7. Leader adds DJI O3 pilot in 200 mW session → verify bandwidth buttons show same styling
8. Mid-session video system change to DJI O3 in 200 mW session → verify bandwidth buttons show styling
9. Self-service: tap own pilot card → channel change options → CHANGE VIDEO SYSTEM → pick DJI O3 in 200 mW session → verify bandwidth buttons show recommended/warning styling

**Push to both remotes after final commit:**

```bash
cd /home/kyleg/containers/atxfpv.org
git push origin main
git subtree push --prefix=skwad skwad main
```
