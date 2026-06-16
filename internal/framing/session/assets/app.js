const els = {
  video: document.getElementById("sourceVideo"),
  canvas: document.getElementById("overlay"),
  meta: document.getElementById("sessionMeta"),
  presets: document.getElementById("presets"),
  speakerMap: document.getElementById("speakerMap"),
  policy: document.getElementById("policy"),
  status: document.getElementById("status"),
  selectedPreset: document.getElementById("selectedPreset"),
  coords: document.getElementById("coords"),
  lockPreset: document.getElementById("lockPreset"),
  presetList: document.getElementById("presetList"),
  commitBadge: document.getElementById("commitBadge"),
  frameBadge: document.getElementById("frameBadge"),
};

const minCrop = 16;
const handleSize = 18;
let state = null;
let selected = 0;
let drag = null;

function show(value) {
  els.status.textContent = typeof value === "string" ? value : JSON.stringify(value, null, 2);
}

async function api(path, options = {}) {
  const res = await fetch(path, {
    headers: {"content-type": "application/json"},
    ...options,
  });
  const data = await res.json();
  if (!res.ok) throw data;
  return data;
}

function presetsDoc() {
  return JSON.parse(els.presets.value);
}

function setPresetsDoc(doc) {
  state.presets = doc;
  els.presets.value = JSON.stringify(doc, null, 2);
  updateInspector();
  renderPresetList();
  draw();
}

function selectedPreset() {
  return state?.presets?.presets?.[selected] || null;
}

function scale() {
  const doc = state.presets;
  return {
    sx: els.canvas.width / doc.source_width,
    sy: els.canvas.height / doc.source_height,
  };
}

function toCanvasRect(rect) {
  const s = scale();
  return {
    x: rect.x * s.sx,
    y: rect.y * s.sy,
    w: rect.w * s.sx,
    h: rect.h * s.sy,
  };
}

function canvasPoint(event) {
  const bounds = els.canvas.getBoundingClientRect();
  const x = (event.clientX - bounds.left) * (els.canvas.width / bounds.width);
  const y = (event.clientY - bounds.top) * (els.canvas.height / bounds.height);
  const s = scale();
  return {
    canvasX: x,
    canvasY: y,
    sourceX: Math.round(x / s.sx),
    sourceY: Math.round(y / s.sy),
  };
}

function clampRect(rect, doc) {
  rect.w = Math.max(minCrop, Math.min(rect.w, doc.source_width));
  rect.h = Math.max(minCrop, Math.min(rect.h, doc.source_height));
  rect.x = Math.max(0, Math.min(rect.x, doc.source_width - rect.w));
  rect.y = Math.max(0, Math.min(rect.y, doc.source_height - rect.h));
  rect.x = Math.round(rect.x);
  rect.y = Math.round(rect.y);
  rect.w = Math.round(rect.w);
  rect.h = Math.round(rect.h);
  return rect;
}

function handlesFor(rect) {
  const r = toCanvasRect(rect);
  const half = handleSize / 2;
  return [
    {name: "nw", x: r.x - half, y: r.y - half},
    {name: "ne", x: r.x + r.w - half, y: r.y - half},
    {name: "sw", x: r.x - half, y: r.y + r.h - half},
    {name: "se", x: r.x + r.w - half, y: r.y + r.h - half},
  ];
}

function hitTest(point) {
  const presets = state?.presets?.presets || [];
  const selectedHit = hitPreset(point, selected);
  if (selectedHit) return selectedHit;
  const order = presets
    .map((preset, index) => ({index, area: preset.crop_px.w * preset.crop_px.h}))
    .sort((a, b) => a.area - b.area)
    .map(item => item.index);
  for (const i of order) {
    if (i === selected) continue;
    const hit = hitPreset(point, i);
    if (hit) return hit;
  }
  return null;
}

