# LTS fijada con Java 21: la línea jdk17 dejó de recibir soporte y la 2.541.3 tiene avisos
# de seguridad. Imagen multiarquitectura (linux/amd64 y linux/arm64).
FROM jenkins/jenkins:2.580.1-lts-jdk21

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

# 1. Instalar herramientas base, Node.js, Maven, PHP, Go, Python 3 y Java JRE
RUN apt-get update && \
    apt-get install -y \
        nodejs \
        npm \
        maven \
        golang-go \
        php-cli \
        php-mbstring \
        php-xml \
        python3 \
        python3-venv \
        python3-pip \
        default-jre \
        unzip \
        git \
        curl \
        gnupg \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# 2. Docker CLI, Buildx y Compose desde el repositorio apt de Docker: apt elige el binario
#    de la arquitectura de la máquina (amd64 en Linux, arm64 en Mac con Apple Silicon), así
#    no hay descargas con nombres por arquitectura que se puedan equivocar. Versiones fijadas.
ARG DOCKER_CLI_VERSION=5:29.9.2-1~debian.13~trixie
ARG BUILDX_VERSION=0.38.1-1~debian.13~trixie
ARG COMPOSE_VERSION=5.6.1-1~debian.13~trixie
RUN install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
        > /etc/apt/sources.list.d/docker.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        "docker-ce-cli=${DOCKER_CLI_VERSION}" \
        "docker-buildx-plugin=${BUILDX_VERSION}" \
        "docker-compose-plugin=${COMPOSE_VERSION}" && \
    rm -rf /var/lib/apt/lists/* && \
    ln -s /usr/libexec/docker/cli-plugins/docker-compose /usr/local/bin/docker-compose && \
    docker buildx version && docker compose version

# 3. Instalar PHPUnit standalone
RUN curl -sSL https://phar.phpunit.de/phpunit-10.phar -o /usr/local/bin/phpunit && \
    chmod +x /usr/local/bin/phpunit

# 4. Instalar SonarScanner CLI nativo (zip sin binarios nativos: usa el Java de la imagen)
RUN curl -sSLo /tmp/sonar-scanner.zip https://binaries.sonarsource.com/Distribution/sonar-scanner-cli/sonar-scanner-cli-5.0.1.3006.zip && \
    unzip -q /tmp/sonar-scanner.zip -d /opt/ && \
    mv /opt/sonar-scanner-5.0.1.3006 /opt/sonar-scanner && \
    ln -s /opt/sonar-scanner/bin/sonar-scanner /usr/local/bin/sonar-scanner && \
    rm /tmp/sonar-scanner.zip

# 5. Instalar herramientas globales de Go para reportes
ENV GOPATH=/opt/go
ENV PATH=$PATH:$GOPATH/bin
RUN mkdir -p $GOPATH && \
    GOSUMDB=off go install github.com/jstemmer/go-junit-report/v2@v2.1.0 && \
    GOSUMDB=off go install github.com/boumenot/gocover-cobertura@v1.2.0 && \
    chmod -R 777 $GOPATH

# 6. Node.js de la tabla de herramientas del README (24.14.0). El de apt (20.x) no
#    reconoce --test-coverage-include ni los umbrales que usa `npm test` (Reto 6), así
#    que el Test de empleados y del Gateway fallaría. Se instala en /usr/local, que va
#    antes que /usr/bin en el PATH. x64 o arm64 según la máquina (Linux o Mac).
ARG NODE_VERSION=24.14.0
RUN case "$(dpkg --print-architecture)" in amd64) ARCH=x64 ;; arm64) ARCH=arm64 ;; *) echo "arquitectura no soportada" && exit 1 ;; esac && \
    curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${ARCH}.tar.xz" \
    | tar -xJ -C /usr/local --strip-components=1 --exclude='*.md' --exclude=LICENSE && \
    node --version

# 7. Asignar usuario al grupo docker
RUN groupadd -f docker && usermod -aG docker jenkins

USER jenkins