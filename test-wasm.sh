#!/bin/bash

# WASM Plugin Test Script
# This script demonstrates all WASM plugin functionality

set -e

echo "=========================================="
echo "Nord WASM Plugin Test Suite"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if TinyGo is installed
if ! command -v tinygo &> /dev/null; then
    echo -e "${YELLOW}Warning: TinyGo not found. Install from https://tinygo.org${NC}"
    echo "Skipping build step..."
else
    echo -e "${BLUE}Step 1: Building WASM plugins...${NC}"
    make build-wasm
    echo -e "${GREEN}✓ Build complete${NC}"
    echo ""
fi

echo -e "${BLUE}Step 2: Listing loaded plugins...${NC}"
go run . -p wasm -a list 2>&1 | grep -A 10 "Loaded WASM Plugins"
echo -e "${GREEN}✓ Plugins loaded${NC}"
echo ""

echo -e "${BLUE}Step 3: Testing Demo Plugin${NC}"
echo ""

echo "  → Getting plugin info..."
go run . -p wasm -a execute "plugin=demo req_action=info" 2>&1 | grep -A 10 "Plugin Response"
echo ""

echo "  → Testing ping action..."
go run . -p wasm -a execute "plugin=demo req_action=ping target=test-server" 2>&1 | grep -A 10 "Plugin Response"
echo ""

echo "  → Testing collect action..."
go run . -p wasm -a execute "plugin=demo req_action=collect hostname=server1" 2>&1 | grep -A 15 "Plugin Response"
echo ""

echo "  → Testing status action..."
go run . -p wasm -a execute "plugin=demo req_action=status" 2>&1 | grep -A 10 "Plugin Response"
echo -e "${GREEN}✓ Demo plugin tests passed${NC}"
echo ""

echo -e "${BLUE}Step 4: Testing Network Monitor Plugin${NC}"
echo ""

echo "  → Getting plugin info..."
go run . -p wasm -a execute "plugin=network-monitor req_action=info" 2>&1 | grep -A 10 "Plugin Response"
echo ""

echo "  → Collecting network metrics..."
go run . -p wasm -a execute "plugin=network-monitor req_action=collect host=router" 2>&1 | grep -A 15 "Plugin Response"
echo ""

echo "  → Checking latency..."
go run . -p wasm -a execute "plugin=network-monitor req_action=check_latency target=8.8.8.8" 2>&1 | grep -A 10 "Plugin Response"
echo ""

echo "  → Checking bandwidth..."
go run . -p wasm -a execute "plugin=network-monitor req_action=check_bandwidth interface=eth0" 2>&1 | grep -A 10 "Plugin Response"
echo -e "${GREEN}✓ Network monitor plugin tests passed${NC}"
echo ""

echo "=========================================="
echo -e "${GREEN}All tests passed! ✓${NC}"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. See WASM_QUICKSTART.md for integration guide"
echo "  2. Check examples/ directory for plugin templates"
echo "  3. Read plugins/wasm/README.md for detailed docs"
echo ""
