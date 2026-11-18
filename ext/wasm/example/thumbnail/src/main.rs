//! A thumbnail of a PNG or JPEG image with the image crate: scaled to fit a square, kept in proportion, as a PNG.
//!
//!   stdin:  the image's bytes (a Uint8Array or ArrayBuffer from the hook), or the same bytes as base64 text
//!   stdout: {"width": <px>, "height": <px>, "png": "<base64>"}
//!
//! Functions: "run" fits the image in 256×256, "small" in 64×64. An image already smaller is only re-encoded.
//! The decoded image must fit in the call's memory (MaxCallMemoryMB, 32 MB by default): about 2.5 megapixels
//! (1920×1280) with the defaults. Bigger photos crash the call with "crashed"; raise MaxCallMemoryMB for them.
//!
//! Build: cargo build --release --target wasm32-wasip1 (rustup target add wasm32-wasip1 first);
//! the module is target/wasm32-wasip1/release/thumbnail.wasm.
use base64::{engine::general_purpose::STANDARD, Engine};
use image::ImageFormat;
use std::io::{Cursor, Read};

/// Ends the program with an error pb_hooks can read (err.code / err.message).
fn fail(code: &str, message: &str) -> ! {
    eprint!("{}", serde_json::json!({ "code": code, "message": message }));
    std::process::exit(1)
}

fn main() {
    let size = match std::env::args().nth(1).as_deref() {
        Some("--describe") => {
            print!("{}", include_str!("../manifest.json"));
            return;
        }
        Some("run") => 256,
        Some("small") => 64,
        other => fail("no_such_function", other.unwrap_or("")),
    };
    let mut input = Vec::new();
    std::io::stdin().read_to_end(&mut input).unwrap();
    if input.is_empty() {
        fail("empty_input", "no image");
    }
    // raw bytes, or base64 text (what a JSON string can carry: the manifest's examples)
    if image::guess_format(&input).is_err() {
        let text: Vec<u8> = input.iter().copied().filter(|b| !b.is_ascii_whitespace()).collect();
        if let Ok(decoded) = STANDARD.decode(&text) {
            input = decoded;
        }
    }
    let img = image::load_from_memory(&input)
        .unwrap_or_else(|e| fail("bad_image", &format!("not a PNG or JPEG image: {e}")));
    drop(input);
    let thumb = if img.width() > size || img.height() > size {
        img.thumbnail(size, size) // fast area sampling, kept in proportion
    } else {
        img
    };
    let mut png = Vec::new();
    thumb
        .write_to(&mut Cursor::new(&mut png), ImageFormat::Png)
        .unwrap_or_else(|e| fail("encode_failed", &e.to_string()));
    let out = serde_json::json!({ "width": thumb.width(), "height": thumb.height(), "png": STANDARD.encode(&png) });
    serde_json::to_writer(std::io::stdout(), &out).unwrap();
}
