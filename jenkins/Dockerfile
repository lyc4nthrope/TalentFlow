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

# 1. Instalar herramientas base, Docker CLI, Node.js, Maven, PHP, Go, Python 3 y Java JRE
RUN apt-get update && \
    apt-get install -y \
        docker.io \
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
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# 2. Instalar Docker Compose (v2.29.1)
RUN mkdir -p /usr/libexec/docker/cli-plugins && \
    curl -SL "https://github.com/docker/compose/releases/download/v2.29.1/docker-compose-linux-$(uname -m)" \
    -o /usr/libexec/docker/cli-plugins/docker-compose && \
    chmod +x /usr/libexec/docker/cli-plugins/docker-compose && \
    ln -s /usr/libexec/docker/cli-plugins/docker-compose /usr/local/bin/docker-compose || true

# 3. Instalar Docker Buildx (v0.19.1)
RUN curl -SL "https://github.com/docker/buildx/releases/download/v0.19.1/buildx-v0.19.1.linux-$(uname -m)" \
    -o /usr/libexec/docker/cli-plugins/docker-buildx && \
    chmod +x /usr/libexec/docker/cli-plugins/docker-buildx

# 4. Instalar PHPUnit standalone
RUN curl -sSL https://phar.phpunit.de/phpunit-10.phar -o /usr/local/bin/phpunit && \
    chmod +x /usr/local/bin/phpunit

# 5. Instalar SonarScanner CLI nativo
RUN curl -sSLo /tmp/sonar-scanner.zip https://binaries.sonarsource.com/Distribution/sonar-scanner-cli/sonar-scanner-cli-5.0.1.3006.zip && \
    unzip -q /tmp/sonar-scanner.zip -d /opt/ && \
    mv /opt/sonar-scanner-5.0.1.3006 /opt/sonar-scanner && \
    ln -s /opt/sonar-scanner/bin/sonar-scanner /usr/local/bin/sonar-scanner && \
    rm /tmp/sonar-scanner.zip

# 6. Instalar herramientas globales de Go para reportes
ENV GOPATH=/opt/go
ENV PATH=$PATH:$GOPATH/bin
RUN mkdir -p $GOPATH && \
    GOSUMDB=off go install github.com/jstemmer/go-junit-report/v2@latest && \
    GOSUMDB=off go install github.com/boumenot/gocover-cobertura@v1.2.0 && \
    chmod -R 777 $GOPATH

# 7. Asignar usuario al grupo docker
RUN groupadd -f docker && usermod -aG docker jenkins

USER jenkins