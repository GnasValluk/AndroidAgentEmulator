"use strict";
const $ = (s, r) => (r || document).querySelector(s);
const state = { token: localStorage.getItem("emu_token") || "", timers: {} };

async function api(path, opts) {
  opts = opts || {};
  const headers = Object.assign({}, opts.headers || {});
  if (state.token) headers["Authorization"] = "Bearer " + state.token;
  const res = await fetch(path, Object.assign({}, opts, { headers: headers }));
  const ct = res.headers.get("content-type") || "";
  const body = ct.indexOf("application/json") >= 0 ? await res.json() : await res.text();
  if (!res.ok) throw new Error((body && body.error) || body || ("HTTP " + res.status));
  return body;
}

function toast(msg) {
  const d = document.createElement("div");
  d.className = "toast-msg"; d.textContent = msg;
  $("#toast").appendChild(d);
  setTimeout(() => d.remove(), 3500);
}

function kv(el, obj) {
  el.innerHTML = Object.keys(obj).map(k =>
    `<div><span>${k}</span><span>${obj[k]}</span></div>`).join("");
}

async function loadServer() {
  try {
    const v = await api("/v1/version");
    kv($("#server-info"), {
      android: v.version + " (API " + v.api_level + ")",
      branch: v.aosp_branch, source: v.source,
    });
    $("#cli-ver").textContent = "android " + v.version;
  } catch (e) { $("#server-info").textContent = "unreachable: " + e.message; }
  try {
    const list = await api("/v1/integrity");
    $("#integrity").innerHTML = list.map(c =>
      `<div><span>${c.name}</span><span class="verdict v-${c.verdict}">${c.verdict}</span></div>`).join("");
  } catch (e) { $("#integrity").textContent = "error: " + e.message; }
}

function cardHtml(d) {
  const snaps = (d.snapshots || []).map(s =>
    `<div class="snap-row"><code>${s}</code><button data-act="boot-snap" data-snap="${s}">Boot</button></div>`).join("")
    || '<div class="tag">no snapshots</div>';
  return `
  <div class="card" id="dev-${d.name}">
    <div class="dev-head"><span class="dot ${d.running ? "on" : ""}"></span><h3>${d.name}</h3>
      <span class="tag">${d.running ? d.serial + " · adb:" + d.adb_port : "stopped"}</span></div>
    <div class="specs"><span class="spec">${d.target || "?"}</span><span class="spec">${d.ram_mb} MB</span>
      <span class="spec">${d.cpus} cpu</span><span class="spec">${d.width}x${d.height} @${d.density}</span></div>
    <div class="actions">
      ${d.running
        ? `<button data-act="stop" class="danger">Stop</button>
           <button data-act="shot">Refresh preview</button>`
        : `<select data-role="gpu"><option>host</option><option>swiftshader_indirect</option><option>auto</option></select>
           <select data-role="snap"><option value="">(default boot)</option>${(d.snapshots || []).map(s => `<option>${s}</option>`).join("")}</select>
           <label class="tag"><input type="checkbox" data-role="nosave" checked> no-save</label>
           <label class="tag"><input type="checkbox" data-role="showwin" checked> window</label>
           <button data-act="start">Start</button>`}
    </div>
    ${d.running ? `<img class="preview" data-role="preview" alt="live preview">` : ""}
    <details><summary>Snapshots</summary>${snaps}
      <div class="snap-row"><input data-role="newsnap" placeholder="new snapshot name"><button data-act="save-snap">Save</button></div>
    </details>
    <details><summary>Settings (stopped only)</summary>
      <div class="form-grid">
        <label>RAM MB<input data-k="ram_mb" type="number" value="${d.ram_mb}"></label>
        <label>CPUs<input data-k="cpus" type="number" value="${d.cpus}"></label>
        <label>Width<input data-k="width" type="number" value="${d.width}"></label>
        <label>Height<input data-k="height" type="number" value="${d.height}"></label>
        <label>Density<input data-k="density" type="number" value="${d.density}"></label>
      </div>
      <button data-act="apply-cfg" ${d.running ? "disabled" : ""}>Apply</button>
    </details>
    <details><summary>Install APK</summary>
      <div class="snap-row"><input data-role="apk" type="file" accept=".apk"><button data-act="install">Install</button></div>
    </details>
    <details><summary>Boot log</summary><button data-act="log">Load log</button><div class="log" data-role="log"></div></details>
  </div>`;
}

