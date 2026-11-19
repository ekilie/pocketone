<script server>
  const { roomName } = params
  const messages = ctx.store(`room_${roomName}`) || []
</script>

<%- include('chat.ets', { roomName, messages }) %>
