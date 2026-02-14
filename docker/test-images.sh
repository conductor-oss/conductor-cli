#!/bin/bash
set -e

REGISTRY=${REGISTRY:-"orkes"}
VERSION=${VERSION:-"latest"}

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║   Testing Orkes CLI Docker Images     ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
echo ""

test_image() {
  local variant=$1
  local image="${REGISTRY}/cli-runner:${variant}-${VERSION}"

  echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${YELLOW}Testing ${variant} image${NC}"
  echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

  # Test 1: Image exists
  if ! docker image inspect "$image" >/dev/null 2>&1; then
    echo -e "${RED}✗ Image not found: $image${NC}"
    ((TESTS_FAILED++))
    return 1
  fi
  echo -e "${GREEN}✓ Image exists${NC}"

  # Test 2: CLI runs and shows version
  if ! docker run --rm "$image" --version >/dev/null 2>&1; then
    echo -e "${RED}✗ CLI failed to run${NC}"
    ((TESTS_FAILED++))
    return 1
  fi
  echo -e "${GREEN}✓ CLI runs successfully${NC}"

  # Test 3: Runtime available
  case $variant in
    base)
      if ! docker run --rm "$image" bash --version >/dev/null 2>&1; then
        echo -e "${RED}✗ Bash not available${NC}"
        ((TESTS_FAILED++))
        return 1
      fi
      echo -e "${GREEN}✓ Bash available${NC}"
      ;;
    python)
      if ! docker run --rm "$image" python --version >/dev/null 2>&1; then
        echo -e "${RED}✗ Python not available${NC}"
        ((TESTS_FAILED++))
        return 1
      fi
      echo -e "${GREEN}✓ Python available${NC}"
      ;;
    node)
      if ! docker run --rm "$image" node --version >/dev/null 2>&1; then
        echo -e "${RED}✗ Node.js not available${NC}"
        ((TESTS_FAILED++))
        return 1
      fi
      echo -e "${GREEN}✓ Node.js available${NC}"
      ;;
    java)
      if ! docker run --rm "$image" java -version >/dev/null 2>&1; then
        echo -e "${RED}✗ Java not available${NC}"
        ((TESTS_FAILED++))
        return 1
      fi
      echo -e "${GREEN}✓ Java available${NC}"
      ;;
    go)
      if ! docker run --rm "$image" go version >/dev/null 2>&1; then
        echo -e "${RED}✗ Go not available${NC}"
        ((TESTS_FAILED++))
        return 1
      fi
      echo -e "${GREEN}✓ Go available${NC}"
      ;;
    dotnet)
      if ! docker run --rm "$image" dotnet --version >/dev/null 2>&1; then
        echo -e "${RED}✗ .NET not available${NC}"
        ((TESTS_FAILED++))
        return 1
      fi
      echo -e "${GREEN}✓ .NET available${NC}"
      ;;
  esac

  # Test 4: Non-root user
  USER_CHECK=$(docker run --rm "$image" whoami 2>/dev/null || echo "error")
  if [ "$USER_CHECK" != "orkes" ]; then
    echo -e "${RED}✗ Not running as orkes user (found: $USER_CHECK)${NC}"
    ((TESTS_FAILED++))
    return 1
  fi
  echo -e "${GREEN}✓ Running as non-root user (orkes)${NC}"

  # Test 5: Required utilities
  for util in curl wget git jq; do
    if ! docker run --rm "$image" which $util >/dev/null 2>&1; then
      echo -e "${RED}✗ Utility $util not found${NC}"
      ((TESTS_FAILED++))
      return 1
    fi
  done
  echo -e "${GREEN}✓ All required utilities present (curl, wget, git, jq)${NC}"

  ((TESTS_PASSED++))
  echo -e "${GREEN}✓ All tests passed for ${variant}${NC}\n"
}

# Test all variants
for variant in base python node java go dotnet; do
  test_image "$variant"
done

echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║       Test Summary                     ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Images tested:${NC} 6"
echo -e "${GREEN}Tests passed:${NC}  ${TESTS_PASSED}"

if [ $TESTS_FAILED -gt 0 ]; then
  echo -e "${RED}Tests failed:${NC}  ${TESTS_FAILED}"
  echo ""
  echo -e "${RED}Some tests failed!${NC}"
  exit 1
else
  echo -e "${RED}Tests failed:${NC}  0"
  echo ""
  echo -e "${GREEN}All tests passed! ✓${NC}"
  exit 0
fi
