<script server>
  const messages = ctx.store(`room_general`) || []
</script>

<%- include('chat.ets', { roomName: 'general', messages }) %>
