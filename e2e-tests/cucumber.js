'use strict';

// Un solo comando (`npm test`) genera los tres reportes del P6 en rutas fijas
// (D9): JUnit XML para Jenkins, Cucumber JSON y HTML para las personas.
module.exports = {
  default: {
    paths: ['features/**/*.feature'],
    require: ['support/**/*.js', 'step_definitions/**/*.js'],
    format: [
      'pretty',
      'junit:reports/junit.xml',
      'json:reports/cucumber.json',
      'html:reports/cucumber.html',
    ],
    formatOptions: { snippetInterface: 'async-await' },
    strict: true,
  },
};
