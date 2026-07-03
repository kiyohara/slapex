# VHS (charmbracelet's terminal recorder) plus CJK fonts, used by the `vhs`
# compose service to record the README terminal demo GIF (Issue #115).
# fonts-noto-cjk provides "Noto Sans Mono CJK JP" for the Japanese fixture.
FROM ghcr.io/charmbracelet/vhs:v0.11.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends fonts-noto-cjk \
    && rm -rf /var/lib/apt/lists/*
