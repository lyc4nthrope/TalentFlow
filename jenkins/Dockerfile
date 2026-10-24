FROM jenkins/jenkins:lts-jdk17

# Instalar plugins de Jenkins necesarios
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

# 1. Instalar Docker CLI, Node.js y dependencias
RUN apt-get update && \
    apt-get install -y \
        docker.io \
        nodejs \
        npm \
        curl \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# 2. Instalar Docker Compose (v2.29.1)
RUN mkdir -p /usr/libexec/docker/cli-plugins && \
    curl -SL "https://github.com/docker/compose/releases/download/v2.29.1/docker-compose-linux-$(uname -m)" \
    -o /usr/libexec/docker/cli-plugins/docker-compose && \
    chmod +x /usr/libexec/docker/cli-plugins/docker-compose && \
    ln -s /usr/libexec/docker/cli-plugins/docker-compose /usr/local/bin/docker-compose || true

# 3. Instalar Docker Buildx actualizado (v0.19.1 o superior)
RUN curl -SL "https://github.com/docker/buildx/releases/download/v0.19.1/buildx-v0.19.1.linux-$(uname -m)" \
    -o /usr/libexec/docker/cli-plugins/docker-buildx && \
    chmod +x /usr/libexec/docker/cli-plugins/docker-buildx

# 4. Asignar usuario al grupo docker
RUN groupadd -f docker && usermod -aG docker jenkins

USER jenkins