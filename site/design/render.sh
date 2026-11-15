#!/bin/bash
# Render logo.html to PNGs with headless Chrome, and make the site's favicons from mark.svg.
set -euo pipefail
cd "$(dirname "$0")"
CH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
shot() { # name query scale
  echo "$(date +%T) rendering $1"
  timeout 60 "$CH" --headless=new --disable-gpu --hide-scrollbars --force-device-scale-factor="$3" \
    --window-size=1200,1200 --default-background-color=00000000 --virtual-time-budget=8000 \
    --allow-file-access-from-files --screenshot="$PWD/out/$1.png" "file://$PWD/logo.html?$2" >/dev/null 2>&1
}
mkdir -p out
shot lockup-dark "v=lockup&bg=dark" 2
shot lockup-light "v=lockup&bg=light" 2
shot lockup-none "v=lockup&bg=none" 2
shot mark "v=mark&bg=none" 1
P=../pb_pages
cp mark.svg "$P/logo.svg"
for s in 16 32 180 192 512; do sips -z $s $s out/mark.png --out "out/mark-$s.png" >/dev/null; done
cp out/mark-16.png "$P/favicon-16x16.png"; cp out/mark-32.png "$P/favicon-32x32.png"
cp out/mark-180.png "$P/apple-touch-icon.png"; cp out/mark-192.png "$P/android-chrome-192x192.png"; cp out/mark-512.png "$P/android-chrome-512x512.png"
echo "$(date +%T) done"
