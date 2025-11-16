#!/bin/bash
# Script to install Playwright system dependencies
# Run with: sudo bash install-playwright-deps.sh

set -euo pipefail

echo "Installing Playwright system dependencies..."

# Update package lists
apt-get update

# Install all required Playwright dependencies
apt-get install -y --no-install-recommends \
  libasound2t64 libatk-bridge2.0-0t64 libatk1.0-0t64 libatspi2.0-0t64 \
  libcairo2 libcups2t64 libdbus-1-3 libdrm2 libgbm1 libglib2.0-0t64 \
  libnspr4 libnss3 libpango-1.0-0 libx11-6 libxcb1 libxcomposite1 \
  libxdamage1 libxext6 libxfixes3 libxkbcommon0 libxrandr2 \
  libcairo-gobject2 libfontconfig1 libfreetype6 libgdk-pixbuf-2.0-0 \
  libgtk-3-0t64 libpangocairo-1.0-0 libx11-xcb1 libxcb-shm0 libxcursor1 \
  libxi6 libxrender1 gstreamer1.0-libav gstreamer1.0-plugins-bad \
  gstreamer1.0-plugins-base gstreamer1.0-plugins-good libicu74 \
  libatomic1 libenchant-2-2 libepoxy0 libevent-2.1-7t64 libflite1 \
  libgles2 libgstreamer-gl1.0-0 libgstreamer-plugins-bad1.0-0 \
  libgstreamer-plugins-base1.0-0 libgstreamer1.0-0 libgtk-4-1 \
  libharfbuzz-icu0 libharfbuzz0b libhyphen0 libjpeg-turbo8 liblcms2-2 \
  libmanette-0.2-0 libopus0 libpng16-16t64 libsecret-1-0 libvpx9 \
  libwayland-client0 libwayland-egl1 libwayland-server0 libwebp7 \
  libwebpdemux2 libwoff1 libxml2 libxslt1.1 libx264-164 libavif16 \
  xvfb fonts-noto-color-emoji fonts-unifont xfonts-cyrillic \
  xfonts-scalable fonts-liberation fonts-ipafont-gothic \
  fonts-wqy-zenhei fonts-tlwg-loma-otf fonts-freefont-ttf

echo ""
echo "✅ Playwright system dependencies installed successfully!"
echo ""
echo "You can now verify the installation by running:"
echo "  cd web/portal"
echo "  export PATH=\$PATH:\$HOME/.local/share/pnpm"
echo "  pnpm exec playwright --version"

