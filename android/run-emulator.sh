#!/bin/bash
# Run the DnswayTest AVD in the Android emulator (Apple Silicon native)
set -euo pipefail

export JAVA_HOME="/usr/local/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home"
export ANDROID_HOME="$HOME/.android/sdk"
export PATH="$JAVA_HOME/bin:$ANDROID_HOME/emulator:$ANDROID_HOME/platform-tools:$PATH"

EMULATOR="$ANDROID_HOME/emulator/emulator"
AVD_NAME="DnswayTest"

echo "Starting emulator: $AVD_NAME"
echo "Using: $EMULATOR"
echo ""

# Launch emulator natively on Apple Silicon
"$EMULATOR" \
    -avd "$AVD_NAME" \
    -no-snapshot \
    -netdelay none \
    -netspeed full \
    -memory 2048 \
    -no-boot-anim &

EMU_PID=$!
echo "Emulator PID: $EMU_PID"
echo "Waiting for boot..."
echo ""
echo "Use: adb wait-for-device && adb shell 'while getprop sys.boot_completed 2>/dev/null; do sleep 2; done'"
echo ""
echo "To install APK once booted:"
echo "  adb install -r android/app/build/outputs/apk/debug/app-debug.apk"
echo ""
echo "To stop emulator: kill $EMU_PID"
