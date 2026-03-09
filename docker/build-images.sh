#!/bin/bash
set -e

VERSION=${VERSION:-"latest"}
REGISTRY=${REGISTRY:-"orkes"}
PUSH=${PUSH:-"false"}
PLATFORM=${PLATFORM:-"linux/amd64"}

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  Building Orkes CLI Docker Images     ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Configuration:${NC}"
echo -e "  Version:   ${YELLOW}${VERSION}${NC}"
echo -e "  Registry:  ${YELLOW}${REGISTRY}${NC}"
echo -e "  Platform:  ${YELLOW}${PLATFORM}${NC}"
echo -e "  Push:      ${YELLOW}${PUSH}${NC}"
echo ""

# Build base image
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Building base image...${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
docker build \
  -f docker/Dockerfile-base \
  -t ${REGISTRY}/cli-runner:${VERSION} \
  -t ${REGISTRY}/cli-runner:base-${VERSION} \
  --platform ${PLATFORM} \
  .
echo -e "${GREEN}✓ Base image built successfully${NC}\n"

# Build runtime variants
for VARIANT in python node java go dotnet; do
  echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${YELLOW}Building ${VARIANT} image...${NC}"
  echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  docker build \
    -f docker/Dockerfile-${VARIANT} \
    -t ${REGISTRY}/cli-runner:${VARIANT}-${VERSION} \
    --platform ${PLATFORM} \
    .
  echo -e "${GREEN}✓ ${VARIANT} image built successfully${NC}\n"
done

# Tag latest
if [ "$VERSION" = "latest" ]; then
  echo -e "${BLUE}Tagging latest versions...${NC}"
  for VARIANT in base python node java go dotnet; do
    docker tag ${REGISTRY}/cli-runner:${VARIANT}-${VERSION} ${REGISTRY}/cli-runner:${VARIANT}
    echo -e "${GREEN}✓ Tagged ${REGISTRY}/cli-runner:${VARIANT}${NC}"
  done
  echo ""
fi

# Push if requested
if [ "$PUSH" = "true" ]; then
  echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${YELLOW}Pushing images to registry...${NC}"
  echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

  docker push ${REGISTRY}/cli-runner:${VERSION}
  docker push ${REGISTRY}/cli-runner:base-${VERSION}

  for VARIANT in python node java go dotnet; do
    docker push ${REGISTRY}/cli-runner:${VARIANT}-${VERSION}
    echo -e "${GREEN}✓ Pushed ${VARIANT}-${VERSION}${NC}"
  done

  if [ "$VERSION" = "latest" ]; then
    docker push ${REGISTRY}/cli-runner:base
    for VARIANT in python node java go dotnet; do
      docker push ${REGISTRY}/cli-runner:${VARIANT}
      echo -e "${GREEN}✓ Pushed ${VARIANT}${NC}"
    done
  fi
  echo ""
fi

echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║         Build Complete! ✓              ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Built images:${NC}"
echo -e "  ${REGISTRY}/cli-runner:${VERSION}"
echo -e "  ${REGISTRY}/cli-runner:base-${VERSION}"
for VARIANT in python node java go dotnet; do
  echo -e "  ${REGISTRY}/cli-runner:${VARIANT}-${VERSION}"
done
echo ""
