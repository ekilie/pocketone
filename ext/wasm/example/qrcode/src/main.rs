//! A QR code as SVG, with the qrcode crate.
//!
//!   stdin:  the text to encode (a string from the hook goes in as it is): a URL, a ticket code, a Wi-Fi string
//!   stdout: an <svg> image, at least 200 px wide, black on white
//!
//! Build: cargo build --release --target wasm32-wasip1 (rustup target add wasm32-wasip1 first);
//! the module is target/wasm32-wasip1/release/qrcode.wasm.
use qrcode::render::svg;
use qrcode::{EcLevel, QrCode};
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
    let mut input = Vec::new();
    std::io::stdin().read_to_end(&mut input).unwrap();
    if input.is_empty() {
        fail("empty_input", "nothing to encode");
    }
    let code = QrCode::with_error_correction_level(&input, EcLevel::M)
        .unwrap_or_else(|e| fail("too_long", &format!("can't encode it: {e}")));
    let image = code.render::<svg::Color>().min_dimensions(200, 200).build();
    std::io::stdout().write_all(image.as_bytes()).unwrap();
}
