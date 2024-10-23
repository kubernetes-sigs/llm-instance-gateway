
FROM python:3.10-slim-buster

WORKDIR /dynamic-lora-reconciler

RUN python3 -m venv /opt/venv

ENV PATH="/opt/venv/bin:$PATH"

RUN pip install --upgrade pip
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY sidecar/sidecar.py . 

CMD ["python", "sidecar.py"]