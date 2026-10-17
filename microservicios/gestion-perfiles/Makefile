# Pruebas con reportes (Reto 6, D9, D10 y D14). Uso: make test
# Resultados en reports/junit.xml y cobertura (Cobertura) en reports/coverage.xml; las
# opciones están en pyproject.toml. Falla si una prueba falla o si la lógica de negocio
# no está al 100 % de líneas y ramas.
# Requiere las dependencias de requirements-dev.txt; por defecto usa el entorno .venv
# (python3 -m venv .venv && .venv/bin/pip install -r requirements-dev.txt).

PYTHON ?= .venv/bin/python

.PHONY: test
test:
	rm -rf reports && mkdir -p reports
	$(PYTHON) -m pytest
