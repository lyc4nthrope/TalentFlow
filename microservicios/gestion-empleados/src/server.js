const { crearApp } = require("./app");
const { crearServicioEmpleados } = require("./services/empleados.service");
const { crearRepositorioEmpleadosEnMemoria } = require("./repository/empleados.repository");

const PUERTO = process.env.PORT || 8080;

const repositorio = crearRepositorioEmpleadosEnMemoria();
const servicio = crearServicioEmpleados(repositorio);
const app = crearApp(servicio);

app.listen(PUERTO, () => {
  console.log(`Servicio de empleados escuchando en http://localhost:${PUERTO}`);
});
