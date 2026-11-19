<%
// The root layout, in ETS. Every page renders inside it (ETS, Markdown and TSX pages alike).
const flash = ctx.flash()
const nav: [string, string][] = [
  ['/', 'Home'],
  ['/jsx', 'JSX/TSX'],
  ['/docs', 'Markdown'],
  ['/notes', 'Notes'],
  ['/clock', 'SSE'],
  ['/stats', 'JSON'],
  ['/speed', 'Speed'],
]
%><!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title><%- slot('title') || 'PocketPages demo' %></title>
    <link rel="stylesheet" href="<%= asset('assets/app.css') %>" />
    <%- slot('head') %>
  </head>
  <body>
    <header>
      <a class="brand" href="/">PocketPages</a>
      <nav>
        <% for (const [href, label] of nav) { %>
          <a href="<%= href %>"<% if (ctx.req.url.pathname === href) { %> class="on"<% } %>><%= label %></a>
        <% } %>
      </nav>
      <div class="who">
        <% if (ctx.auth) { %>
          <form method="post" action="/auth/logout">
            <span><%= ctx.auth.email() %></span> <button>Sign out</button>
          </form>
        <% } else { %>
          <a href="/auth/login">Sign in</a>
        <% } %>
      </div>
    </header>
    <% if (flash) { %><p class="flash"><%= String(flash) %></p><% } %>
    <main><%- slot() %></main>
    <footer>
      Rendered by <code>ext/core/pages</code> in PocketOne · page file <code><%= ctx.route.file %></code>
    </footer>
  </body>
</html>
