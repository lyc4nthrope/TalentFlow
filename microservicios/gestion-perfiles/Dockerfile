# Etapa 1: instala las dependencias en un entorno virtual aislado.
FROM python:3.13-slim AS builder
ENV PIP_NO_CACHE_DIR=1 PIP_DISABLE_PIP_VERSION_CHECK=1
RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"
# Dependencias primero: si solo cambia el código, Docker reutiliza esta capa.
COPY requirements.txt .
RUN pip install -r requirements.txt

# Etapa 2: imagen final con el entorno ya instalado, sin herramientas de build.
FROM python:3.13-slim AS runner
ENV PATH="/opt/venv/bin:$PATH" \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1
WORKDIR /app
# Usuario sin privilegios: si el proceso se ve comprometido, no es root del contenedor.
RUN useradd --system --uid 10001 --no-create-home perfiles
COPY --from=builder /opt/venv /opt/venv
COPY app ./app
USER perfiles
EXPOSE 8083
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8083"]
