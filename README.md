# TalentFlow

Sistema de onboarding y offboarding de empleados basado en una arquitectura orientada a microservicios.

## Estructura

```
TalentFlow/
├── shared/                  # Modelo canónico compartido (@talentflow/shared)
├── services/                # Servicios (cada reto agrega o evoluciona un servicio)
│   └── empleados/           # Reto 1
├── docs/                    # Evidencia y decisiones por reto
├── docker-compose.yml       # Orquesta todos los servicios
└── package.json             # npm workspaces
```

## Mapa de retos

| Reto | Servicio | Estado | Qué se agregó | Cómo correr |
|------|----------|--------|---------------|-------------|
| 1 | empleados | ✅ Completado | POST/GET, modelo canónico, validaciones, Docker, 19 pruebas | `npm run dev:empleados` |

## Comandos comunes

Instalar dependencias (una sola vez, desde la raíz):

```bash
npm install
```

Ejecutar las pruebas de todos los servicios:

```bash
npm test
```

Correr todos los servicios (Docker):

```bash
docker compose up
```

## Stack del semestre

- **Node.js + Express** - servicios de negocio core (empleados, departamentos)
- **Python + FastAPI** - servicios de datos y cálculos de negocio
- **Go** - infraestructura (gateway/eventos)
- **TypeScript + React** - frontend del proyecto final
