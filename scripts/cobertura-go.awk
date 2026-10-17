# Cobertura de la lógica de negocio en los servicios Go (Reto 6, D11 y D14).
#
# Modo "filtrar" (por defecto): lee el perfil de `go test -coverprofile` y escribe en
# la salida estándar solo los bloques de los paquetes de negocio. Un bloque se excluye
# si su primera línea, o la anterior (la del `if`), tiene el comentario
# `cobertura: inalcanzable (<razón>)`; cada exclusión se informa en stderr.
#
#   awk -v modulo="$(go list -m)" -v paquetes="internal/dominio internal/aplicacion" \
#       -f cobertura-go.awk reports/coverage.out > reports/coverage-negocio.out
#
# Modo "exigir": lee el perfil ya filtrado y termina con código 1 si alguna sentencia
# no se ejecutó, listando archivo:línea de cada bloque sin cubrir. Se cuenta con
# sentencias exactas (no con el porcentaje redondeado de `go tool cover -func`, que
# mostraría 100.0 % con una sentencia sin cubrir entre más de 2000).
#
#   awk -v modo=exigir -f cobertura-go.awk reports/coverage-negocio.out

BEGIN {
	if (modo == "") modo = "filtrar"
	if (modo == "filtrar") {
		if (modulo == "" || paquetes == "") {
			print "cobertura-go.awk: faltan -v modulo=... o -v paquetes=..." > "/dev/stderr"
			error = 1
			exit 1
		}
		n = split(paquetes, lista, " ")
		for (i = 1; i <= n; i++) incluido[lista[i]] = 1
	}
}

FNR == 1 && /^mode:/ { if (modo == "filtrar") print; next }

modo == "filtrar" {
	archivo = $1
	sub(/:[^:]*$/, "", archivo)
	rel = substr(archivo, length(modulo) + 2)
	dir = rel
	sub(/\/[^\/]*$/, "", dir)
	if (!(dir in incluido)) next
	visto[dir] = 1

	posicion = $1
	sub(/^.*:/, "", posicion)
	split(posicion, partes, ".")
	inicio = partes[1] + 0
	razon = marca(rel, inicio)
	if (razon != "") {
		if (!((rel, inicio) in avisado)) {
			printf "cobertura: excluido %s:%d — %s\n", rel, inicio, razon > "/dev/stderr"
			avisado[rel, inicio] = 1
		}
		next
	}
	print
	next
}

modo == "exigir" {
	total += $2
	if ($3 > 0) {
		cubiertas += $2
	} else {
		sin_cubrir[++faltan] = $1
	}
}

# marca devuelve la razón de la exclusión si la línea `inicio` o la anterior la tiene.
function marca(rel, inicio,    linea, l, m) {
	if (!(rel in leido)) {
		l = 0
		while ((getline linea < rel) > 0) fuente[rel, ++l] = linea
		close(rel)
		leido[rel] = 1
	}
	for (m = inicio - 1; m <= inicio; m++) {
		linea = fuente[rel, m]
		if (linea ~ /cobertura: inalcanzable/) {
			sub(/^.*cobertura: inalcanzable */, "", linea)
			return linea
		}
	}
	return ""
}

END {
	if (error) exit 1
	if (modo == "filtrar") {
		for (p in incluido) {
			if (!(p in visto)) {
				printf "cobertura: el paquete de negocio %s no aparece en el perfil\n", p > "/dev/stderr"
				exit 1
			}
		}
		exit 0
	}
	if (total == 0) {
		print "cobertura: el perfil de negocio está vacío" > "/dev/stderr"
		exit 1
	}
	printf "cobertura de la lógica de negocio: %d de %d sentencias (%.2f %%)\n", cubiertas, total, 100 * cubiertas / total
	if (faltan > 0) {
		print "cobertura: la lógica de negocio debe estar al 100 %; bloques sin cubrir:" > "/dev/stderr"
		for (i = 1; i <= faltan; i++) print "  " sin_cubrir[i] > "/dev/stderr"
		exit 1
	}
}
