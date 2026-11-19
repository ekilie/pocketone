<%_ if (ctx.req.method === 'POST') {
  ctx.realtime.send('chat', include('chat-msg.ets', { message: data.message }))
} _%>