function hitPreset(point, index) {
  const preset = state?.presets?.presets?.[index];
  if (!preset) return null;
  if (isBackgroundWide(preset) && state.presets.presets.length > 1) return null;
  const rect = toCanvasRect(preset.crop_px);
  const margin = handleSize;
  for (const handle of handlesFor(preset.crop_px)) {
    if (point.canvasX >= handle.x - margin && point.canvasX <= handle.x + handleSize + margin &&
        point.canvasY >= handle.y - margin && point.canvasY <= handle.y + handleSize + margin) {
      return {index, mode: "resize", handle: handle.name};
    }
  }
  if (index === selected &&
      point.canvasX >= rect.x - margin && point.canvasX <= rect.x + rect.w + margin &&
      point.canvasY >= rect.y - margin && point.canvasY <= rect.y + rect.h + margin) {
    const east = Math.abs(point.canvasX - (rect.x + rect.w));
    const west = Math.abs(point.canvasX - rect.x);
    const south = Math.abs(point.canvasY - (rect.y + rect.h));
    const north = Math.abs(point.canvasY - rect.y);
    if (Math.min(east, west, south, north) <= margin) {
      return {
        index,
        mode: "resize",
        handle: `${north < south ? "n" : "s"}${west < east ? "w" : "e"}`,
      };
    }
  }
  if (point.canvasX >= rect.x && point.canvasX <= rect.x + rect.w &&
      point.canvasY >= rect.y && point.canvasY <= rect.y + rect.h) {
    return {index, mode: "move"};
  }
  return null;
}

function isBackgroundWide(preset) {
  return preset.type === "wide" || preset.id === "wide";
}

function applyDrag(point) {
  if (!drag) return;
  const doc = presetsDoc();
  const preset = doc.presets[drag.index];
  if (!preset || preset.locked) return;
  const dx = point.sourceX - drag.start.sourceX;
  const dy = point.sourceY - drag.start.sourceY;
  const rect = {...drag.startRect};
  if (drag.mode === "move") {
    rect.x += dx;
    rect.y += dy;
  } else if (drag.mode === "draw") {
    rect.x = Math.min(drag.start.sourceX, point.sourceX);
    rect.y = Math.min(drag.start.sourceY, point.sourceY);
    rect.w = Math.abs(point.sourceX - drag.start.sourceX);
    rect.h = Math.abs(point.sourceY - drag.start.sourceY);
  } else {
    if (drag.handle.includes("n")) {
      rect.y += dy;
      rect.h -= dy;
    }
    if (drag.handle.includes("s")) {
      rect.h += dy;
    }
    if (drag.handle.includes("w")) {
      rect.x += dx;
      rect.w -= dx;
    }
    if (drag.handle.includes("e")) {
      rect.w += dx;
    }
  }
  preset.crop_px = clampRect(rect, doc);
  setPresetsDoc(doc);
}

function updateInspector() {
  const preset = selectedPreset();
  if (!preset) {
    els.selectedPreset.textContent = "No preset selected";
    els.coords.textContent = "";
    return;
  }
  const doc = state.presets;
  const r = preset.crop_px;
  const norm = {
    x: +(r.x / doc.source_width).toFixed(5),
    y: +(r.y / doc.source_height).toFixed(5),
    w: +(r.w / doc.source_width).toFixed(5),
    h: +(r.h / doc.source_height).toFixed(5),
  };
  const zoom = +(doc.source_width / r.w).toFixed(2);
  els.selectedPreset.textContent = `${preset.id}${preset.locked ? " (locked)" : ""}`;
  els.coords.textContent = `px ${r.x},${r.y} ${r.w}x${r.h} · norm ${norm.x},${norm.y} ${norm.w}x${norm.h} · zoom ${zoom}x · aspect ${doc.target_aspect}`;
  els.lockPreset.textContent = preset.locked ? "Unlock" : "Lock";
  if (els.frameBadge) {
    els.frameBadge.textContent = `${r.w}x${r.h} @ ${r.x},${r.y}`;
  }
}

