<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title><%= ctx.meta('title') || 'Ask the docs' %></title>
  <link rel="stylesheet" href="https://unpkg.com/mvp.css@1.17.2/mvp.css" />
  <style>
    header { padding-bottom: 0; }
    header nav { margin-bottom: 1rem; }
    main { padding-top: 1rem; }
    form { min-width: min(40rem, 100%); }
    form input { width: 100%; box-sizing: border-box; }
  </style>
</head>
<body>
  <header>
    <nav>
      <a href="/">Ask the docs</a>
      <ul>
        <li><a href="/">Ask</a></li>
        <li><a href="/docs">Docs</a></li>
      </ul>
    </nav>
  </header>
  <main><%- slot() %></main>
</body>
</html>
