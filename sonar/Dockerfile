# Imagen del arranque de SonarQube (Reto 7, D15): solo curl y jq sobre Alpine fijada.
FROM alpine:3.20.3
RUN apk add --no-cache curl jq
COPY init.sh /init.sh
ENTRYPOINT ["/bin/sh", "/init.sh"]
