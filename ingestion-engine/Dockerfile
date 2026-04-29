FROM python:3.12-slim

WORKDIR /app

# W5.4: install runtime deps from the hash-pinned lockfile, not from
# pyproject.toml directly. requirements.txt is pip-compile'd from
# pyproject.toml with --generate-hashes; CI's "Verify lockfile freshness"
# step keeps it in sync. Using the lockfile here eliminates version drift
# between developer machines, CI, and production images.
COPY requirements.txt ./
RUN pip install --no-cache-dir --require-hashes -r requirements.txt

# Install the application itself without re-resolving dependencies.
COPY . .
RUN pip install --no-cache-dir --no-deps .

EXPOSE 8081

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8081"]
