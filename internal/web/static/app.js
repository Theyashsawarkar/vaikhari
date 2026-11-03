// vaikhari in the browser: the same posts, drafts, schedule and approvals
// as the terminal screen, with previews that look like X's and LinkedIn's.
"use strict";

const TOKEN = document.querySelector("meta[name=vaikhari-token]").content;
const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];

let S = { stamp: "", accounts: null, posts: [] }; // the server's state
let C = blank(); // the post in the editor
let M = null; // its measure (lengths, X parts, LinkedIn fold)
let view = "compose";
let liOpen = false; // LinkedIn preview expanded past "…see more"
let lost = false; // the server stopped answering
let seenReady = null; // ids of the posts already waiting for X
let busy = false; // a post is on its way: no second one
const retrying = new Set(); // posts whose LinkedIn retry is on its way

function blank() {
  return { id: 0, status: "draft", text: "", networks: S.accounts ? [...S.accounts.networks] : ["x", "linkedin"],
    images: [], when: "", approving: null, saved: "" };
}

// ---------------------------------------------------------------- server

async function api(method, path, body) {
  const opt = { method, headers: { "X-Vaikhari-Token": TOKEN } };
  if (body instanceof FormData) opt.body = body;
  else if (body !== undefined) { opt.body = JSON.stringify(body); opt.headers["Content-Type"] = "application/json"; }
  const r = await fetch(path, opt);
  let data = {};
  try { data = await r.json(); } catch { /* empty */ }
  if (!r.ok) { const e = new Error(data.error || r.statusText); e.data = data; throw e; }
  return data;
}

async function refresh(force) {
  try {
    const st = await api("GET", "/api/state");
    const changed = force || st.stamp !== S.stamp || JSON.stringify(st.accounts) !== JSON.stringify(S.accounts);
    const first = !S.accounts;
    S = st;
    if (first && !C.id && !C.text) C.networks = [...S.accounts.networks];
    if (changed) { renderSide(); if (view === "compose") renderPreviews(); else renderList(); }
    if (lost) { lost = false; toast("Back in touch with vaikhari"); }
    noticeWaiting();
  } catch (e) {
    if (!lost) toast("Lost vaikhari: is `vaikhari web` still running? (" + e.message + ")", true);
    lost = true;
  }
}

// noticeWaiting says when a scheduled X post comes due: X needs your click.
function noticeWaiting() {
  const ready = S.posts.filter((p) => p.status === "ready");
  const n = ready.length + S.posts.filter((p) => p.status === "approval").length;
  document.title = n ? `(${n}) vaikhari` : "vaikhari";
  if (seenReady) for (const p of ready) if (p.send_at && !seenReady.has(p.id)) toast(`#${p.id} is due on X: open it from the Queue`);
  seenReady = new Set(ready.map((p) => p.id));
}

async function loadTheme() {
  try {
    const t = await api("GET", "/api/theme");
    const root = document.documentElement;
    for (const [k, v] of Object.entries(t.colors)) root.style.setProperty("--" + k.toLowerCase(), v);
    root.classList.toggle("light", !!t.light);
  } catch { /* the built-in colours stay */ }
}

// ---------------------------------------------------------------- helpers

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
}

