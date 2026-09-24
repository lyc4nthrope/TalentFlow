for ($i = 2; $i -le 9; $i++) {
    Write-Host "--- Peticion $i ---"
    $inicio = Get-Date
    try {
        Invoke-WebRequest -Uri http://localhost:8080/empleados -Method POST -ContentType "application/json" -Body "{`"id`":`"E00$i`",`"nombre`":`"Test`",`"apellido`":`"Test`",`"email`":`"test$i@x.com`",`"numeroEmpleado`":`"EMP-00$i`",`"cargo`":`"Dev`",`"area`":`"TI`",`"departamentoId`":`"IT`",`"fechaIngreso`":`"2026-01-01`"}" -UseBasicParsing | Out-Null
    } catch {
        Write-Host "Error capturado (esperado si el circuito no acepta la peticion)"
    }
    $fin = Get-Date
    Write-Host "Duracion: $(($fin - $inicio).TotalMilliseconds) ms"
}