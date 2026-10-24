FROM jenkins/jenkins:lts-jdk17

# Instalar plugins necesarios de forma automatizada (incliye reportes de pruebas)
# Ref: https://github.com/jenkinsci/docker#preinstalling-plugins
RUN jenkins-plugin-cli --plugins \
    docker-workflow \
    pipeline-stage-view \
    workflow-aggregator \
    git \
    configuration-as-code \
    job-dsl \
    sonar \
    # --- Publicación de reportes de pruebas (Reto 6) ---
    junit \
    coverage \
    cucumber-reports \
    htmlpublisher

# Isntalar cliente Docker dentro del contenedor Jenkins
USER root
RUN apt-get update && \
    apt-get install -y docker.io docker-compose-plugin && \
    rm -rf /var/lib/apt/lists/*

# Agregar usuario jenkins al grupo docker para accerder al socket
RUN groupadd -f docker && usermod -aG docker jenkins

# Garantiza permisos de acceso al socket
RUN chmod 666 /var/run/docker.sock || true

USER jenkins