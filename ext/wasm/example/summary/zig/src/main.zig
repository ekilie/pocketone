//! The summary module in Zig: the same input, output and errors as ../go (Go), a plain program too.
//!
//!   stdin:  {"title": "...", "body": "..."}
//!   stdout: {"text": "<title>: <first 12 words> …", "words": <word count>}
//!
//! Build: zig build -Doptimize=ReleaseSmall (Zig 0.17); the module is zig-out/bin/summary.wasm.
const std = @import("std");

const Post = struct { title: []const u8 = "", body: []const u8 = "" };

/// Ends the program with an error pb_hooks can read (err.code / err.message).
fn fail(io: std.Io, code: []const u8, message: []const u8) noreturn {
    var buf: [256]u8 = undefined;
    var w = std.Io.File.stderr().writerStreaming(io, &buf);
    w.interface.print("{{\"code\":\"{s}\",\"message\":\"{s}\"}}", .{ code, message }) catch {};
    w.interface.flush() catch {};
    std.process.exit(1);
}

pub fn main(init: std.process.Init) !void {
    const io = init.io;
    const allocator = init.arena.allocator();
    var out_buf: [4096]u8 = undefined;
    var stdout = std.Io.File.stdout().writerStreaming(io, &out_buf);
    const out = &stdout.interface;

    // the function asked for: --describe prints the manifest (manifest.json beside the source, which build.zig makes
    // importable); "run" is the only other
    const args = try init.minimal.args.toSlice(allocator);
    const function: []const u8 = if (args.len > 1) args[1] else "";
    if (std.mem.eql(u8, function, "--describe")) {
        try out.writeAll(@embedFile("manifest.json"));
        return out.flush();
    }
    if (!std.mem.eql(u8, function, "run")) fail(io, "no_such_function", function);

    var in_buf: [4096]u8 = undefined;
    var stdin = std.Io.File.stdin().readerStreaming(io, &in_buf);
    const input = try stdin.interface.allocRemaining(allocator, .unlimited);
    const post = std.json.parseFromSliceLeaky(Post, allocator, input, .{ .ignore_unknown_fields = true }) catch
        fail(io, "bad_input", "the input isn't a post");

    var words: std.ArrayList([]const u8) = .empty;
    var it = std.mem.tokenizeAny(u8, post.body, " \t\r\n");
    while (it.next()) |w| try words.append(allocator, w);
    if (words.items.len == 0) fail(io, "empty_body", "the post has no body to summarize");

    var text: std.ArrayList(u8) = .empty;
    if (post.title.len > 0) {
        try text.appendSlice(allocator, post.title);
        try text.appendSlice(allocator, ": ");
    }
    for (words.items[0..@min(words.items.len, 12)], 0..) |w, i| {
        if (i > 0) try text.append(allocator, ' ');
        try text.appendSlice(allocator, w);
    }
    if (words.items.len > 12) try text.appendSlice(allocator, " …");

    try std.json.Stringify.value(.{ .text = text.items, .words = words.items.len }, .{}, out);
    try out.flush();
}
