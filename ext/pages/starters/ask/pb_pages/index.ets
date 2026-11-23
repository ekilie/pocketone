<%
  // GET /?q=...: the answer is written on the server, so the page works without JavaScript and a question is a link
  const q = (ctx.req.query('q') || '').trim()
  const result = q ? await ctx.resolve('ask').answer(ctx, q) : null
%>
<section>
  <form method="get" action="/">
    <label for="q">Ask a question about the docs</label>
    <input id="q" name="q" value="<%= q %>" placeholder="How do I get started?" maxlength="300" autofocus />
    <button type="submit">Ask</button>
  </form>
</section>

<% if (result) { %>
<section>
  <article>
    <% if (result.error) { %>
      <p><mark><%= result.error %></mark></p>
    <% } else if (!result.text) { %>
      <p>Nothing in the docs matches that question.</p>
    <% } else { %>
      <p style="white-space: pre-wrap"><%= result.text %></p>
    <% } %>
    <% if (result.sources.length) { %>
      <h3>Sources</h3>
      <ol>
        <% for (const s of result.sources) { %>
          <li><a href="<%= s.url %>"><%= s.title %><% if (s.heading) { %> › <%= s.heading %><% } %></a></li>
        <% } %>
      </ol>
    <% } %>
  </article>
</section>
<% } %>
