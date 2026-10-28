# SonarQube del Reto 7 con un arranque que fija los límites de disco de su Elasticsearch
# (D20 de docs/reto-07/PLAN.md). Versión exacta en lugar de la etiqueta flotante lts-community.
FROM sonarqube:9.9.8-community

COPY --chown=sonarqube:sonarqube arranque.sh /opt/sonarqube/docker/arranque.sh
RUN chmod +x /opt/sonarqube/docker/arranque.sh

ENTRYPOINT ["/opt/sonarqube/docker/arranque.sh"]
