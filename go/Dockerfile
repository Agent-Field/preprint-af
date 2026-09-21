FROM golang:1.24-bookworm AS builder

WORKDIR /src/go
COPY go/go.mod go/go.sum ./
RUN go mod download
COPY go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/preprint-af ./cmd/preprint-af

FROM python:3.11-slim AS runtime

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PATH="/root/.opencode/bin:${PATH}" \
    AI_MODEL="openrouter/deepseek/deepseek-v4-pro" \
    OPENCODE_MODEL="openrouter/deepseek/deepseek-v4-pro" \
    PORT=8001

RUN apt-get update && apt-get install -y --no-install-recommends \
      bash ca-certificates curl git \
      latexmk texlive-latex-recommended texlive-latex-extra texlive-fonts-recommended && \
    rm -rf /var/lib/apt/lists/* && \
    pip install --no-cache-dir matplotlib numpy pandas && \
    curl -fsSL https://opencode.ai/install | bash && \
    opencode --version && \
    latexmk -v >/dev/null && \
    pdflatex --version >/dev/null && \
    python -c "import matplotlib, numpy, pandas; matplotlib.use('Agg'); import matplotlib.pyplot as plt; plt.plot([0,1],[0,1]); plt.savefig('/tmp/preprint-af-figure.png', dpi=300); plt.savefig('/tmp/preprint-af-figure.pdf')" && \
    test -s /tmp/preprint-af-figure.png && \
    test -s /tmp/preprint-af-figure.pdf && \
    rm /tmp/preprint-af-figure.png /tmp/preprint-af-figure.pdf

WORKDIR /app
COPY --from=builder /out/preprint-af /usr/local/bin/preprint-af
COPY go/testdata/latex-smoke /opt/preprint-af/latex-smoke
RUN cd /opt/preprint-af/latex-smoke && \
    for asset in article.cls geometry.sty amsmath.sty amssymb.sty graphicx.sty booktabs.sty xcolor.sty tcolorbox.sty natbib.sty hyperref.sty plainnat.bst; do kpsewhich "$asset" | grep -q .; done && \
    bibtex --version >/dev/null && \
    latexmk -pdf -interaction=nonstopmode -halt-on-error main.tex >/dev/null && \
    test -s main.pdf
RUN mkdir -p /app/.tmp

EXPOSE 8001
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD command -v latexmk >/dev/null && command -v pdflatex >/dev/null && curl -f http://localhost:${PORT}/health || exit 1
ENTRYPOINT ["/usr/local/bin/preprint-af"]