async function previewImg(img, port) {
  try {
    const res = await fetch("/v1/screen.png?port=" + port,
      { headers: state.token ? { "Authorization": "Bearer " + state.token } : {} });
    if (!res.ok) return;
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    if (img.dataset.url) URL.revokeObjectURL(img.dataset.url);
    img.dataset.url = url; img.src = url;
  } catch (e) { /* offline, retry next tick */ }
}

function armPreviews(devices) {
  Object.keys(state.timers).forEach(k => { clearInterval(state.timers[k]); delete state.timers[k]; });
  devices.forEach(d => {
    if (!d.running) return;
    const img = $(`#dev-${d.name} [data-role=preview]`);
    if (!img) return;
    previewImg(img, d.adb_port);
    state.timers[d.name] = setInterval(() => {
      if (!document.body.contains(img)) { clearInterval(state.timers[d.name]); return; }
      previewImg(img, d.adb_port);
    }, 3000);
  });
}

async function loadDevices() {
  const box = $("#devices");
  try {
    const list = await api("/v1/avds");
    $("#dev-count").textContent = list.length + " avd";
    box.innerHTML = list.map(cardHtml).join("") || '<div class="tag">no AVDs found (check ANDROID_AVD_HOME)</div>';
    armPreviews(list);
  } catch (e) { box.innerHTML = '<div class="tag">error: ' + e.message + "</div>"; }
}

async function onClick(e) {
  const btn = e.target.closest("button[data-act]");
  if (!btn) return;
  const card = btn.closest(".card");
  const name = card.id.replace("dev-", "");
  const act = btn.dataset.act;
  btn.disabled = true;
  try {
    if (act === "start") {
      const body = {
        gpu: $('[data-role=gpu]', card).value,
        snapshot: $('[data-role=snap]', card).value,
        no_save: $('[data-role=nosave]', card).checked,
        no_window: !$('[data-role=showwin]', card).checked,
      };
      await api(`/v1/avds/${name}/start`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
      toast(name + " starting…");
    } else if (act === "stop") {
      await api(`/v1/avds/${name}/stop`, { method: "POST" });
      toast(name + " stopped");
    } else if (act === "save-snap") {
      const n = $('[data-role=newsnap]', card).value.trim();
      if (!n) throw new Error("snapshot name required");
      await api(`/v1/avds/${name}/snapshots?name=${encodeURIComponent(n)}`, { method: "POST" });
      toast("snapshot " + n + " saved");
    } else if (act === "boot-snap") {
      await api(`/v1/avds/${name}/start`, { method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ gpu: "host", snapshot: btn.dataset.snap, no_save: true, no_window: false }) });
      toast(name + " booting " + btn.dataset.snap + "…");
    } else if (act === "apply-cfg") {
      const patch = {};
      card.querySelectorAll("[data-k]").forEach(i => { patch[i.dataset.k] = parseInt(i.value, 10); });
      const r = await api(`/v1/avds/${name}/config`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(patch) });
      toast("config updated: " + JSON.stringify(r));
    } else if (act === "install") {
      const f = $('[data-role=apk]', card).files[0];
      if (!f) throw new Error("choose an .apk file");
      await api(`/v1/apk/install?port=${portOf(card)}`, { method: "POST", headers: { "Content-Type": "application/vnd.android.package-archive" }, body: f });
      toast(f.name + " installed");
    } else if (act === "log") {
      const t = await api(`/v1/avds/${name}/log?tail=60`);
      $('[data-role=log]', card).textContent = t.log || "(empty)";
    } else if (act === "shot") {
      const img = $('[data-role=preview]', card);
      if (img) previewImg(img, portOf(card));
    }
  } catch (err) { toast("error: " + err.message); }
  btn.disabled = false;
  loadDevices();
}

function portOf(card) {
  const tag = $(".dev-head .tag", card).textContent;
  const m = tag.match(/adb:(\d+)/);
  return m ? m[1] : "5555";
}

document.addEventListener("click", onClick);
$("#refresh").addEventListener("click", () => { loadServer(); loadDevices(); });
$("#save-token").addEventListener("click", () => {
  state.token = $("#token").value.trim();
  localStorage.setItem("emu_token", state.token);
  toast("token saved"); loadServer(); loadDevices();
});
$("#token").value = state.token;
loadServer(); loadDevices();
setInterval(loadDevices, 15000);
