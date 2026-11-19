<% slot('title', () => { %>Error <%= ctx.error.status %><% }) %>
<h1><%= ctx.error.status %></h1>
<p><%= ctx.error.message %></p>
<p class="muted">This is <code>+error.ets</code>. <a href="/">Home</a></p>
