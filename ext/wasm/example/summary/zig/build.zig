const std = @import("std");

// zig build -Doptimize=ReleaseSmall  ->  zig-out/bin/summary.wasm (a WASI program)
pub fn build(b: *std.Build) void {
    const optimize = b.standardOptimizeOption(.{});
    const target = b.standardTargetOptions(.{
        .default_target = .{ .os_tag = .wasi, .cpu_arch = .wasm32 },
    });
    const mod = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
    });
    mod.addAnonymousImport("manifest.json", .{ .root_source_file = b.path("manifest.json") });
    const exe = b.addExecutable(.{ .name = "summary", .root_module = mod });
    // Zig's default stack is 16 MB: every call would start with 16 MB of memory. 1 MB is plenty here.
    exe.stack_size = 1 << 20;
    b.installArtifact(exe);
}
