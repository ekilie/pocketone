//! Markdown to HTML with pulldown-cmark (CommonMark, plus tables, strikethrough and task lists).
//!
//!   stdin:  Markdown text (a string from the hook goes in as it is)
//!   stdout: the HTML
//!
//! Raw HTML in the Markdown is passed through, as CommonMark says: run untrusted input through sanitize-html after.
//!
//! Build: cargo build --release --target wasm32-wasip1 (rustup target add wasm32-wasip1 first);
//! the module is target/wasm32-wasip1/release/markdown.wasm.
use pulldown_cmark::{html, Options, Parser};
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
    let opts = Options::ENABLE_TABLES | Options::ENABLE_STRIKETHROUGH | Options::ENABLE_TASKLISTS;
    let mut out = String::with_capacity(input.len() * 3 / 2);
    html::push_html(&mut out, Parser::new_ext(&input, opts));
    std::io::stdout().write_all(out.as_bytes()).unwrap();
}
