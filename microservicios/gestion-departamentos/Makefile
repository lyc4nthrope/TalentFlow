# Pruebas con reportes (proyecto final, F1). Uso: make test (sin PHP en el host: todo en Docker).
# Deja reports/junit.xml y reports/clover.xml. Falla si una prueba falla, si no hay reporte
# de cobertura o si las líneas de src/ quedan por debajo del 70 %.

.PHONY: test
test:
	rm -rf reports
	docker build --progress=plain --target reportes -o reports .
	@# Rutas del contenedor (/app/...) a las del host, para que SonarQube encuentre los archivos.
	sed -i 's#"/app/#"$(CURDIR)/#g' reports/clover.xml reports/junit.xml
	@estado=$$(cat reports/estado 2>/dev/null || echo 1); \
	if [ "$$estado" != 0 ]; then echo "make test: fallaron las pruebas o la cobertura (código $$estado)"; exit 1; fi
