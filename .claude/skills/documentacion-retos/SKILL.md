---
name: documentacion-retos
description: "Trigger: documentar reto, plan del reto, README del reto, manual, evidencia, guion de sustentación. Documenta cada reto de TalentFlow con el formato del equipo."
license: Apache-2.0
metadata:
  author: "lyc4nthrope"
  version: "1.0"
---

## Activation Contract

Cargar al planear, documentar, evidenciar o preparar la sustentación de un reto, o al actualizar el README raíz.

## Hard Rules

- Escribir en español neutro y profesional, con frases cortas. Preferir tablas a la prosa.
- Cada decisión lleva su **por qué**. Cada desviación del PDF dice qué cambió y la razón.
- Ningún dato sin verificar: los números salen de una ejecución real; las rutas, comandos y enlaces se comprueban antes del commit.
- No inventar ni redondear: si algo no se pudo verificar, se dice.
- El README raíz siempre abre con el estado ("Retos 1 a N completados") y el índice por reto.
- Un documento por propósito; no duplicar contenido entre archivos: enlazar.
- Commits `docs(retoN): …` (conventional commits, sin atribución a IA).

## Decision Gates

| Necesito… | Archivo | Plantilla |
|---|---|---|
| Planear el reto | `docs/reto-NN/PLAN.md` | `assets/PLAN.md` |
| Explicar el reto a quien lo usa o lo evalúa | `docs/reto-NN/README.md` | `assets/manual.md` |
| Guardar la evidencia de lo verificado | `docs/reto-NN/pruebas.md` | `assets/pruebas.md` |
| Preparar la sustentación | `docs/reto-NN/sustentacion.md` | `assets/sustentacion.md` |
| Integrar el trabajo de otra rama | `docs/reto-NN/INTEGRACION.md` | `assets/integracion.md` |
| Entregables que pide el PDF en el README | sección `## Tema (Reto N)` del README raíz | `assets/seccion-readme.md` |

## Execution Steps

1. Leer el PDF del reto (entregables, consideraciones, criterios) y el estado real del código.
2. Escribir `PLAN.md`: diagnóstico con evidencia, decisiones `D1..Dn`, fases `F0..Fn` pequeñas con verificación y estado.
3. Al cerrar cada fase: marcar ✅ y agregar su entrada en "Aprendizajes por fase" (qué se hizo, números reales, sorpresas).
4. Al terminar: la sección del README raíz, el manual, `pruebas.md` y `sustentacion.md`; actualizar el índice y el mapa de retos del README.
5. Verificar con `references/checklist.md` y hacer el commit.

## Output Contract

Devolver los archivos creados o modificados, qué se verificó (comandos ejecutados, enlaces comprobados) y lo que quedó sin verificar.

## References

- `README.md` — la misma guía, explicada para personas.
- `references/checklist.md` — verificación antes del commit.
- `assets/` — plantillas de cada documento.
