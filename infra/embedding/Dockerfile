FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    HF_HOME=/models \
    TRANSFORMERS_CACHE=/models

RUN groupadd --system embedding \
    && useradd --system --gid embedding --home-dir /app embedding \
    && mkdir -p /app /models \
    && chown -R embedding:embedding /app /models

WORKDIR /app
COPY requirements.txt ./requirements.txt
RUN pip install --no-cache-dir -r requirements.txt
COPY app.py ./app.py
RUN chown embedding:embedding /app/app.py

USER embedding
EXPOSE 8090
HEALTHCHECK --interval=10s --timeout=5s --start-period=180s --retries=12 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8090/healthz', timeout=3)"
ENTRYPOINT ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8090"]
