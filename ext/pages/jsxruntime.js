// The JSX runtime for .jsx/.tsx pages (SPEC 4.5): __h(tag, props, ...children) renders straight to an HTML string,
// wrapped as { __html } so it isn't escaped again when nested. Evaluated once per VM; returns the runtime object.
// An async component returns a promise; toHTML and h then return promises too, up to the page, which the driver
// awaits. Sync trees never make one, so they render as plain string concatenation.
;(function (esc) {
  var VOID = {
    area: 1, base: 1, br: 1, col: 1, embed: 1, hr: 1, img: 1, input: 1,
    link: 1, meta: 1, source: 1, track: 1, wbr: 1,
  }
  var RENAME = { className: 'class', htmlFor: 'for' }

  function Raw(s) {
    this.__html = s
  }
  Raw.prototype.toString = function () {
    return this.__html
  }
  function raw(s) {
    return new Raw(s === undefined || s === null ? '' : String(s))
  }

  // toHTML returns the HTML for v, or a promise of it if v is (or holds) a promise.
  function toHTML(v) {
    if (v === null || v === undefined || v === true || v === false) return ''
    if (v instanceof Raw) return v.__html
    if (Array.isArray(v)) {
      var out = ''
      for (var i = 0; i < v.length; i++) {
        var s = toHTML(v[i])
        if (typeof s !== 'string') return joinAsync(v, i, out, s)
        out += s
      }
      return out
    }
    if (typeof v === 'object') {
      if (typeof v.__html === 'string') return v.__html
      if (typeof v.then === 'function') return v.then(toHTML)
    }
    return esc(String(v))
  }

  // joinAsync finishes toHTML of the array v from index i, where out is the HTML so far and first is the promise
  // v[i] gave. Siblings are already running (JSX evaluates children first), so they're awaited together.
  function joinAsync(v, i, out, first) {
    var parts = [out, first]
    for (var j = i + 1; j < v.length; j++) parts.push(toHTML(v[j]))
    return Promise.all(parts).then(function (p) {
      return p.join('')
    })
  }

  function styleString(o) {
    var out = []
    for (var k in o) {
      var v = o[k]
      if (v === null || v === undefined || v === false) continue
      var name = k.indexOf('--') === 0 ? k : k.replace(/[A-Z]/g, function (c) { return '-' + c.toLowerCase() })
      out.push(name + ':' + v)
    }
    return out.join(';')
  }

  function Fragment(props) {
    var s = toHTML(props && props.children)
    return typeof s === 'string' ? raw(s) : s.then(raw)
  }

  function h(tag, props) {
    var children = []
    for (var i = 2; i < arguments.length; i++) children.push(arguments[i])

    if (typeof tag === 'function') {
      var p = {}
      for (var k in props) p[k] = props[k]
      if (children.length) p.children = children.length === 1 ? children[0] : children
      return tag(p)
    }

    var attrs = ''
    var inner = null
    for (var name in props) {
      var v = props[name]
      if (name === 'children' || name === 'key' || name === 'ref') continue
      if (name === 'dangerouslySetInnerHTML') {
        inner = v && v.__html !== undefined ? String(v.__html) : ''
        continue
      }
      if (v === null || v === undefined || v === false || typeof v === 'function') continue
      var attr = RENAME[name] || name
      if (v === true) {
        attrs += ' ' + attr
        continue
      }
      if (attr === 'style' && typeof v === 'object') v = styleString(v)
      attrs += ' ' + attr + '="' + esc(String(v)) + '"'
    }
    if (VOID[tag]) return raw('<' + tag + attrs + '>')
    if (inner === null) inner = toHTML(children.length ? children : props && props.children)
    // JSX can't spell a doctype, so <html> brings its own
    var doctype = tag === 'html' ? '<!doctype html>' : ''
    if (typeof inner !== 'string') {
      return inner.then(function (s) {
        return raw(doctype + '<' + tag + attrs + '>' + s + '</' + tag + '>')
      })
    }
    return raw(doctype + '<' + tag + attrs + '>' + inner + '</' + tag + '>')
  }

  return { h: h, Fragment: Fragment, raw: raw, toHTML: toHTML }
})
