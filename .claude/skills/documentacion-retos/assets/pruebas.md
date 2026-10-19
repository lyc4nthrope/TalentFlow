# Reto N — Evidencia

Cada sección dice qué se comprobó, cómo y qué salió. Las salidas son reales (recortadas), con fecha.

## 1. <Qué se comprobó>

Comando:

```bash
<comando exacto>
```

Resultado (AAAA-MM-DD):

```text
<salida real, recortada a lo que importa>
```

Lo que demuestra: <una o dos frases>.

## 2. Las pruebas fallan cuando deben

Cambio hecho a propósito (no versionado):

```diff
- <línea original>
+ <línea rota>
```

Qué falló y con qué mensaje: <salida real>. Se revirtió con `git checkout -- <ruta>`.
