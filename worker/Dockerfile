FROM python:3.12-slim
WORKDIR /app
ENV PYTHONUNBUFFERED=1

COPY worker/pyproject.toml ./
COPY worker/src/ ./src/
RUN pip install --no-cache-dir .

CMD ["pubmed-worker"]
