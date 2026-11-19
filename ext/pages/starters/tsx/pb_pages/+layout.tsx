// The root layout. Every page renders inside it; `children` is the page's HTML. <html> brings its own doctype.

const css = `
  body { margin: 0; font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; }
  main { max-width: 56rem; margin: 0 auto; padding: 4rem 1.5rem; }
  h1 { font-size: 3rem; margin: 0 0 .5rem; }
  .lead { font-size: 1.25rem; color: #94a3b8; margin: 0 0 3rem; }
  .grid { display: grid; gap: 1.25rem; grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr)); }
  .card { padding: 1.5rem; border: 1px solid #334155; border-radius: .75rem; background: #1e293b; }
  .card h3 { margin: .5rem 0; }
  .card p { margin: 0; color: #94a3b8; }
  .icon { font-size: 2rem; }
  footer { margin-top: 3rem; color: #64748b; font-size: .875rem; }
  a { color: #38bdf8; }
`

export default function Layout({ children, slot }: pages.Props) {
  return (
    <html lang="en">
      <head>
        <meta charset="UTF-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
        <title>{slot('title') || 'PocketPages'}</title>
        <style dangerouslySetInnerHTML={{ __html: css }} />
      </head>
      <body>
        <main>{children}</main>
      </body>
    </html>
  )
}