// linkify: escaped text with links, #tags and @names coloured.
function linkify(s) {
  return esc(s).replace(/(https?:\/\/[^\s<]+|\b[\w-]+\.(?:com|org|net|io|dev|in|co|ai|app|me|xyz|gg|tv)(?:\/[^\s<]*)?)|(^|[\s(])([#@][\p{L}\p{N}_]+)/gu,
    (m, url, pre, tag) => url ? `<a>${url}</a>` : `${pre}<span class="tag">${tag}</span>`);
}

function debounce(fn, ms) {
  let t;
  return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); };
}

const fmt = (n) => n.toLocaleString("en-US");

function ago(ms) {
  const d = (Date.now() - ms) / 1000;
  if (d < 60) return "just now";
  if (d < 3600) return Math.floor(d / 60) + " min ago";
  if (d < 86400) return Math.floor(d / 3600) + " h ago";
  return new Date(ms).toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

const safeURL = (u) => /^https:\/\//.test(u || "");

function initial(name) { return (name || "?").trim().charAt(0).toUpperCase() || "?"; }

let toastT;
function toast(msg, bad) {
  const t = $("#toast");
  t.textContent = msg;
  t.classList.toggle("bad", !!bad);
  t.classList.add("on");
  clearTimeout(toastT);
  toastT = setTimeout(() => t.classList.remove("on"), bad ? 6000 : 3200);
}

const ICON = {
  reply: '<svg viewBox="0 0 24 24"><path d="M21 12a8 8 0 0 1-11.6 7.2L4 21l1.8-5.4A8 8 0 1 1 21 12z"/></svg>',
  repost: '<svg viewBox="0 0 24 24"><path d="m17 2 4 4-4 4"/><path d="M3 11V9a3 3 0 0 1 3-3h15"/><path d="m7 22-4-4 4-4"/><path d="M21 13v2a3 3 0 0 1-3 3H3"/></svg>',
  like: '<svg viewBox="0 0 24 24"><path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21.2l8.8-8.8a5.5 5.5 0 0 0 0-7.8z"/></svg>',
  views: '<svg viewBox="0 0 24 24"><path d="M4 20V10M10 20V4M16 20v-7M22 20H2"/></svg>',
  thumb: '<svg viewBox="0 0 24 24"><path d="M7 10v11H3V10zM7 10l4-8a3 3 0 0 1 3 3v4h5.5a2 2 0 0 1 2 2.3l-1.4 8A2 2 0 0 1 18 21H7"/></svg>',
  comment: '<svg viewBox="0 0 24 24"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>',
  send: '<svg viewBox="0 0 24 24"><path d="m22 2-7 20-4-9-9-4z"/><path d="M22 2 11 13"/></svg>',
  globe: '<svg viewBox="0 0 24 24" style="width:12px;height:12px;vertical-align:-1px"><circle cx="12" cy="12" r="10"/><path d="M2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20"/></svg>',
  ext: '<svg viewBox="0 0 24 24"><path d="M15 3h6v6M10 14 21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/></svg>',
  x: '<svg class="logo" viewBox="0 0 24 24"><path d="M18.9 2H22l-7.2 8.2L23 22h-6.6l-5.2-6.8L5.3 22H2.2l7.7-8.8L2 2h6.8l4.7 6.2zm-1.1 18h1.7L7.2 3.9H5.4z"/></svg>',
  li: '<svg class="logo" viewBox="0 0 24 24" style="color:#0a66c2"><path d="M20.4 2H3.6A1.6 1.6 0 0 0 2 3.6v16.8A1.6 1.6 0 0 0 3.6 22h16.8a1.6 1.6 0 0 0 1.6-1.6V3.6A1.6 1.6 0 0 0 20.4 2zM8 19H5V9.5h3zM6.5 8.2a1.7 1.7 0 1 1 0-3.5 1.7 1.7 0 0 1 0 3.5zM19 19h-3v-4.6c0-1.1 0-2.5-1.5-2.5S12.8 13 12.8 14.3V19h-3V9.5h2.8v1.3a3.1 3.1 0 0 1 2.8-1.5c3 0 3.6 2 3.6 4.6z"/></svg>',
  empty: '<svg viewBox="0 0 24 24"><path d="M22 12h-6l-2 3h-4l-2-3H2"/><path d="M5.5 5.1 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.5-6.9A2 2 0 0 0 16.8 4H7.2a2 2 0 0 0-1.7 1.1z"/></svg>',
};

// ---------------------------------------------------------------- side

function renderSide() {
  const ps = S.posts;
  const q = ps.filter((p) => ["approval", "ready", "failed", "scheduled"].includes(p.status));
  const waiting = q.filter((p) => p.status !== "scheduled").length;
  const nq = $("#n-queue");
  nq.textContent = q.length || "";
  nq.classList.toggle("hot", waiting > 0);
  $("#n-drafts").textContent = ps.filter((p) => p.status === "draft").length || "";
  $("#n-posted").textContent = ps.filter((p) => p.status === "posted").length || "";
  const a = S.accounts;
  const li = a.linkedin;
  let liLine;
  if (li.ok) {
    const warn = li.days < 7;
    liLine = `<div class="acct"><span class="dot" style="background:var(--${warn ? "yellow" : "green"})"></span><div>${esc(li.name)}<div class="what">LinkedIn · ${li.days} days left</div></div>${warn ? '<button class="soft small" data-act="signin">Renew</button>' : ""}</div>`;
  } else {
    liLine = `<div class="acct"><span class="dot" style="background:var(--red)"></span><div>LinkedIn<div class="what">${li.has_app ? "not signed in" : "not set up"}</div></div><button class="soft small" data-act="${li.has_app ? "signin" : "setup"}">${li.has_app ? "Sign in" : "Set up"}</button></div>`;
  }
  $("#accounts").innerHTML = liLine +
    `<div class="acct"><span class="dot" style="background:var(--sky)"></span><div>@${esc(a.x.handle)}<div class="what">X · one click, free</div></div></div>`;
}

// ---------------------------------------------------------------- compose

// edits to a scheduled or failed post aren't saved as you type (that would take
// it off the schedule): leaving them asks first
function dirtyPost() {
  return !C.approving && C.id && C.status !== "draft" && snapshot() !== C.saved;
}
function askLeave(then) {
  if (!dirtyPost()) return then();
  leaveNext = then;
  modal(`<h3>Leave your edits to #${C.id}?</h3><p>They aren't saved: ${C.status === "scheduled" ? "Schedule" : "Post now"} saves them.</p>
    <div class="row"><button class="ghost" data-act="close">Keep editing</button><button class="danger" data-act="leave-yes">Leave them</button></div>`);
}
let leaveNext = null;

async function loadIntoEditor(p, approving) {
  if (dirtyPost()) return askLeave(() => { C.saved = snapshot(); loadIntoEditor(p, approving); });
  await flush();
  C = { id: p.id, status: p.status, text: p.text, networks: [...(p.networks || [])],
    images: (p.images || []).map((path, i) => ({ path, url: p.image_urls[i] })), when: "",
    approving: approving ? p : null, saved: "" };
  C.saved = snapshot();
  if ((p.status === "scheduled" || approving) && p.send_at) {
    const d = new Date(p.send_at);
    const pad = (n) => String(n).padStart(2, "0");
    C.when = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }
  show("compose");
  syncEditor();
  $("#text").focus();
}

function syncEditor() {
  $("#text").value = C.text;
  $("#when").value = C.when;
  $$(".net").forEach((b) => b.classList.toggle("on", C.networks.includes(b.dataset.net)));
  const titles = { draft: C.id ? `Draft #${C.id}` : "New post", scheduled: `Scheduled #${C.id}`, failed: `#${C.id} (failed)` };
  $("#editor-title").textContent = C.approving ? `#${C.id}, written for you` : (titles[C.status] || "New post");
  $("#save-draft").hidden = !!C.approving;
  $("label.tool").hidden = !!C.approving;
  // an agent's post can go to fewer networks than it asked for, not to more
  $$(".net").forEach((b) => { b.disabled = !!C.approving && !C.approving.networks.includes(b.dataset.net); });
  const b = $("#approve-banner");
  if (C.approving) {
    const p = C.approving;
    b.hidden = false;
    b.innerHTML = `<div><div class="who">${esc(p.requester || "An agent")} wants to post this as you</div>${p.note ? `<div class="why">Why: ${esc(p.note)}</div>` : ""}</div>
      <div class="btns"><button class="danger" data-act="decline-cur">Decline</button></div>`;
  } else b.hidden = true;
  renderThumbs();
  whenChanged();
  measure();
  $("#saved").textContent = "";
}

function renderThumbs() {
  $("#thumbs").innerHTML = C.images.map((im, i) =>
    `<div class="thumb"><img src="${esc(im.url)}" alt="">${C.approving ? "" : `<button title="Remove" aria-label="Remove this image" data-rm="${i}">✕</button>`}</div>`).join("");
}

const measure = debounce(async () => {
  try { M = await api("POST", "/api/check", { text: C.text }); } catch { return; }
  renderPreviews();
}, 90);

function ring(len, limit) {
  const r = 8, c = 2 * Math.PI * r;
  const frac = Math.min(len / limit, 1);
  const color = len > limit ? "var(--red)" : len >= limit - 20 ? "#ffd400" : "#1d9bf0";
  const left = limit - len;
  return `<span class="ring">${left <= 20 ? `<span style="color:${color}">${left}</span>` : ""}<svg viewBox="0 0 20 20"><circle class="track" cx="10" cy="10" r="${r}" fill="none" stroke-width="2"/><circle class="fill" cx="10" cy="10" r="${r}" fill="none" stroke="${color}" stroke-width="2" stroke-dasharray="${c}" stroke-dashoffset="${c * (1 - frac)}"/></svg></span>`;
}

function media(cls, urls) {
  if (!urls.length) return "";
  const n = Math.min(urls.length, 4);
  return `<div class="${cls} n${n}">${urls.slice(0, 4).map((u) => `<img src="${esc(u)}" alt="">`).join("")}</div>`;
}

function renderPreviews() {
  if (!S.accounts) return;
  const a = S.accounts;
  const xName = a.x.name, handle = a.x.handle, liName = a.linkedin.ok ? a.linkedin.name : "You";
  const urls = C.images.map((i) => i.url);
  const m = M || { x_parts: [], x_limit: 280, li_len: 0, li_limit: 3000, li_text: "", li_fold: "" };

  // X
  const px = $("#pv-x");
  px.classList.toggle("off", !C.networks.includes("x"));
  if (!C.networks.includes("x")) {
    px.innerHTML = `<div class="x-empty">Not posting to X. Tick X below the editor to add it.</div>`;
    $("#x-meta").textContent = "";
  } else if (!m.x_parts.length) {
    px.innerHTML = `<div class="x-empty">Your post shows here as X will show it.</div>`;
    $("#x-meta").textContent = "";
  } else {
    const parts = m.x_parts;
    px.innerHTML = parts.map((p, i) => `
      <div class="tw">
        <div class="rail"><div class="av">${initial(xName)}</div>${i < parts.length - 1 ? '<div class="thread-line"></div>' : ""}</div>
        <div>
          <div class="who"><b>${esc(xName)}</b><span class="h">@${esc(handle)} · now</span></div>
          <div class="body">${linkify(p.text)}</div>
          ${i === 0 ? media("media", urls) : ""}
          <div class="foot">${ICON.reply}${ICON.repost}${ICON.like}${ICON.views}${ring(p.len, m.x_limit)}</div>
        </div>
      </div>`).join("");
    const longest = Math.max(...parts.map((p) => p.len));
    $("#x-meta").innerHTML = parts.length > 1 ? `a thread of ${parts.length} · longest ${longest}/${m.x_limit}` : `${longest}/${m.x_limit}`;
  }

  // LinkedIn
  const pl = $("#pv-li");
  pl.classList.toggle("off", !C.networks.includes("linkedin"));
  const head = `<div class="li-head"><div class="av">${a.linkedin.picture ? `<img src="${esc(a.linkedin.picture)}" alt="">` : initial(liName)}</div>
    <div><b>${esc(liName)}</b><small>Your headline</small><small>Just now · ${ICON.globe}</small></div></div>`;
  if (!C.networks.includes("linkedin")) {
    pl.innerHTML = `<div class="li-empty">Not posting to LinkedIn. Tick LinkedIn below the editor to add it.</div>`;
    $("#li-meta").textContent = "";
  } else if (!m.li_text) {
    pl.innerHTML = head + `<div class="li-empty">Your post shows here as LinkedIn will show it.</div>`;
    $("#li-meta").textContent = "";
  } else {
    const folded = m.li_fold && !liOpen;
    const body = folded ? `${linkify(m.li_fold)}<span>…</span> <button class="more" data-act="li-more">see more</button>` : linkify(m.li_text);
    pl.innerHTML = head + `<div class="li-body">${body}</div>` + media("li-media", urls) +
      `<div class="li-actions"><span>${ICON.thumb}Like</span><span>${ICON.comment}Comment</span><span>${ICON.repost}Repost</span><span>${ICON.send}Send</span></div>`;
    const pct = Math.min(100, (m.li_len / m.li_limit) * 100);
    const col = m.li_len > m.li_limit ? "var(--red)" : pct > 90 ? "var(--yellow)" : "var(--green)";
    $("#li-meta").innerHTML = `<span class="meter"><span class="bar"><i style="width:${pct}%;background:${col}"></i></span>${fmt(m.li_len)} / ${fmt(m.li_limit)}${m.li_fold ? " · folds at “see more”" : ""}</span>`;
  }
  updateGo();
}

// blocker is why the post can't go yet ("" when it can).
function blocker() {
  if (!C.text.trim()) return "";
  if (M && !M.x_parts.length) return "There's nothing to post yet";
  if (!C.networks.length) return "Pick X or LinkedIn";
  if (whenBad) return "Fix the time, or clear it to post now";
  if (M && C.networks.includes("linkedin") && M.li_len > M.li_limit) return `${fmt(M.li_len - M.li_limit)} over LinkedIn's limit`;
  if (C.networks.includes("x") && C.images.length > 4) return "X takes at most 4 images";
  return "";
}

function updateGo() {
  const why = blocker();
  const go = $("#go");
  go.disabled = busy || !C.text.trim() || !!why;
  const el = $("#saved");
  if (why) { el.textContent = why; el.classList.add("bad"); el.dataset.why = "1"; }
  else if (el.dataset.why) { el.textContent = ""; el.classList.remove("bad"); delete el.dataset.why; }
  $("#go-label").textContent = C.approving ? (whenOK ? "Approve & schedule" : "Approve & post") : whenOK ? "Schedule" : "Post now";
}

let whenOK = null; // the parsed time, when the field holds one
let whenBad = false; // the field holds something that isn't a time
let whenReading = null;
async function readWhen() {
  const v = $("#when").value.trim();
  C.when = v;
  const say = $("#when-say");
  if (!v) { whenOK = null; whenBad = false; say.textContent = ""; say.className = ""; updateGo(); return; }
  try {
    const r = await api("POST", "/api/when", { text: v });
    if ($("#when").value.trim() !== v) return; // typed on since
    whenOK = r.ok ? r : null;
    whenBad = !r.ok;
    say.textContent = r.ok ? `Goes out ${r.full}` : r.error;
    say.className = r.ok ? "ok" : "bad";
  } catch { whenOK = null; whenBad = true; }
  updateGo();
}
const whenChanged = debounce(() => { whenReading = readWhen(); }, 250);

// autosave: a draft you write is kept as you type
let saveTimer, saving = null;
function autosave() {
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => { saveNow(); }, 1400);
}
// saves run one at a time: two at once would make two drafts
function saveNow() {
  saving = (saving || Promise.resolve()).then(doAutosave).finally(() => { saving = null; });
  return saving;
}
// what a draft's save would change
const snapshot = () => JSON.stringify([C.text, C.networks, C.images.map((i) => i.path)]);
function unsaved() {
  return !C.approving && C.status === "draft" && !!C.text.trim() && snapshot() !== C.saved;
}
async function doAutosave() {
  if (!unsaved()) return;
  const cur = C, snap = snapshot(), b = body();
  b.when = "";
  try {
    const p = await api("POST", "/api/drafts", b);
    cur.id = p.id; cur.saved = snap;
    if (C !== cur) return; // the editor has moved on to another post
    $("#editor-title").textContent = `Draft #${p.id}`;
    $("#saved").textContent = "Saved as a draft";
    refresh();
  } catch (e) { if (C === cur) $("#saved").textContent = "Not saved: " + e.message; }
}
// settle: no autosave pending or running, so C.id is final (no second post)
async function settle() {
  clearTimeout(saveTimer);
  if (saving) await saving;
}
// flush: what's typed is kept before the editor moves on
async function flush() {
  await settle();
  await doAutosave();
}

function body() {
  return { id: C.id, text: C.text, networks: C.networks, images: C.images.map((i) => i.path), when: whenOK ? C.when : "" };
}

async function saveDraft() {
  if (!C.text.trim()) return toast("Write something first", true);
  await settle();
  try {
    const b = body();
    b.when = "";
    const p = await api("POST", "/api/drafts", b);
    C.id = p.id; C.status = "draft"; C.saved = snapshot();
    syncEditor();
    $("#saved").textContent = "Saved as a draft";
    toast(`Saved draft #${p.id}`);
    refresh(true);
  } catch (e) { toast(e.message, true); }
}

function whatHappens(networks) {
  const li = S.accounts.linkedin;
  const lines = [];
  if (networks.includes("linkedin")) {
    lines.push(li.ok ? `<div class="net-line">${ICON.li}<div><b>LinkedIn</b> goes out at once, as ${esc(li.name)}.</div></div>`
      : `<div class="net-line">${ICON.li}<div><b>LinkedIn</b> <span style="color:var(--red)">isn't signed in</span>: it will fail (Retry in Queue after signing in), or untick it.</div></div>`);
  }
  if (networks.includes("x")) {
    const n = M ? M.x_parts.length : 1;
    lines.push(`<div class="net-line">${ICON.x}<div><b>X</b> opens in a new tab with it filled in: you press Post.${n > 1 ? ` A thread of ${n}, one part at a time.` : ""}${C.images.length ? " The image is copied for you to paste." : ""}</div></div>`);
  }
  return lines.join("");
}

async function go() {
  if (busy) return;
  // what's in the time field decides what this button does: read it first,
  // however fast the click came
  await readWhen();
  if (!C.text.trim() || blocker()) return;
  await settle();
  if (whenOK) return C.approving ? approveCurrent() : schedule();
  const who = C.approving ? `<p>${esc(C.approving.requester || "An agent")} wrote this. It goes out under your name.</p>` : "";
  modal(`<h3>${C.approving ? "Approve and post it now?" : "Post it now?"}</h3>${who}${whatHappens(C.networks)}
    <div class="row"><button class="ghost" data-act="close">Not yet</button><button class="primary" data-act="${C.approving ? "approve-now" : "post-now"}">${C.approving ? "Approve & post" : "Post now"}</button></div>`);
}

function setBusy(on) { busy = on; updateGo(); }

async function schedule() {
  setBusy(true);
  try {
    const r = await api("POST", "/api/submit", body());
    if (r.error || r.post.status !== "scheduled") { toast(r.error || `#${r.post.id} isn't scheduled`, true); refresh(true); setBusy(false); return; }
    toast(`Scheduled #${r.post.id}: goes out ${r.post.say}`);
    C = blank(); syncEditor(); refresh(true);
  } catch (e) { toast(e.message, true); }
  setBusy(false);
}

async function postNow() {
  closeModal();
  if (busy) return;
  setBusy(true);
  await flush();
  try {
    const r = await api("POST", "/api/submit", body());
    if (r.post.status === "draft") { toast(r.error || "It didn't go out", true); setBusy(false); refresh(true); return; }
    C = blank(); syncEditor();
    await afterPublish(r.post, r.error);
  } catch (e) { toast(e.message, true); }
  setBusy(false);
  refresh(true);
}

async function afterPublish(p, err) {
  if (err) toast("LinkedIn: " + err, true);
  else if (p.networks.includes("linkedin") && p.results && p.results.linkedin && p.results.linkedin.state === "posted") toast("Posted on LinkedIn");
  if (p.networks.includes("x") && p.status !== "scheduled" && (!p.results || !p.results.x || ["pending", "opened"].includes(p.results.x.state))) await xFlow(p);
}

// ---------------------------------------------------------------- X, one click

async function xFlow(p) {
  let r;
  try { r = await api("POST", `/api/posts/${p.id}/x-open`); } catch (e) { return toast(e.message, true); }
  const post = r.post;
  const parts = post.x_parts.length;
  const part = post.x_next;
  let png = null;
  if (part === 1 && post.image_urls && post.image_urls.length) {
    png = await toPNG(post.image_urls[0]).catch(() => null);
  }
  const thread = parts > 1 && part < parts;
  xWarned = false;
  modal(`<h3>Post it on X${parts > 1 ? ` · part ${part} of ${parts}` : ""}</h3>
    <ol class="steps">
      <li><b class="k">1</b><div>Open X: the post is filled in.${png ? " The image is copied: paste it with <code>Ctrl+V</code>." : ""}<div style="margin-top:10px"><a class="openx" id="openx" href="${esc(r.url)}" target="_blank" rel="noopener noreferrer">${ICON.x} Open X ${ICON.ext}</a></div></div></li>
      <li><b class="k">2</b><div>Press <b>Post</b> there.</div></li>
      <li><b class="k">3</b><div>Back here, ${parts > 1 && part < parts ? "paste the post's link (Share → Copy link) so the next part replies to it" : "paste its link if you want it kept (optional)"}:
        <div class="link-row"><input type="text" id="xlink" placeholder="https://x.com/you/status/…" aria-label="The post's link on X"><button class="soft small" data-act="x-paste">Paste</button></div>
        <p class="link-say" id="xlink-say" data-thread="${thread ? 1 : ""}" role="status"></p></div></li>
    </ol>
    <div class="row"><button class="ghost" data-act="x-skip" data-id="${post.id}">Skip X</button><button class="ghost" data-act="close">Later</button><button class="primary" id="x-done" data-act="x-posted" data-id="${post.id}">I posted it</button></div>`);
  $("#xlink").addEventListener("input", xLinkSay);
  $("#openx").addEventListener("click", () => {
    if (png && navigator.clipboard && window.ClipboardItem) {
      navigator.clipboard.write([new ClipboardItem({ "image/png": png })]).catch(() => toast("Couldn't copy the image: drag it into X instead", true));
    }
  });
}

async function toPNG(url) {
  const blob = await (await fetch(url)).blob();
  if (blob.type === "image/png") return blob;
  const bmp = await createImageBitmap(blob);
  const c = document.createElement("canvas");
  c.width = bmp.width; c.height = bmp.height;
  c.getContext("2d").drawImage(bmp, 0, 0);
  return await new Promise((ok) => c.toBlob(ok, "image/png"));
}

const X_STATUS = /^https?:\/\/(www\.)?(x|twitter)\.com\/[^/\s]+\/status(es)?\/\d+/;
let xWarned = false; // you've been told a thread needs the link

function xLinkSay() {
  const v = $("#xlink").value.trim(), say = $("#xlink-say"), thread = say.dataset.thread;
  xWarned = false;
  $("#x-done").textContent = "I posted it";
  if (!v) { say.textContent = ""; say.className = "link-say"; return; }
  const ok = X_STATUS.test(v);
  say.textContent = ok ? (thread ? "✓ The next part will reply to this" : "✓ Kept with the post") : "That isn't a link to a post on X";
  say.className = "link-say " + (ok ? "ok" : "bad");
}

async function xPasteLink() {
  try {
    $("#xlink").value = (await navigator.clipboard.readText()).trim();
    xLinkSay();
  } catch { toast("The browser wouldn't share the clipboard: paste with Ctrl+V", true); }
}

async function xPosted(id) {
  const link = ($("#xlink") || {}).value || "";
  const say = $("#xlink-say");
  // without this link X can't chain the next part: a thread falls apart
  if (!link.trim() && say && say.dataset.thread && !xWarned) {
    xWarned = true;
    say.textContent = "Without its link, the next part won't reply to this one: it goes out as a separate post.";
    say.className = "link-say bad";
    $("#x-done").textContent = "Go on without it";
    $("#xlink").focus();
    return;
  }
  try {
    const r = await api("POST", `/api/posts/${id}/x-posted`, { link: link.trim() });
    closeModal();
    if (r.more) return xFlow(r.post);
    toast("Posted on X");
  } catch (e) { toast(e.message, true); }
  refresh(true);
}

// ---------------------------------------------------------------- approve

async function approveCurrent() {
  closeModal();
  if (busy) return;
  setBusy(true);
  const p = C.approving;
  // what you see is what goes: the networks ticked and the time in the field
  const answer = { networks: C.networks, when: whenOK ? C.when : "" };
  if (C.text !== p.text) answer.text = C.text;
  try {
    const r = await api("POST", `/api/posts/${p.id}/approve`, answer);
    C = blank(); syncEditor();
    if (r.post.status === "scheduled") toast(`Approved: goes out ${r.post.say}`);
    else await afterPublish(r.post);
  } catch (e) {
    toast(e.message, true);
    if (e.data && e.data.post && e.data.post.status !== "approval") { C = blank(); syncEditor(); await afterPublish(e.data.post, e.message); }
  }
  setBusy(false);
  refresh(true);
}

// ---------------------------------------------------------------- lists

const LISTS = {
  queue: { title: "Queue", sub: "Waiting on you, then the schedule, soonest first.", statuses: ["approval", "ready", "failed", "scheduled"] },
  drafts: { title: "Drafts", sub: "Posts you kept for later. Writing in Compose saves them here as you type.", statuses: ["draft"] },
  posted: { title: "Posted", sub: "What went out, with its links.", statuses: ["posted"] },
};

const STATUS = {
  approval: ["needs your approval", "var(--peach)"], ready: ["waiting for your click on X", "var(--yellow)"],
  failed: ["failed", "var(--red)"], scheduled: ["scheduled", "var(--blue)"], draft: ["draft", "var(--muted)"], posted: ["posted", "var(--green)"],
};

function renderList() {
  const L = LISTS[view];
  $("#list-title").textContent = L.title;
  $("#list-sub").textContent = L.sub;
  let ps = S.posts.filter((p) => L.statuses.includes(p.status));
  if (view === "queue") {
    const now = ps.filter((p) => p.status !== "scheduled");
    const later = ps.filter((p) => p.status === "scheduled").sort((a, b) => a.send_at - b.send_at);
    ps = [...now, ...later];
  }
  if (!ps.length) {
    $("#list").innerHTML = `<div class="empty">${ICON.empty}<div>${view === "queue" ? "Nothing waiting or scheduled." : view === "drafts" ? "No drafts." : "Nothing posted yet."}</div></div>`;
    return;
  }
  $("#list").innerHTML = ps.map(item).join("");
}

function result(p, n) {
  const r = (p.results || {})[n] || {};
  const name = n === "x" ? "X" : "LinkedIn";
  const st = r.state || "pending";
  let html;
  if (st === "posted") html = safeURL(r.url) ? `<span class="ok">✓</span> <a href="${esc(r.url)}" target="_blank" rel="noopener noreferrer">${esc(r.url)}</a>` : `<span class="ok">✓ posted</span>`;
  else if (st === "opened") html = `<span class="wait">open in your browser: press Post there, then confirm</span>`;
  else if (st === "failed") html = `<span class="err">✗ ${esc(r.error || "failed")}</span>`;
  else if (st === "skipped") html = `<span style="color:var(--muted)">skipped</span>`;
  else if (n === "x" && p.status === "ready") html = `<span class="wait">ready: part ${p.x_next} of ${p.x_parts.length}</span>`;
  else html = `<span style="color:var(--muted)">not yet</span>`;
  return `<div><span class="n">${name}</span>${html}</div>`;
}

function item(p) {
  const [label, color] = STATUS[p.status] || [p.status, "var(--muted)"];
  const when = p.status === "scheduled" ? `goes out ${esc(p.say)}` : ago(p.updated);
  const nets = p.networks.map((n) => `<span class="badge">${n === "x" ? ICON.x + "X" : ICON.li + "LinkedIn"}</span>`).join("");
  const by = p.origin === "agent" ? ` · by ${esc(p.requester || "an agent")}` : "";
  const st = (n) => ((p.results || {})[n] || {}).state || "pending";
  const btn = (act, text, cls = "soft") => `<button class="${cls} small" data-act="${act}" data-id="${p.id}">${text}</button>`;
  let btns = [];
  switch (p.status) {
    case "approval": btns = [btn("review", "Review & approve", "primary"), btn("decline", "Decline", "danger")]; break;
    case "scheduled": btns = [btn("edit", "Edit"), btn("post", "Post now"), btn("move", "Move"), btn("cancel", "Cancel", "ghost")]; break;
    case "ready": case "failed":
      if (st("linkedin") === "failed") btns.push(btn("retry", "Retry LinkedIn"));
      if (p.networks.includes("x") && !["posted", "skipped"].includes(st("x"))) btns.push(btn("xopen", "Open on X", "primary"));
      if (p.status === "failed" && !p.networks.includes("x")) btns.push(btn("edit", "Edit"));
      btns.push(btn("cancel", "Cancel", "ghost"));
      break;
    case "draft": btns = [btn("edit", "Edit", "primary"), btn("delete", "Delete", "ghost")]; break;
  }
  return `<div class="item card"><div class="stripe" style="background:${color}"></div><div class="in">
    <div class="top"><span class="pill" style="background:color-mix(in srgb, ${color} 18%, transparent);color:${color}">${label}</span>${nets}<span>#${p.id} · ${when}${by}</span></div>
    ${p.note ? `<div class="why">Why: ${esc(p.note)}</div>` : ""}
    <div class="text${p.text.length > 240 || p.text.split("\n").length > 3 ? " clip" : ""}" data-act="expand">${linkify(p.text).replace(/^---$/gm, '<span class="break"></span>')}</div>
    ${["draft", "approval", "scheduled"].includes(p.status) ? "" : `<div class="res">${p.networks.map((n) => result(p, n)).join("")}</div>`}
    ${btns.length ? `<div class="btns">${btns.join("")}</div>` : ""}
  </div></div>`;
}

function postById(id) { return S.posts.find((p) => p.id === Number(id)); }

async function act(name, id, el) {
  const p = postById(id);
  try {
    switch (name) {
      case "edit": return loadIntoEditor(p);
      case "review": return loadIntoEditor(p, true);
      case "decline": await api("POST", `/api/posts/${id}/decline`); toast(`Declined #${id}`); break;
      case "decline-cur": await api("POST", `/api/posts/${C.approving.id}/decline`); toast("Declined. Nothing was posted."); C = blank(); syncEditor(); break;
      case "cancel":
        return modal(`<h3>Cancel #${id}?</h3><p>${esc(p.text.slice(0, 160))}${p.text.length > 160 ? "…" : ""}</p><p>It won't go out${p.status === "scheduled" ? "" : " anywhere else"}, and it can't be brought back.</p>
          <div class="row"><button class="ghost" data-act="close">Keep it</button><button class="danger" data-act="cancel-yes" data-id="${id}">Cancel the post</button></div>`);
      case "cancel-yes": closeModal(); await api("POST", `/api/posts/${id}/cancel`); toast(`Cancelled #${id}`); break;
      case "retry": {
        if (retrying.has(id)) return;
        retrying.add(id); el.disabled = true;
        try { const r = await api("POST", `/api/posts/${id}/retry`); toast("Posted on LinkedIn"); await afterPublish(r.post); }
        finally { retrying.delete(id); }
        break;
      }
      case "xopen": return xFlow(p);
      case "delete":
        return modal(`<h3>Delete draft #${id}?</h3><p>${esc(p.text.slice(0, 160))}${p.text.length > 160 ? "…" : ""}</p>
          <div class="row"><button class="ghost" data-act="close">Keep it</button><button class="danger" data-act="delete-yes" data-id="${id}">Delete</button></div>`);
      case "delete-yes": closeModal(); await api("DELETE", `/api/posts/${id}`); toast(`Deleted #${id}`); if (C.id === Number(id)) { C = blank(); syncEditor(); } break;
      case "post":
        return modal(`<h3>Post #${id} now?</h3>${whatHappens(p.networks)}
          <div class="row"><button class="ghost" data-act="close">Not yet</button><button class="primary" data-act="post-id" data-id="${id}">Post now</button></div>`);
      case "post-id": {
        closeModal();
        const r = await api("POST", "/api/submit", { id: p.id, text: p.text, networks: p.networks, images: p.images || [], when: "" });
        await afterPublish(r.post, r.error);
        break;
      }
      case "move":
        return modal(`<h3>Move #${id}</h3><label>When should it go out?</label><input type="text" id="move-when" placeholder="tomorrow 9am · fri 6pm · in 2h">
          <div class="row"><button class="ghost" data-act="close">Cancel</button><button class="primary" data-act="move-yes" data-id="${id}">Move</button></div>`, "#move-when");
      case "move-yes": { if (!$("#move-when").value.trim()) return toast("When should it go out?", true); const r = await api("POST", `/api/posts/${id}/reschedule`, { when: $("#move-when").value }); closeModal(); toast(`#${id} goes out ${r.post.say}`); break; }
      case "x-posted": return xPosted(id);
      case "x-skip": closeModal(); await api("POST", `/api/posts/${id}/x-skip`); toast("Skipped X for this post"); break;
      case "post-now": return postNow();
      case "approve-now": return approveCurrent();
      case "leave-yes": { closeModal(); const f = leaveNext; leaveNext = null; return f && f(); }
      case "x-paste": return xPasteLink();
      case "close": return closeModal();
      case "expand": return el.classList.toggle("open");
      case "li-more": liOpen = true; return renderPreviews();
      case "signin": return signIn();
      case "setup": return setup();
      case "setup-save": return setupSave();
      case "settings-save": return settingsSave();
    }
  } catch (e) { toast(e.message, true); }
  refresh(true);
}

// ---------------------------------------------------------------- settings, LinkedIn

let modalFrom = null; // what had the focus, to give it back
function modal(html, focus) {
  if ($("#modal-bg").hidden) modalFrom = document.activeElement;
  $("#modal").innerHTML = html;
  $("#modal-bg").hidden = false;
  const f = focus ? $(focus) : $("#modal input, #modal .primary");
  if (f) f.focus();
}
function closeModal() {
  if ($("#modal-bg").hidden) return;
  $("#modal-bg").hidden = true;
  stopSignInWait();
  if (modalFrom && modalFrom.isConnected) modalFrom.focus();
}

function setup() {
  const a = S.accounts;
  modal(`<h3>Set up LinkedIn</h3>
    <p>vaikhari posts through a LinkedIn app of your own: free, about two minutes, once.</p>
    <ol class="steps">
      <li><b class="k">1</b><div>Create an app at <a href="https://www.linkedin.com/developers/apps/new" target="_blank" rel="noopener noreferrer" style="color:var(--blue)">linkedin.com/developers</a>: any name, a LinkedIn page as its company (one you create works), a logo.</div></li>
      <li><b class="k">2</b><div>On its <b>Products</b> tab add <b>Share on LinkedIn</b> and <b>Sign In with LinkedIn using OpenID Connect</b>.</div></li>
      <li><b class="k">3</b><div>On its <b>Auth</b> tab add this redirect URL: <code>${esc(a.redirect)}</code> and copy the Client ID and Primary Client Secret here.</div></li>
    </ol>
    <label>Client ID</label><input type="text" id="cid" autocomplete="off">
    <label>Client Secret</label><input type="password" id="csecret" autocomplete="off">
    <div class="row"><button class="ghost" data-act="close">Cancel</button><button class="primary" data-act="setup-save">Save and sign in</button></div>`, "#cid");
}

async function setupSave() {
  try {
    await api("POST", "/api/auth/linkedin/setup", { client_id: $("#cid").value, client_secret: $("#csecret").value });
    await signIn();
  } catch (e) { toast(e.message, true); }
}

let signInWait = null;
function stopSignInWait() { clearInterval(signInWait); signInWait = null; }

async function signIn() {
  try {
    const r = await api("POST", "/api/auth/linkedin");
    modal(`<h3>Sign in to LinkedIn</h3><p>LinkedIn's sign-in lasts 60 days; vaikhari says when it runs low.</p>
      <div style="margin:16px 0"><a class="openx" href="${esc(r.url)}" target="_blank" rel="noopener noreferrer">${ICON.li} Open LinkedIn's sign-in ${ICON.ext}</a></div>
      <p style="font-size:13px">This window updates once you've signed in.</p>
      <div class="row"><button class="ghost" data-act="close">Close</button></div>`);
    const was = JSON.stringify(S.accounts.linkedin);
    const until = Date.now() + 5 * 60 * 1000;
    stopSignInWait();
    signInWait = setInterval(async () => {
      await refresh(true);
      const li = S.accounts.linkedin;
      if ((li.ok && JSON.stringify(li) !== was) || Date.now() > until) {
        stopSignInWait();
        if (li.ok) { closeModal(); toast("Signed in to LinkedIn as " + li.name); }
      }
    }, 2000);
  } catch (e) { toast(e.message, true); }
}

function settings() {
  const a = S.accounts;
  const li = a.linkedin;
  modal(`<h3>Settings</h3>
    <label>Your name on X (for the preview)</label><input type="text" id="s-xname" value="${esc(a.x.name)}">
    <label>Your X handle</label><input type="text" id="s-xhandle" value="@${esc(a.x.handle)}">
    <label>A new post goes to</label>
    <div class="nets" style="margin-top:4px"><button class="net ${a.networks.includes("x") ? "on" : ""}" data-snet="x">${ICON.x}X</button><button class="net ${a.networks.includes("linkedin") ? "on" : ""}" data-snet="linkedin">${ICON.li}LinkedIn</button></div>
    <label>LinkedIn</label>
    <div class="net-line">${ICON.li}<div style="flex:1">${li.ok ? `Signed in as <b>${esc(li.name)}</b>, ${li.days} days left` : li.has_app ? "Not signed in" : "Not set up yet"}</div>
      <button class="soft small" data-act="${li.has_app ? "signin" : "setup"}">${li.ok ? "Sign in again" : li.has_app ? "Sign in" : "Set up"}</button>${li.has_app ? '<button class="ghost small" data-act="setup">Change app</button>' : ""}</div>
    <div class="verse"><p lang="sa">चत्वारि वाक् परिमिता पदानि<br>तानि विदुर्ब्राह्मणा ये मनीषिणः ।<br>गुहा त्रीणि निहिता नेङ्गयन्ति<br>तुरीयं वाचो मनुष्या वदन्ति ॥</p>
      <small>Speech has four measures. Three lie hidden and do not stir; the fourth is what people speak. Ṛgveda 1.164.45. Later tradition calls that fourth stage <i>vaikharī</i> (वैखरी): speech spoken aloud.</small></div>
    <p style="font-size:12.5px">vaikhari ${esc(a.version)} · X is posted with one click on X's own page: free, no X account given to vaikhari.</p>
    <div class="row"><button class="ghost" data-act="close">Close</button><button class="primary" data-act="settings-save">Save</button></div>`);
}

async function settingsSave() {
  const nets = $$("[data-snet].on").map((b) => b.dataset.snet);
  try {
    await api("POST", "/api/settings", { x_name: $("#s-xname").value, x_handle: $("#s-xhandle").value, networks: nets });
    closeModal();
    toast("Saved");
  } catch (e) { toast(e.message, true); }
}

// ---------------------------------------------------------------- images

async function addImages(files) {
  if (C.approving) return toast("An agent's post goes with its own images: decline it and write your own to change them", true);
  for (const f of files) {
    if (!f.type.startsWith("image/")) continue;
    const fd = new FormData();
    fd.append("image", f, f.name || "pasted.png");
    try {
      const r = await api("POST", "/api/images", fd);
      C.images.push({ path: r.path, url: r.url });
    } catch (e) { toast(e.message, true); }
  }
  renderThumbs();
  renderPreviews();
  autosave();
}

// ---------------------------------------------------------------- wiring

function show(v) {
  view = v;
  $$("nav button").forEach((b) => b.classList.toggle("on", b.dataset.view === v));
  $("#view-compose").classList.toggle("on", v === "compose");
  $("#view-list").classList.toggle("on", v !== "compose");
  if (v === "compose") renderPreviews(); else renderList();
}

document.addEventListener("click", (e) => {
  const nav = e.target.closest("nav button");
  if (nav) { flush(); return show(nav.dataset.view); }
  const net = e.target.closest(".net[data-net]");
  if (net) {
    if (net.disabled) return;
    const n = net.dataset.net;
    C.networks = C.networks.includes(n) ? C.networks.filter((x) => x !== n) : [...C.networks, n];
    net.classList.toggle("on");
    renderPreviews();
    autosave();
    return;
  }
  const snet = e.target.closest("[data-snet]");
  if (snet) return snet.classList.toggle("on");
  const rm = e.target.closest("[data-rm]");
  if (rm) { C.images.splice(Number(rm.dataset.rm), 1); renderThumbs(); renderPreviews(); autosave(); return; }
  const a = e.target.closest("[data-act]");
  if (a) return act(a.dataset.act, a.dataset.id, a);
  if (e.target.id === "modal-bg") closeModal();
});

$("#text").addEventListener("input", (e) => { C.text = e.target.value; liOpen = false; measure(); autosave(); $("#saved").textContent = ""; });
$("#text").addEventListener("paste", (e) => {
  const imgs = [...(e.clipboardData || {}).files || []].filter((f) => f.type.startsWith("image/"));
  if (imgs.length) { e.preventDefault(); addImages(imgs); }
});
$(".editor").addEventListener("dragover", (e) => e.preventDefault());
$(".editor").addEventListener("drop", (e) => { e.preventDefault(); addImages([...e.dataTransfer.files]); });
$("#image-input").addEventListener("change", (e) => { addImages([...e.target.files]); e.target.value = ""; });
$("#when").addEventListener("input", whenChanged);
$("#go").addEventListener("click", go);
$("#save-draft").addEventListener("click", saveDraft);
$("#new-post").addEventListener("click", () => askLeave(async () => { await flush(); C = blank(); syncEditor(); $("#text").focus(); }));
$("#add-break").addEventListener("click", () => {
  const t = $("#text"), at = t.selectionStart, end = t.selectionEnd;
  const before = t.value.slice(0, at), after = t.value.slice(end);
  const mark = (before && !before.endsWith("\n") ? "\n" : "") + "---\n";
  t.value = before + mark + after;
  t.selectionStart = t.selectionEnd = at + mark.length;
  t.dispatchEvent(new Event("input"));
  t.focus();
});
// closing the tab mid-sentence keeps what's typed
window.addEventListener("beforeunload", (e) => { if (dirtyPost()) { e.preventDefault(); e.returnValue = ""; } });
window.addEventListener("pagehide", () => {
  if (!unsaved()) return;
  const b = body();
  b.when = "";
  fetch("/api/drafts", { method: "POST", keepalive: true, headers: { "X-Vaikhari-Token": TOKEN, "Content-Type": "application/json" }, body: JSON.stringify(b) }).catch(() => {});
});
$("#open-settings").addEventListener("click", settings);

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && !$("#modal-bg").hidden) return closeModal();
  if ((e.ctrlKey || e.metaKey) && e.key === "Enter" && view === "compose" && $("#modal-bg").hidden) { e.preventDefault(); go(); }
  if ((e.ctrlKey || e.metaKey) && e.key === "s" && view === "compose" && $("#modal-bg").hidden) { e.preventDefault(); if (!C.approving) saveDraft(); }
  if (e.key === "Enter" && !$("#modal-bg").hidden && e.target.tagName === "INPUT") {
    const primary = $("#modal .primary");
    if (primary) { e.preventDefault(); primary.click(); }
  }
});

(async () => {
  await loadTheme();
  await refresh(true);
  C = blank();
  syncEditor();
  $("#text").focus();
  setInterval(refresh, 2500);
  setInterval(loadTheme, 15000);
})();
