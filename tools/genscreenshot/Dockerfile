# Headless Chromium plus CJK / emoji fonts on top of the Go toolchain, used
# by the `screenshot` compose service to regenerate the README sample
# screenshots from doc/samples/ (Issue #132):
#   docker compose run --rm screenshot
# fonts-noto-cjk renders the Japanese sample, fonts-noto-color-emoji the
# emoji and reactions.
FROM golang:1.26

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        chromium \
        fonts-noto-cjk \
        fonts-noto-color-emoji \
    && rm -rf /var/lib/apt/lists/*

ENV CHROME_BIN=/usr/bin/chromium
