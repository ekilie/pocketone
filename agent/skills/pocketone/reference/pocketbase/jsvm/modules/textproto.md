Source: https://pocketbase.io/jsvm/modules/textproto.html

# Namespace textproto

Package textproto implements generic support for text-based request/response protocols in the style of HTTP, NNTP, and SMTP.

This package enforces the HTTP/1.1 character set defined by RFC 9112 for header keys and values.

The package provides:

[Error], which represents a numeric error response from a server.

[Pipeline], to manage pipelined requests and responses in a client.

[Reader], to read numeric response code lines, key: value headers, lines wrapped with leading spaces on continuation lines, and whole text blocks ending with a dot on a line by itself.

[Writer], to write dot-encoded text blocks.

[Conn], a convenient packaging of [Reader], [Writer], and [Pipeline] for use with a single network connection.
