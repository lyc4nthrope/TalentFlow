'use strict';

const { When, Then } = require('@cucumber/cucumber');
const { exigirCodigo, extracto, ALIAS_USUARIO } = require('../support/world');
const { esperarPerfil } = require('../support/operaciones');

// Perfil del empleado autenticado (proyecto final, F2): foto, redes sociales y código postal.

async function actualizarMiPerfil(mundo, cuerpo) {
  const { id } = mundo.recurso(ALIAS_USUARIO);
  await esperarPerfil(mundo, id);
  mundo.recursos.set('perfil enviado', cuerpo);
  await mundo.peticion('PUT', `/perfiles/${encodeURIComponent(id)}`, { cuerpo });
}

When('actualizo mi perfil con mi foto, mis redes sociales y mi código postal', async function () {
  const { id } = this.recurso(ALIAS_USUARIO);
  await actualizarMiPerfil(this, {
    foto: `https://cdn.talentflow.test/fotos/${id}.jpg`,
    // Sin twitter: redesSociales reemplaza el objeto, la red no enviada queda null.
    redesSociales: { linkedin: `https://www.linkedin.com/in/${id}`, github: `https://github.com/${id}` },
    codigoPostal: '630001',
  });
});

When('actualizo la foto de mi perfil a {string}', async function (foto) {
  await actualizarMiPerfil(this, { foto });
});

// Se relee el perfil: el 200 del PUT no basta, tiene que haber quedado guardado.
Then('mi perfil debe mostrar la foto, las redes sociales y el código postal que envié', async function () {
  const { id } = this.recurso(ALIAS_USUARIO);
  const enviado = this.recurso('perfil enviado');
  const { body } = exigirCodigo(await this.enviar('GET', `/perfiles/${encodeURIComponent(id)}`), 200, 'Consultar mi perfil');
  const esperado = {
    foto: enviado.foto,
    redesSociales: { linkedin: enviado.redesSociales.linkedin, github: enviado.redesSociales.github, twitter: null },
    codigoPostal: enviado.codigoPostal,
  };
  const obtenido = { foto: body?.foto, redesSociales: body?.redesSociales, codigoPostal: body?.codigoPostal };
  if (JSON.stringify(obtenido) !== JSON.stringify(esperado)) {
    throw new Error(
      `GET /perfiles/${id} debía traer ${extracto(esperado)} y trae ${extracto(obtenido)}. Cuerpo: ${extracto(body)}`
    );
  }
});
