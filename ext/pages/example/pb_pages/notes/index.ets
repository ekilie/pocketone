<%
const { plural, when } = require('../_private/format')
type Note = { id: string; title: string; body: string; created: string; expand?: { author?: { name?: string; email?: string } } }
const notes: Note[] = data.notes
%><% slot('title', () => { %>Notes<% }) %>
<h1>Notes</h1>
<p class="muted">
  <%= plural(notes.length, 'note') %>, read with <code>ctx.pb()</code> as
  <%= ctx.auth ? ctx.auth.email() : 'a guest' %>.
</p>

<% if (ctx.auth) { %>
  <form method="post" class="stack card" style="margin-bottom: 1.25rem">
    <input name="title" placeholder="Title" required maxlength="120" />
    <textarea name="body" rows="3" placeholder="Say something"></textarea>
    <button>Post note</button>
  </form>
<% } else { %>
  <p><a href="/auth/login">Sign in</a> to post (user@example.com / password).</p>
<% } %>

<div class="grid">
  <% for (const n of notes) { %>
    <a class="card" href="/notes/<%= n.id %>">
      <h3><%= n.title %></h3>
      <div class="muted"><%= n.body %></div>
      <div class="badge" style="margin-top: .5rem">
        <%= n.expand?.author?.name || n.expand?.author?.email || 'someone' %> · <%= when(n.created) %>
      </div>
    </a>
  <% } %>
</div>
