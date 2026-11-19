<% slot('title', () => { %><%= data.note.title %><% }) %>
<p><a href="/notes">← All notes</a></p>
<h1><%= data.note.title %></h1>
<p><%= data.note.body %></p>
<p class="muted">id <code><%= params.id %></code>, created <%= data.note.created %></p>
<p class="muted">This page is ETS (<code>notes/[id].ets</code>) sharing <code>notes/+page.ts</code> with the list. Try <a href="/notes/missing">a missing id</a>.</p>
