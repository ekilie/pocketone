<%
// The front page: ask for a story, or pick one already told.
const { DEFAULTS, MAX } = ctx.resolve('story')
const told = ctx.app.findRecordsByFilter('stories', "status = 'done'", '-created', 24, 0)
const ideas = [
  'A little owl who is afraid of the dark learns the night is full of friends',
  'A sleepy dragon who wants to see the snow',
  'Two otters drifting down a moonlit river, holding hands',
  'A teddy bear who goes looking for the moon',
]
%>
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Bedtime Story</title>
  <style>
    :root { color-scheme: dark; --bg: #0b1020; --ink: #e9e4d8; --dim: #a39d8f; --line: #232a44; --glow: #f2c46d; }
    * { box-sizing: border-box; }
    body { margin: 0; min-height: 100vh; background: radial-gradient(ellipse at top, #1a2140 0%, var(--bg) 60%); color: var(--ink);
      font: 17px/1.55 Georgia, 'Iowan Old Style', serif; }
    main { max-width: 680px; margin: 0 auto; padding: 56px 16px 80px; }
    h1 { font-weight: normal; font-size: 40px; margin: 0 0 6px; letter-spacing: .01em; }
    h1 span { color: var(--glow); }
    .sub { color: var(--dim); margin: 0 0 32px; }
    textarea { width: 100%; min-height: 96px; padding: 14px 16px; border-radius: 12px; border: 1px solid var(--line);
      background: #0f1530; color: var(--ink); font: inherit; resize: vertical; }
    textarea:focus { outline: 2px solid #3b4675; }
    label { color: var(--dim); font-size: 15px; }
    label input { width: 64px; margin-left: 6px; padding: 6px 8px; border-radius: 8px; border: 1px solid var(--line); background: #0f1530;
      color: var(--ink); font: inherit; }
    .row { display: flex; gap: 12px; align-items: center; margin-top: 12px; flex-wrap: wrap; }
    button { padding: 12px 22px; border: 0; border-radius: 999px; background: var(--glow); color: #1b1606; font: inherit;
      cursor: pointer; }
    button:disabled { opacity: .5; cursor: default; }
    .note { color: var(--dim); font-size: 14px; }
    .ideas { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 18px; }
    .ideas a { color: var(--dim); font-size: 14px; border: 1px solid var(--line); border-radius: 999px; padding: 4px 12px;
      text-decoration: none; cursor: pointer; }
    .ideas a:hover { color: var(--ink); border-color: #3b4675; }
    .err { color: #ff9b8a; }
    h2 { font-weight: normal; font-size: 20px; margin: 56px 0 12px; color: var(--dim); }
    ul.told { list-style: none; padding: 0; margin: 0; display: grid; gap: 12px; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); }
    ul.told a { display: block; color: var(--ink); text-decoration: none; border-radius: 10px; overflow: hidden; background: #0f1530;
      border: 1px solid var(--line); }
    ul.told img { display: block; width: 100%; aspect-ratio: 16 / 9; object-fit: cover; opacity: .85; }
    ul.told span { display: block; padding: 8px 10px; font-size: 15px; }
  </style>
</head>
<body>
<main>
  <h1>Bedtime <span>Story</span></h1>
  <p class="sub">Tell me what tonight's story is about. A minute or two later it's ready, read aloud, with pictures and a quiet song.</p>

  <form id="ask">
    <textarea id="prompt" maxlength="500" placeholder="A little fox who counts the stars..." required></textarea>
    <div class="row">
      <label>Minutes <input id="minutes" type="number" min="1" max="<%= MAX.minutes %>" value="<%= DEFAULTS.minutes %>"></label>
      <label>Pictures <input id="pictures" type="number" min="1" max="<%= MAX.pictures %>" value="<%= DEFAULTS.pictures %>"></label>
    </div>
    <div class="row">
      <button id="go">Tell me a story</button>
      <span class="note" id="msg">A few cents a minute in model costs.</span>
    </div>
  </form>
  <div class="ideas">
    <% for (const idea of ideas) { %><a data-idea="<%= idea %>"><%= idea %></a><% } %>
  </div>

  <% if (told.length) { %>
  <h2>Stories told</h2>
  <ul class="told">
    <% for (const s of told) {
      const pics = s.get('pictures') || [] %>
    <li><a href="/story/<%= s.id %>">
      <% if (pics.length) { %><img src="/api/files/stories/<%= s.id %>/<%= pics[0] %>?thumb=480x270" alt="" loading="lazy"><% } %>
      <span><%= s.getString('title') || s.getString('prompt') %></span>
    </a></li>
    <% } %>
  </ul>
  <% } %>
</main>
<script>
  const form = document.getElementById('ask'), prompt = document.getElementById('prompt'),
    go = document.getElementById('go'), msg = document.getElementById('msg')
  document.querySelectorAll('[data-idea]').forEach((a) => a.addEventListener('click', () => { prompt.value = a.dataset.idea; prompt.focus() }))
  form.addEventListener('submit', async (e) => {
    e.preventDefault()
    go.disabled = true
    msg.className = 'note'
    msg.textContent = 'Starting...'
    try {
      const res = await fetch('/story', { method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prompt: prompt.value, minutes: document.getElementById('minutes').value,
          pictures: document.getElementById('pictures').value }) })
      const body = await res.json()
      if (!res.ok) throw new Error(body.message || res.statusText)
      location.href = '/story/' + body.id
    } catch (err) {
      msg.className = 'note err'
      msg.textContent = err.message
      go.disabled = false
    }
  })
</script>
</body>
</html>
