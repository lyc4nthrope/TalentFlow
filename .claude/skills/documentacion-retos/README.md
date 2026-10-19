# Cómo documentamos los retos de TalentFlow

Guía para personas. La versión para la IA es `SKILL.md`: dice lo mismo, en forma de reglas.

## La idea en una frase

**Primero se planea en objetivos pequeños, después se demuestra cada cosa con evidencia real, y al final se explica para que cualquiera la entienda y la pueda probar.**

## Los documentos de cada reto

Todo vive en `docs/reto-NN/`. Cada archivo tiene un solo propósito.

| Archivo | Para qué | Quién lo lee |
|---|---|---|
| `PLAN.md` | Qué falta, qué decidimos y en qué orden lo hacemos | El equipo, mientras se trabaja |
| `README.md` (manual) | Cómo levantar, probar y entender el reto, criterio por criterio | El profesor y cualquier compañero |
| `pruebas.md` | La evidencia: salidas reales, números, lo que se rompió a propósito | Quien quiera comprobar que funciona |
| `sustentacion.md` | El paso a paso para la presentación, en el orden de la rúbrica | Quien sustenta |
| `INTEGRACION.md` | Solo si se junta trabajo de otra rama: qué se conservó, qué cambió y por qué | El equipo |

Además, el **README raíz** tiene una sección por reto con lo que el PDF pide "documentar en el README", y siempre abre con el estado y un índice por reto.

## El plan, que es la pieza central

1. **Diagnóstico:** una tabla `requisito → estado actual → brecha`, con evidencia (archivo y línea, o resultado de un comando). Así se ve exactamente qué falta.
2. **Decisiones `D1..Dn`:** una tabla `Decisión | Elección | Por qué`. Cada una es una recomendación justificada, no una lista de opciones.
3. **Fases `F0..Fn`:** pequeñas, de una sesión cada una, con su verificación concreta y un estado ⬜/✅.
4. **Fuera de alcance:** los defectos encontrados que no son de este reto, con evidencia.
5. **Aprendizajes por fase:** al cerrar cada fase se agrega qué se hizo, los números reales y las sorpresas. Si una decisión cambia, se corrige en su tabla y se explica por qué.

Cada fase termina verificada (pruebas, y una ejecución real cuando aplica) y con su propio commit.

## Las reglas de estilo

- **Español neutro, frases cortas, tablas en vez de párrafos** cuando se comparan cosas.
- **Todo decidido lleva su por qué.** Si nos apartamos del PDF, se dice qué cambió y la razón.
- **Nada sin verificar.** Los números se miden, las rutas se comprueban, los comandos se ejecutan, los enlaces se prueban. Si algo no se pudo verificar, se escribe así.
- **Comandos listos para copiar y pegar**, y lo que se espera ver al ejecutarlos.
- **Para el profesor:** el orden lo pone la rúbrica, no nosotros.
- **No repetir:** si algo ya está en otro archivo, se enlaza.

## Plantillas

En `assets/`: `PLAN.md`, `manual.md`, `pruebas.md`, `sustentacion.md`, `integracion.md` y `seccion-readme.md`. Se copian a `docs/reto-NN/` y se llenan.

Antes de hacer el commit se repasa `references/checklist.md`.

## Ejemplo real

El Reto 6 completo: [`docs/reto-06/`](../../../docs/reto-06/).
