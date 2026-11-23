<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title><%= ctx.meta('title') || 'Ask the docs' %></title>
  <link rel="stylesheet" href="https://unpkg.com/mvp.css@1.17.2/mvp.css" />
</head>
<body>
  <header>
    <nav>
      <a href="/"><b>Ask the docs</b></a>
      <ul>
        <li><a href="/">Ask</a></li>
        <li><a href="/docs">Docs</a></li>
      </ul>
    </nav>
  </header>
  <main><%- slot() %></main>
</body>
</html>
