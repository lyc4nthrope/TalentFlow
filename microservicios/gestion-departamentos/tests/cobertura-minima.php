<?php

// Uso: php tests/cobertura-minima.php reports/clover.xml 70. Sale con 1 si las líneas
// cubiertas de src/ quedan por debajo del umbral o si no hay reporte.
[, $archivo, $umbral] = $argv + [null, 'reports/clover.xml', '70'];
$clover = @simplexml_load_file($archivo);
if ($clover === false) {
    fwrite(STDERR, "No se pudo leer el reporte de cobertura $archivo\n");
    exit(1);
}
$metricas = $clover->project->metrics;
$total = (int) $metricas['statements'];
$cubiertas = (int) $metricas['coveredstatements'];
$porcentaje = $total > 0 ? 100 * $cubiertas / $total : 0;
printf("Cobertura de líneas de src/: %.2f %% (%d/%d), mínimo %s %%\n", $porcentaje, $cubiertas, $total, $umbral);
exit($porcentaje >= (float) $umbral ? 0 : 1);
