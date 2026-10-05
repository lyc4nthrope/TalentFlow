for ($i = 1; $i -le 8; $i++) {
    Write-Host "--- Peticion $i ---"
    $inicio = Get-Date
    try {
        Invoke-WebRequest -Uri http://localhost:8080/empleados -Method POST -ContentType "application/json" -Body "{`"id`":`"E20$i`",`"nombre`":`"Test`",`"apellido`":`"Test`",`"email`":`"test2$i@x.com`",`"numeroEmpleado`":`"EMP-20$i`",`"cargo`":`"Dev`",`"area`":`"TI`",`"departamentoId`":`"IT`",`"fechaIngreso`":`"2026-01-01`"}" -UseBasicParsing | Out-Null
    } catch {
        Write-Host "Error capturado (puede ser normal)"
    }
    $fin = Get-Date
    Write-Host "Duracion: $(($fin - $inicio).TotalMilliseconds) ms"
    $estado = Invoke-WebRequest -Uri http://localhost:8080/empleados/circuito-departamentos -UseBasicParsing | ConvertFrom-Json
    Write-Host "Estado del circuito: $($estado.estado)"
}