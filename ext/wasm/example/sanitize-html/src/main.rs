//! Untrusted HTML to safe HTML with ammonia: an allow list of tags and attributes, links get rel="noopener noreferrer",
//! scripts, styles, event handlers and javascript: URLs are removed.
//!
//!   stdin:  HTML (a string from the hook goes in as it is)
//!   stdout: the cleaned HTML
//!
//! Build: cargo build --release --target wasm32-wasip1 (rustup target add wasm32-wasip1 first);
//! the module is target/wasm32-wasip1/release/sanitize-html.wasm.
use std::io::{Read, Write};

/// Ends the program with an error pb_hooks can read (err.code / err.message).
fn fail(code: &str, message: &str) -> ! {
    eprint!("{}", serde_json::json!({ "code": code, "message": message }));
    std::process::exit(1)
}

fn main() {
    match std::env::args().nth(1).as_deref() {
        Some("--describe") => {
            print!("{}", include_str!("../manifest.json"));
            return;
        }
        Some("run") => {}
        other => fail("no_such_function", other.unwrap_or("")),
    }
    let mut input = String::new();
    if let Err(e) = std::io::stdin().read_to_string(&mut input) {
        fail("bad_input", &format!("the input isn't UTF-8 text: {e}"));
    }
    let clean = ammonia::clean(&input);
    std::io::stdout().write_all(clean.as_bytes()).unwrap();
}
