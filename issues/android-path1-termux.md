# Android Path 1: Termux + Browser Shortcut

No root required. No code changes needed.

## Steps

### 1. Install Termux
- Download from **F-Droid** (NOT Google Play Store): https://f-droid.org/en/packages/com.termux/
- The Play Store version is outdated and packages will fail to install.

### 2. Set Up Go and Ondict
```bash
pkg update && pkg upgrade
pkg install golang git
go install github.com/ChaosNyaruko/ondict@latest
```

### 3. Copy Dictionary Files
Connect phone via USB (or use any file manager) and copy your `.mdx`/`.mdd` files to:
```
/data/data/com.termux/files/home/.config/ondict/dicts/
```
Or copy to SD card first, then move from Termux:
```bash
mkdir -p ~/.config/ondict/dicts
cp /sdcard/your-dict.mdx ~/.config/ondict/dicts/
```

### 4. Start the Server
```bash
ondict -serve -listen=:1345
```

### 5. Add Home Screen Shortcut
- Open Chrome, navigate to `http://localhost:1345`
- Tap the three-dot menu → "Add to Home Screen"
- Give it a name (e.g. "Ondict")

Tapping that icon opens the dictionary directly in Chrome. Looks and feels close to a real app.

### 6. (Optional) Auto-start on Boot
Install the Termux:Boot add-on from F-Droid:
```bash
pkg install termux-boot
```
Create the boot script:
```bash
mkdir -p ~/.termux/boot
cat > ~/.termux/boot/start-ondict.sh << 'EOF'
#!/data/data/com.termux/files/usr/bin/sh
ondict -serve -listen=:1345
EOF
chmod +x ~/.termux/boot/start-ondict.sh
```
After this, the server starts automatically when the phone boots.

## Limitations
- Termux must be running in the background (don't force-stop it)
- Not a true APK; opens in the browser, not a standalone app
- Android may kill Termux if under heavy memory pressure (acquire a wakelock via Termux:API if needed)
