<% slot('title', () => { %>Not found<% }) %>
<h1>404</h1>
<p>Nothing at <code><%= ctx.req.url.pathname %></code>. This is <code>+404.ets</code>, inside the root layout.</p>
<p><a href="/">Home</a></p>
