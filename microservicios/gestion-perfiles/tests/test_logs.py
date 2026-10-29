import json
import logging

from opentelemetry.sdk.trace import TracerProvider

from app.logs import FormatoJson, configurar


def _registro(mensaje: str, *args) -> logging.LogRecord:
    return logging.LogRecord("perfiles", logging.INFO, __file__, 1, mensaje, args, None)


def test_linea_json_con_formato_comun_sin_traza():
    linea = json.loads(FormatoJson("perfiles-service").format(_registro("perfil %s creado", "E001")))
    assert linea["level"] == "INFO"
    assert linea["service"] == "perfiles-service"
    assert linea["message"] == "perfil E001 creado"
    assert "traceId" not in linea


def test_incluye_traceid_del_span_activo_y_el_error():
    tracer = TracerProvider().get_tracer("prueba")
    with tracer.start_as_current_span("process") as span:
        try:
            raise ValueError("boom")
        except ValueError:
            registro = _registro("falló")
            registro.exc_info = __import__("sys").exc_info()
            linea = json.loads(FormatoJson("x").format(registro))
    assert linea["traceId"] == format(span.get_span_context().trace_id, "032x")
    assert "ValueError: boom" in linea["error"]


def test_sin_espacios_para_el_enlace_de_grafana_a_jaeger():
    tracer = TracerProvider().get_tracer("prueba")
    with tracer.start_as_current_span("x"):
        texto = FormatoJson("x").format(_registro("hola"))
    assert '"traceId":"' in texto


def test_configurar_deja_un_solo_manejador_json_tambien_para_uvicorn():
    filtro = logging.Filter("nada")
    configurar("x", (filtro,))
    assert isinstance(logging.getLogger().handlers[0].formatter, FormatoJson)
    assert logging.getLogger("uvicorn.access").handlers == []
    assert filtro in logging.getLogger("uvicorn.access").filters
    logging.getLogger("uvicorn.access").removeFilter(filtro)
