const express = require('express');
const { crearClienteDepartamentos } = require('./servicios/clienteDepartamentos');
const { AppError } = require('./errores');

const app = express();
app.use(express.json());

const PORT = process.env.PORT || 8081;
const DEPARTAMENTOS_URL = process.env.DEPARTAMENTOS_SERVICE_URL || 'http://departamentos-service:8082';

// Instanciar el cliente con Circuit Breaker integrado
const departamentosClient = crearClienteDepartamentos({
  baseUrl: DEPARTAMENTOS_URL,
  timeoutMs: 3000,
  maxReintentos: 1
});

// Base de datos ficticia en memoria (o tu conexión a PostgreSQL)
const empleadosDB = [];

// 1. Crear Empleado (Invocación síncrona con protección de Circuit Breaker)
app.post('/empleados', async (req, res, next) => {
  try {
    const { nombre, departamentoId } = req.body;

    if (!nombre || !departamentoId) {
      return res.status(400).json({ error: 'Nombre y departamentoId son obligatorios' });
    }

    // Verificar existencia con el cliente protegido por Opossum
    const existeDepto = await departamentosClient.existe(departamentoId);

    if (!existeDepto) {
      return res.status(404).json({ error: `El departamento ${departamentoId} no existe` });
    }

    const nuevoEmpleado = {
      id: empleadosDB.length + 1,
      nombre,
      departamentoId,
      fechaCreacion: new Date()
    };

    empleadosDB.push(nuevoEmpleado);
    return res.status(201).json(nuevoEmpleado);

  } catch (error) {
    // Captura el AppError 503 lanzado cuando el circuito está ABIERTO o la dependencia cae
    next(error);
  }
});

// 2. Listar Empleados
app.get('/empleados', (req, res) => {
  res.json(empleadosDB);
});

// 3. Documentación Swagger / OpenAPI (Rutas mapeadas en el Gateway)
app.get('/docs', (req, res) => {
  res.send('<h1>Documentación API Empleados</h1>');
});

app.get('/openapi.json', (req, res) => {
  res.json({ openapi: '3.0.0', info: { title: 'API Empleados', version: '1.0.0' } });
});

// Middleware Global para Manejo de Errores (Garantiza respuestas JSON de tipo AppError)
app.use((err, req, res, next) => {
  console.error(`[Error Empleados]: ${err.message}`);

  if (err instanceof AppError) {
    return res.status(err.statusCode || 500).json({
      error: err.message,
      status: err.statusCode
    });
  }

  return res.status(500).json({ error: 'Error interno del servidor de empleados' });
});

app.listen(PORT, () => {
  console.log(`Servicio de Empleados corriendo en puerto ${PORT}`);
});