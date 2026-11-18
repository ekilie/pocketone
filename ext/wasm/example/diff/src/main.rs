//! A line diff of two texts with similar: a unified diff and how many lines were added and removed.
//!
//!   stdin:  {"old": "...", "new": "...", "context": 3}   (context: unchanged lines around each change, default 3)
//!   stdout: {"unified": "@@ -1,2 +1,2 @@\n...", "added": <lines>, "removed": <lines>, "changed": <bool>}
//!
//! Build: cargo build --release --target wasm32-wasip1 (rustup target add wasm32-wasip1 first);
//! the module is target/wasm32-wasip1/release/diff.wasm.
use serde::{Deserialize, Serialize};
use similar::{ChangeTag, TextDiff};

#[derive(Deserialize)]
struct Input {
    #[serde(default)]
    old: String,
    #[serde(default)]
    new: String,
    context: Option<usize>,
}

#[derive(Serialize)]
struct Output {
    unified: String,
    added: usize,
    removed: usize,
    changed: bool,
}

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
    let input: Input = serde_json::from_reader(std::io::stdin())
        .unwrap_or_else(|e| fail("bad_input", &format!("want {{\"old\", \"new\"}}: {e}")));
    let diff = TextDiff::from_lines(&input.old, &input.new);
    let (mut added, mut removed) = (0, 0);
    for change in diff.iter_all_changes() {
        match change.tag() {
            ChangeTag::Insert => added += 1,
            ChangeTag::Delete => removed += 1,
            ChangeTag::Equal => {}
        }
    }
    let unified = diff
        .unified_diff()
        .context_radius(input.context.unwrap_or(3))
        .header("old", "new")
        .to_string();
    let out = Output { unified, added, removed, changed: added + removed > 0 };
    serde_json::to_writer(std::io::stdout(), &out).unwrap();
}
