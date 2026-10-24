FROM jenkins/jenkins:lts-jdk17

# Instalar plugins necesarios de forma automatizada (incluye reportes de pruebas)
RUN jenkins-plugin-cli --plugins \
    docker-workflow \
    pipeline-stage-view \
    workflow-aggregator \
    git \
    configuration-as-code \
    job-dsl \
    sonar \
    junit \
    coverage \
    cucumber-reports \
    htmlpublisher

USER root

# Instalar Docker CLI, Plugin de Docker Compose y Node.js/npm (para npx)
RUN apt-get update && \
    apt-get install -y \
        docker.io \
        docker-compose-plugin \
        nodejs \
        npm \
        curl \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Agregar usuario jenkins al grupo docker para acceder al socket
RUN groupadd -f docker && usermod -aG docker jenkins

USER jenkins