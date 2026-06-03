#!/bin/bash
# Build script for the Dnsway Android native library (Go → .so)
#
# Prerequisites:
#   - Go 1.21+
#   - Android NDK (r25+) installed, ANDROID_NDK_HOME set
#   - For local dev without NDK: brew install FiloSottile/musl-cross/musl-cross
#
# Usage:
#   ./build-android.sh           # Build for arm64 only (default)
#   ./build-android.sh all       # Build for arm64 + x86_64
#   ./build-android.sh arm64     # Build for arm64 only
#   ./build-android.sh clean     # Remove build artifacts
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ANDROID_DIR="$SCRIPT_DIR/android"
JNILIBS_DIR="$ANDROID_DIR/app/src/main/jniLibs"
GO_MOD_DIR="$SCRIPT_DIR"

# Library name that matches System.loadLibrary("dnsway")
LIB_NAME="libdnsway.so"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

clean() {
    echo -e "${YELLOW}Cleaning build artifacts...${NC}"
    rm -rf "$JNILIBS_DIR/arm64-v8a" "$JNILIBS_DIR/x86_64" "$JNILIBS_DIR/armeabi-v7a"
    # Clean Go build cache for this package
    cd "$GO_MOD_DIR"
    go clean -cache ./internal/android/
    echo -e "${GREEN}Cleaned.${NC}"
    exit 0
}

if [ "${1:-}" = "clean" ]; then
    clean
fi

# Check for Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed.${NC}"
    exit 1
fi

echo -e "${GREEN}Building shared library: ${LIB_NAME}${NC}"
echo ""

# Detect if NDK is available
NDK_AVAILABLE=false
TOOLCHAIN=""
if [ -n "${ANDROID_NDK_HOME:-}" ]; then
    TOOLCHAIN="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/darwin-x86_64"
    if [ -d "$TOOLCHAIN" ]; then
        NDK_AVAILABLE=true
        echo -e "${GREEN}Using NDK: $ANDROID_NDK_HOME${NC}"
    fi
fi

# Also try common NDK paths
if [ "$NDK_AVAILABLE" = false ]; then
    for NDK_PATH in "$HOME/Library/Android/sdk/ndk" "$HOME/Android/Sdk/ndk"; do
        if [ -d "$NDK_PATH" ]; then
            LATEST=$(ls -1 "$NDK_PATH" 2>/dev/null | sort -V | tail -1)
            if [ -n "$LATEST" ]; then
                TOOLCHAIN="$NDK_PATH/$LATEST/toolchains/llvm/prebuilt/darwin-x86_64"
                if [ -d "$TOOLCHAIN" ]; then
                    NDK_AVAILABLE=true
                    echo -e "${GREEN}Using NDK: $NDK_PATH/$LATEST${NC}"
                    break
                fi
            fi
        fi
    done
fi

# Build architectures
BUILD_ARMV7=false
BUILD_ARM64=true
BUILD_X86_64=false

case "${1:-}" in
    all)
        BUILD_ARMV7=true
        BUILD_ARM64=true
        BUILD_X86_64=true
        ;;
    arm64)
        BUILD_ARM64=true
        ;;
    armeabi-v7a|armv7)
        BUILD_ARMV7=true
        BUILD_ARM64=false
        ;;
    x86_64)
        BUILD_X86_64=true
        BUILD_ARM64=false
        ;;
    *)
        # Default: arm64 only
        ;;
esac

# Export domain categories from database to CSV for Android
export_categories() {
    local output_dir="$ANDROID_DIR/app/src/main/assets/categories"
    mkdir -p "$output_dir"

    # Check if PostgreSQL is available locally
    if command -v psql &> /dev/null; then
        echo -e "${YELLOW}Exporting domain categories from local PostgreSQL...${NC}"
        PGPASSWORD="${DB_PASSWORD:-}" psql -h "${DB_HOST:-localhost}" -U "${DB_USER:-postgres}" -d "${DB_NAME:-dnspc}" \
            -c "COPY (SELECT domain, category_id FROM domain_categories ORDER BY domain, category_id) TO '$output_dir/domain_categories.csv' WITH CSV" \
            2>/dev/null && {
            echo -e "${GREEN}Exported $(wc -l < "$output_dir/domain_categories.csv") domain-category mappings${NC}"
            return 0
        }
    fi

    # Fallback: create an empty file with header
    echo "domain,category_id" > "$output_dir/domain_categories.csv"
    echo -e "${YELLOW}Warning: Could not export from database. Created empty categories file.${NC}"
    echo -e "${YELLOW}Run manually: psql -d dnspc -c \"COPY domain_categories TO 'domain_categories.csv' WITH CSV\"${NC}"
}

ARCHES=""
if [ "$BUILD_ARM64" = true ]; then
    ARCHES="$ARCHES arm64"
fi
if [ "$BUILD_ARMV7" = true ]; then
    ARCHES="$ARCHES armv7"
fi
if [ "$BUILD_X86_64" = true ]; then
    ARCHES="$ARCHES x86_64"
fi

cd "$GO_MOD_DIR"

# Export domain categories from database
export_categories

for ARCH in $ARCHES; do
    case "$ARCH" in
        arm64)
            TARGET="android/arm64"
            ABI="arm64-v8a"
            CC=""
            if [ "$NDK_AVAILABLE" = true ]; then
                CC="$TOOLCHAIN/bin/aarch64-linux-android21-clang"
            fi
            ;;
        armv7)
            TARGET="android/arm"
            ABI="armeabi-v7a"
            CC=""
            if [ "$NDK_AVAILABLE" = true ]; then
                CC="$TOOLCHAIN/bin/armv7a-linux-androideabi21-clang"
            fi
            ;;
        x86_64)
            TARGET="android/amd64"
            ABI="x86_64"
            CC=""
            if [ "$NDK_AVAILABLE" = true ]; then
                CC="$TOOLCHAIN/bin/x86_64-linux-android21-clang"
            fi
            ;;
    esac

    OUT_DIR="$JNILIBS_DIR/$ABI"
    mkdir -p "$OUT_DIR"

    echo -e "${YELLOW}Building for $ABI ($TARGET)...${NC}"

    BUILD_ENV="GOOS=android CGO_ENABLED=1"
    if [ -n "$CC" ]; then
        BUILD_ENV="$BUILD_ENV CC=$CC"
    fi

    # Build shared library
    eval "$BUILD_ENV" go build \
        -buildmode=c-shared \
        -ldflags="-s -w" \
        -o "$OUT_DIR/$LIB_NAME" \
        ./internal/android/

    echo -e "${GREEN}  → $OUT_DIR/$LIB_NAME${NC}"

    # Show file info
    file "$OUT_DIR/$LIB_NAME" 2>/dev/null || true
done

echo ""
echo -e "${GREEN}✓ Build complete!${NC}"
echo ""
echo "Library locations:"
for f in "$JNILIBS_DIR"/*/"$LIB_NAME"; do
    if [ -f "$f" ]; then
        size=$(stat -f%z "$f" 2>/dev/null || stat -c%s "$f" 2>/dev/null || echo "?")
        echo "  $f ($(echo "scale=1; $size/1048576" | bc) MB)"
    fi
done

echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "  1. Run: cd $ANDROID_DIR && ./gradlew assembleDebug"
echo "  2. Install APK on device"
echo ""
echo -e "${GREEN}Done.${NC}"
