/// <reference path="../pb_data/types.d.ts" />

// A tool from JS: write the REST endpoint, then the tool wrapper ($mcp.addEndpoint at the top level).

routerAdd(
  'GET',
  '/api/example/recent-users',
  (e) => {
    const limit = Math.min(100, Math.max(1, parseInt(e.request.url.query().get('limit')) || 10))
    const rows = $app.findRecordsByFilter('users', '', '-created', limit, 0)
    return e.json(200, { users: rows.map((r) => ({ email: r.email(), created: r.getString('created') })) })
  },
  $apis.requireSuperuserAuth()
)

$mcp.addEndpoint({
  name: 'example_list_recent_users',
  title: 'Recent users',
  description: 'The newest users (email and created date). Superusers only.',
  method: 'GET',
  path: '/api/example/recent-users', // other args (limit) go to the query string
  inputSchema: {
    type: 'object',
    properties: { limit: { type: 'integer', minimum: 1, maximum: 100 } },
    additionalProperties: false,
  },
  // optional: reshape the result for the model
  response: ({ status, json }) => {
    if (status !== 200) throw new Error(json?.message || 'HTTP ' + status)
    return { count: json.users.length, users: json.users }
  },
})

$mcp.instructions('example_list_recent_users lists the newest app users.')