function renderPresetList() {
  if (!els.presetList || !state?.presets?.presets) return;
  els.presetList.replaceChildren();
  state.presets.presets.forEach((preset, index) => {
    const item = document.createElement("li");
    item.className = "preset-item";
    item.setAttribute("role", "option");
    item.setAttribute("aria-selected", index === selected ? "true" : "false");
    item.tabIndex = 0;

    const swatch = document.createElement("span");
    swatch.className = "preset-color";
    swatch.style.background = index === selected ? "var(--accent)" : "var(--warn)";

    const id = document.createElement("span");
    id.className = "preset-id";
    id.textContent = preset.id;

    const meta = document.createElement("span");
    meta.className = "preset-meta";
    meta.textContent = `${preset.crop_px.w}x${preset.crop_px.h}`;

    item.append(swatch, id, meta);
    if (preset.locked) {
      const lock = document.createElement("span");
      lock.className = "preset-lock";
      lock.textContent = "locked";
      item.append(lock);
    }
    const selectItem = () => {
      selected = index;
      updateInspector();
      renderPresetList();
      draw();
    };
    item.addEventListener("click", selectItem);
    item.addEventListener("keydown", event => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        selectItem();
      }
    });
    els.presetList.append(item);
  });
}

function draw() {
  const ctx = els.canvas.getContext("2d");
  ctx.clearRect(0, 0, els.canvas.width, els.canvas.height);
  if (!state?.presets?.presets) return;
  state.presets.presets.forEach((preset, i) => {
    const r = toCanvasRect(preset.crop_px);
    const active = i === selected;
    ctx.strokeStyle = active ? "#7ee787" : "#f5c542";
    ctx.lineWidth = active ? 3 : 2;
    ctx.strokeRect(r.x, r.y, r.w, r.h);
    ctx.fillStyle = active ? "rgba(126,231,135,.12)" : "rgba(245,197,66,.08)";
    ctx.fillRect(r.x, r.y, r.w, r.h);
    ctx.fillStyle = "rgba(0,0,0,.68)";
    ctx.fillRect(r.x, r.y, 180, 28);
    ctx.fillStyle = "#fff";
    ctx.fillText(`${preset.id}${preset.locked ? " locked" : ""}`, r.x + 8, r.y + 18);
    if (active && !preset.locked) {
      ctx.fillStyle = "#7ee787";
      for (const handle of handlesFor(preset.crop_px)) {
        ctx.fillRect(handle.x, handle.y, handleSize, handleSize);
      }
    }
  });
}

async function load() {
  state = await api("/api/state");
  els.meta.textContent = `${state.session_id} · ${state.project_id || "project"} · commit ${state.commit_enabled}`;
  if (els.commitBadge) {
    els.commitBadge.textContent = state.commit_enabled ? "commit enabled" : "dry run";
    els.commitBadge.classList.toggle("commit-enabled", Boolean(state.commit_enabled));
  }
  els.video.src = state.media_url || "";
  els.presets.value = JSON.stringify(state.presets, null, 2);
  els.speakerMap.value = JSON.stringify(state.speaker_map, null, 2);
  els.policy.value = JSON.stringify(state.policy, null, 2);
  show({safe_zone_warnings: state.safe_zone_warnings, artifacts: state.artifacts});
  updateInspector();
  renderPresetList();
  draw();
}

document.getElementById("addPreset").addEventListener("click", () => {
  const doc = presetsDoc();
  const n = doc.presets.length + 1;
  doc.presets.push({
    id: `preset_${n}`,
    label: `Preset ${n}`,
    type: "speaker",
    locked: false,
    crop_px: {x: 0, y: 0, w: Math.floor(doc.source_width / 2), h: Math.floor(doc.source_height / 2)}
  });
  selected = doc.presets.length - 1;
  setPresetsDoc(doc);
});

