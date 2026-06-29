# Use a lightweight Python base
FROM python:3.11-slim

# Set working directory
WORKDIR /app

# Install system dependencies (needed for chromadb/sqlite)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install requirements
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy source code
COPY . .

# Set environment variable for persistent storage
ENV CHROMA_DB_PATH=/data/chroma_db

# Expose gRPC and metrics ports
EXPOSE 50051 8000

# Run the server
CMD ["python", "server.py"]
