//! CSV to JSON rows with the csv crate: quoted fields, commas and newlines inside quotes, a BOM, CRLF line ends.
//!
//!   stdin:  the CSV file's bytes or text, first line the column names
//!   stdout: {"columns": ["name", ...], "rows": [{"name": "...", ...}, ...], "errors": [{"line": 7, "message": "..."}]}
//!
//! Functions: "run" reads comma-separated, "tsv" tab-separated, "semicolon" semicolon-separated (spreadsheets in
//! many European locales). Every value is a string, as in the file; a row with the wrong number of fields goes to
//! errors and the others are kept.
//!
//! Build: cargo build --release --target wasm32-wasip1 (rustup target add wasm32-wasip1 first);
//! the module is target/wasm32-wasip1/release/csv.wasm.
use serde_json::{json, Map, Value};
use std::io::Read;

/// Ends the program with an error pb_hooks can read (err.code / err.message).
fn fail(code: &str, message: &str) -> ! {
    eprint!("{}", json!({ "code": code, "message": message }));
    std::process::exit(1)
}

fn main() {
    let delimiter = match std::env::args().nth(1).as_deref() {
        Some("--describe") => {
            print!("{}", include_str!("../manifest.json"));
            return;
        }
        Some("run") => b',',
        Some("tsv") => b'\t',
        Some("semicolon") => b';',
        other => fail("no_such_function", other.unwrap_or("")),
    };
    let mut input = Vec::new();
    std::io::stdin().read_to_end(&mut input).unwrap();
    let data = input.strip_prefix(b"\xEF\xBB\xBF").unwrap_or(&input); // Excel's UTF-8 BOM
    let mut reader = csv::ReaderBuilder::new().delimiter(delimiter).from_reader(data);
    let columns: Vec<String> = match reader.headers() {
        Ok(h) => h.iter().map(|c| c.trim().to_string()).collect(),
        Err(e) => fail("bad_input", &format!("no header line: {e}")),
    };
    if columns.is_empty() || columns.iter().all(String::is_empty) {
        fail("bad_input", "no header line");
    }
    let (mut rows, mut errors) = (Vec::new(), Vec::new());
    for result in reader.records() {
        match result {
            Ok(record) => {
                let row: Map<String, Value> =
                    columns.iter().zip(record.iter()).map(|(c, v)| (c.clone(), Value::from(v))).collect();
                rows.push(Value::Object(row));
            }
            Err(e) => {
                let line = e.position().map(|p| p.line()).unwrap_or(0);
                errors.push(json!({ "line": line, "message": e.to_string() }));
            }
        }
    }
    serde_json::to_writer(std::io::stdout(), &json!({ "columns": columns, "rows": rows, "errors": errors })).unwrap();
}
