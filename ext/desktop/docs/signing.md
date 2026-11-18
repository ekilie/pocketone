---
title: Signing and sharing
description: "Sign your PocketOne desktop app so people can install it without warnings: Developer ID signing and notarization on macOS, Authenticode on Windows, checksums and signatures on Linux."
---

# Signing and sharing

`pocketone desktop build` gives you apps that run on the machine that built them. Before you send them to other people,
sign them, so each operating system knows the app comes from you and hasn't been changed since:

| Platform | Unsigned, your users see | To sign, you need |
|---|---|---|
| macOS | "can't be opened", until they approve it in System Settings | An Apple Developer account and a Mac |
| Windows | A SmartScreen "Windows protected your PC" warning | A code-signing certificate |
| Linux | Nothing; Linux doesn't check | Optional: a GPG key, to publish a signature |

Sign after every build: signing applies to the exact file, so a rebuilt app needs signing again.

## macOS

macOS wants two things for an app downloaded from the internet: a **Developer ID signature**, and **notarization**,
where Apple scans the app and records that it did. You do both on a Mac, with the tools that come with Xcode.

### Once: get a Developer ID certificate

1. Join the [Apple Developer Program](https://developer.apple.com/programs/) (paid, yearly).
2. In Xcode, open **Settings → Accounts**, add your Apple ID, select your team and choose **Manage Certificates**.
   Click **+** and pick **Developer ID Application**. Xcode creates the certificate and puts it in your keychain.
3. Check that it's there and copy its name:

   ```sh
   security find-identity -v -p codesigning
   #  1) 3F2A…  "Developer ID Application: Your Name (ABCDE12345)"
   ```

   `ABCDE12345` is your team ID.

4. Create an [app-specific password](https://account.apple.com) for your Apple ID (**Sign-In and Security →
   App-Specific Passwords**), and save it for the notary tool under a profile name:

   ```sh
   xcrun notarytool store-credentials pocketone-notary \
     --apple-id you@example.com --team-id ABCDE12345 --password <app-specific password>
   ```

### Every release: sign, notarize, staple

Do this for each macOS build (`dist/darwin-arm64/Hello.app`, `dist/darwin-amd64/Hello.app`):

1. **Sign** with your Developer ID, the hardened runtime (`--options runtime`, which notarization requires) and a
   secure timestamp. `--force` replaces the ad hoc signature the build added:

   ```sh
   codesign --force --options runtime --timestamp \
     --sign "Developer ID Application: Your Name (ABCDE12345)" dist/darwin-arm64/Hello.app
   codesign --verify --strict --verbose=2 dist/darwin-arm64/Hello.app
   ```

2. **Notarize:** zip the app and send it to Apple. It usually takes a few minutes:

   ```sh
   ditto -c -k --keepParent dist/darwin-arm64/Hello.app Hello.zip
   xcrun notarytool submit Hello.zip --keychain-profile pocketone-notary --wait
   ```

   If it says `Invalid`, `xcrun notarytool log <submission id> --keychain-profile pocketone-notary` shows why.

3. **Staple** the result to the app, so it opens even when the user is offline, and check it:

   ```sh
   xcrun stapler staple dist/darwin-arm64/Hello.app
   spctl --assess --type execute --verbose dist/darwin-arm64/Hello.app
   # dist/darwin-arm64/Hello.app: accepted
   # source=Notarized Developer ID
   ```

4. **Package** the stapled app to share it. A zip:

   ```sh
   ditto -c -k --keepParent dist/darwin-arm64/Hello.app Hello-macos-arm64.zip
   ```

   or a disk image, which you sign too:

   ```sh
   hdiutil create -volname Hello -srcfolder dist/darwin-arm64/Hello.app -ov -format UDZO Hello.dmg
   codesign --timestamp --sign "Developer ID Application: Your Name (ABCDE12345)" Hello.dmg
   ```

One download for both kinds of Mac: join the two executables with `lipo` before you sign, and ship one app.

```sh
cp -R dist/darwin-arm64/Hello.app Hello.app
lipo -create dist/darwin-arm64/Hello.app/Contents/MacOS/Hello dist/darwin-amd64/Hello.app/Contents/MacOS/Hello \
  -output Hello.app/Contents/MacOS/Hello
```

## Windows

Windows checks an **Authenticode** signature. You need a code-signing certificate from a certificate authority, such
as DigiCert, Sectigo or SSL.com. Certificates issued today keep their key on a hardware token or in the CA's cloud
signing service rather than in a file, so how you sign depends on which you get.

### Once: get a certificate

- **Organization validation (OV):** the CA checks that your business exists. For an individual, pick a certificate
  issued to you by name.
- **Extended validation (EV):** a stricter check, for businesses.
- **A cloud signing service**, such as Azure Trusted Signing, signs without a token, for organizations it can verify.

Either way, a new certificate starts with no reputation: SmartScreen may still warn for a while, until enough people
have installed your signed app.

### Every release: sign with signtool

On Windows, with the Windows SDK installed (it includes `signtool`) and your certificate on its token or in the
certificate store:

```bat
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 /a dist\windows-amd64\Hello.exe
signtool verify /pa /v dist\windows-amd64\Hello.exe
```

`/a` picks the best certificate it finds; `/sha1 <thumbprint>` picks one exactly. A cloud signing service gives you
its own `signtool` plugin or tool; follow its instructions for the same step. Sign `windows-arm64` the same way.

### From a Mac or Linux

[osslsigncode](https://github.com/mtrojnar/osslsigncode) signs Windows executables on macOS and Linux, if your
certificate's key is in a file or on a token it can reach:

```sh
osslsigncode sign -pkcs12 certificate.pfx -pass "$PFX_PASSWORD" -n "Hello" \
  -t http://timestamp.digicert.com -in dist/windows-amd64/Hello.exe -out Hello.exe
```

## Linux

Linux runs the binary you give it; there's no signature check to pass. What you can do is let people check the
download is yours and intact:

```sh
cd dist/linux-amd64
sha256sum Hello > Hello.sha256          # a checksum to publish next to the download
gpg --armor --detach-sign Hello         # Hello.asc: a signature anyone with your public key can check
```

Users check with `sha256sum -c Hello.sha256` and `gpg --verify Hello.asc Hello`. Packaging the app as a `.deb`,
Flatpak or AppImage, where the package or repository is signed, also works.

## Coming to desktop build

Signing from `pocketone desktop build` itself, with your certificates, is planned, so a release build comes out signed
and notarized.
