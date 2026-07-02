FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    DEBIAN_FRONTEND=noninteractive \
    PATH="/root/.opencode/bin:${PATH}"

WORKDIR /app

# TeX Live powers the compile gate; latexmk drives it. Figure scripts need matplotlib
# (installed via requirements.txt). Without TeX Live the compile phase cannot produce a PDF.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      bash ca-certificates curl \
      latexmk texlive-latex-recommended texlive-latex-extra texlive-fonts-recommended && \
    rm -rf /var/lib/apt/lists/* && \
    curl -fsSL https://opencode.ai/install | bash

COPY requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir -r /app/requirements.txt && \
    python -c "import agentfield.harness.providers.opencode" && \
    opencode --version

COPY . /app/

EXPOSE 8001

CMD ["python", "main.py"]
