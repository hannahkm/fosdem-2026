FROM python:3.11-slim

# Install system dependencies needed for Frida
RUN apt-get update && apt-get install -y \
    procps \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /frida

# Copy requirements and install Python dependencies
COPY app/injector/sidecar/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# Copy Frida hook scripts and bridge application
COPY app/injector/sidecar/hook.js app/injector/sidecar/hook-simple.js app/injector/sidecar/hook-test.js app/injector/sidecar/hook-registers.js app/injector/sidecar/hook-trampoline.js app/injector/sidecar/bridge.py ./

# Run Python in unbuffered mode so logs appear immediately
CMD ["python", "-u", "bridge.py"]
