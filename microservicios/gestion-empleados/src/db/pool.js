const { Pool } = require("pg");

function crearPool(config = {}) {
  const {
    host = process.env.DB_HOST,
    port = process.env.DB_PORT ? Number(process.env.DB_PORT) : 5432,
    database = process.env.DB_NAME,
    user = process.env.DB_USER,
    password = process.env.DB_PASS
  } = config;

  return new Pool({ host, port, database, user, password });
}

module.exports = { crearPool };