els.lockPreset.addEventListener("click", () => {
  const doc = presetsDoc();
  if (!doc.presets[selected]) return;
  doc.presets[selected].locked = !doc.presets[selected].locked;
  setPresetsDoc(doc);
});

els.canvas.addEventListener("pointerdown", (event) => {
  if (!state?.presets || !els.video.paused) return;
  const point = canvasPoint(event);
  const hit = hitTest(point);
  if (!hit) {
    const preset = selectedPreset();
    if (!preset || preset.locked || isBackgroundWide(preset)) return;
    els.canvas.setPointerCapture(event.pointerId);
    drag = {
      index: selected,
      mode: "draw",
      start: point,
      startRect: {...preset.crop_px},
    };
    return;
  }
  selected = hit.index;
  const preset = selectedPreset();
  if (!preset || preset.locked) {
    updateInspector();
    draw();
    return;
  }
  els.canvas.setPointerCapture(event.pointerId);
  drag = {
    ...hit,
    start: point,
    startRect: {...preset.crop_px},
  };
  updateInspector();
  draw();
});

els.canvas.addEventListener("pointermove", (event) => {
  if (!state?.presets) return;
  const point = canvasPoint(event);
  if (drag) {
    applyDrag(point);
    return;
  }
  const hit = hitTest(point);
  if (!hit) {
    els.canvas.style.cursor = "crosshair";
  } else if (state.presets.presets[hit.index]?.locked) {
    els.canvas.style.cursor = "not-allowed";
  } else if (hit.mode === "move") {
    els.canvas.style.cursor = "move";
  } else if (hit.handle === "nw" || hit.handle === "se") {
    els.canvas.style.cursor = "nwse-resize";
  } else {
    els.canvas.style.cursor = "nesw-resize";
  }
});

function endDrag(event) {
  if (!drag) return;
  try {
    els.canvas.releasePointerCapture(event.pointerId);
  } catch (_) {}
  drag = null;
  show({status: "edited", preset: selectedPreset()});
}

els.canvas.addEventListener("pointerup", endDrag);
els.canvas.addEventListener("pointercancel", endDrag);

document.getElementById("togglePlay").addEventListener("click", async () => {
  try {
    if (els.video.paused) {
      els.video.muted = true;
      await els.video.play();
      els.canvas.classList.add("playing");
    } else {
      els.video.pause();
      els.canvas.classList.remove("playing");
    }
    show({status: els.video.paused ? "paused" : "playing", current_time: els.video.currentTime, duration: els.video.duration});
  } catch (error) {
    show({status: "playback_error", message: String(error && error.message ? error.message : error)});
  }
});

els.video.addEventListener("pause", () => els.canvas.classList.remove("playing"));
els.video.addEventListener("play", () => els.canvas.classList.add("playing"));

document.getElementById("savePresets").addEventListener("click", async () => {
  const data = await api("/api/presets", {method: "POST", body: els.presets.value});
  state = data.state;
  selected = Math.min(selected, state.presets.presets.length - 1);
  els.presets.value = JSON.stringify(state.presets, null, 2);
  show(data);
  updateInspector();
  renderPresetList();
  draw();
});

document.getElementById("commitArtifacts").addEventListener("click", async () => {
  show(await api("/api/commit?commit=true", {method: "POST"}));
});

document.getElementById("shutdown").addEventListener("click", async () => {
  show(await api("/api/shutdown", {method: "POST"}));
});

els.presets.addEventListener("change", () => {
  try {
    const doc = presetsDoc();
    state.presets = doc;
    selected = Math.min(selected, doc.presets.length - 1);
    updateInspector();
    renderPresetList();
    draw();
  } catch (error) {
    show({status: "preset_json_error", message: String(error && error.message ? error.message : error)});
  }
});
els.speakerMap.addEventListener("change", async () => show(await api("/api/speaker-map", {method: "POST", body: els.speakerMap.value})));
els.policy.addEventListener("change", async () => show(await api("/api/policy", {method: "POST", body: els.policy.value})));
load().catch(show);
