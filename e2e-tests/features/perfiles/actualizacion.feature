# language: es
Característica: El empleado completa su perfil
  Como empleado
  Quiero agregar a mi perfil mi foto, mis redes sociales y mi código postal
  Para que mis compañeros me reconozcan y me encuentren

  La foto y las redes solo aceptan enlaces http o https: un enlace javascript: o data:
  se ejecutaría al mostrar el perfil en el navegador. El perfil lo crea gestion-perfiles
  al consumir empleado.creado; los pasos esperan a que exista antes de editarlo.

  Antecedentes:
    Dado que existe un empleado activo con credenciales establecidas
    Y que estoy autenticado como "USER"

  Escenario: El empleado agrega su foto, sus redes sociales y su código postal
    Cuando actualizo mi perfil con mi foto, mis redes sociales y mi código postal
    Entonces la respuesta debe tener código 200
    Y mi perfil debe mostrar la foto, las redes sociales y el código postal que envié

  Escenario: No se acepta una foto con un enlace javascript:
    Cuando actualizo la foto de mi perfil a "javascript:alert(1)"
    Entonces la respuesta debe tener código 400
    Y el error debe señalar el campo "foto"
