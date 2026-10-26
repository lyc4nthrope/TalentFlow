pipeline {
    agent {
        node {
            label ''
            customWorkspace '/Users/valeria/Documents/GitHub/TalentFlow/.jenkins-workspace'
        }
    }

    environment {
        REGISTRY  = "localhost:5000"
        IMAGE_TAG = "${env.BUILD_NUMBER}"
    }

    stages {
        stage('Checkout') {
            steps {
                echo 'Obteniendo código del repositorio...'
                checkout scm
            }
        }

        stage('Build & Publish Microservicios') {
            parallel {
                stage('auth-service') {
                    steps {
                        dir('microservicios/auth-service') {
                            sh """
                                set -e
                                echo "==> Building & Publishing auth-service"
                                docker build -t ${REGISTRY}/auth-service:${IMAGE_TAG} -t ${REGISTRY}/auth-service:latest .
                                docker push ${REGISTRY}/auth-service:${IMAGE_TAG}
                                docker push ${REGISTRY}/auth-service:latest
                            """
                        }
                    }
                }

                stage('gestion-departamentos') {
                    steps {
                        dir('microservicios/gestion-departamentos') {
                            sh """
                                set -e
                                echo "==> Building & Publishing gestion-departamentos"
                                docker build -t ${REGISTRY}/gestion-departamentos:${IMAGE_TAG} -t ${REGISTRY}/gestion-departamentos:latest .
                                docker push ${REGISTRY}/gestion-departamentos:${IMAGE_TAG}
                                docker push ${REGISTRY}/gestion-departamentos:latest
                            """
                        }
                    }
                }

                stage('gestion-empleados') {
                    steps {
                        dir('microservicios/gestion-empleados') {
                            sh """
                                set -e
                                echo "==> Building & Publishing gestion-empleados"
                                docker build -t ${REGISTRY}/gestion-empleados:${IMAGE_TAG} -t ${REGISTRY}/gestion-empleados:latest .
                                docker push ${REGISTRY}/gestion-empleados:${IMAGE_TAG}
                                docker push ${REGISTRY}/gestion-empleados:latest
                            """
                        }
                    }
                }

                stage('gestion-perfiles') {
                    steps {
                        dir('microservicios/gestion-perfiles') {
                            sh """
                                set -e
                                echo "==> Building & Publishing gestion-perfiles"
                                docker build -t ${REGISTRY}/gestion-perfiles:${IMAGE_TAG} -t ${REGISTRY}/gestion-perfiles:latest .
                                docker push ${REGISTRY}/gestion-perfiles:${IMAGE_TAG}
                                docker push ${REGISTRY}/gestion-perfiles:latest
                            """
                        }
                    }
                }

                stage('gestion-vacaciones') {
                    steps {
                        dir('microservicios/gestion-vacaciones') {
                            sh """
                                set -e
                                echo "==> Building & Publishing gestion-vacaciones"
                                docker build -t ${REGISTRY}/gestion-vacaciones:${IMAGE_TAG} -t ${REGISTRY}/gestion-vacaciones:latest .
                                docker push ${REGISTRY}/gestion-vacaciones:${IMAGE_TAG}
                                docker push ${REGISTRY}/gestion-vacaciones:latest
                            """
                        }
                    }
                }
            }
        }
        stage('E2E Tests') {
            steps {
                sh '''
                    set -e
                    echo "WORKSPACE=${WORKSPACE}"
                    test -f "${WORKSPACE}/docker-compose.e2e.yml"

                    # Limpiar reportes
                    rm -rf "${WORKSPACE}/e2e-tests/reports"
                    mkdir -p "${WORKSPACE}/e2e-tests/reports"
                    chmod -R 777 "${WORKSPACE}/e2e-tests/reports"

                    # Elegir docker compose v2 o legacy
                    if docker compose version >/dev/null 2>&1; then
                        COMPOSE_CMD='docker compose'
                    elif command -v docker-compose >/dev/null 2>&1; then
                        COMPOSE_CMD='docker-compose'
                    else
                        echo "Docker Compose no está disponible."
                        exit 1
                    fi

                    echo "==> Limpiando estado previo para evitar volúmenes stale del broker y BD..."
                    sh -c "$COMPOSE_CMD -f \"${WORKSPACE}/docker-compose.e2e.yml\" down -v --remove-orphans --rmi local || true"

                    echo "==> Levantando infraestructura E2E..."
                    sh -c "$COMPOSE_CMD -f \"${WORKSPACE}/docker-compose.e2e.yml\" up -d --build --remove-orphans"

                    echo "==> Esperando a que RabbitMQ termine la importación de la topología..."
                    sh -c "$COMPOSE_CMD -f \"${WORKSPACE}/docker-compose.e2e.yml\" wait broker-init || true"

                    echo "==> Verificando que la infraestructura esté lista antes del BDD..."
                    sh -c "$COMPOSE_CMD -f \"${WORKSPACE}/docker-compose.e2e.yml\" ps"

                    echo "==> Ejecutando suite BDD dentro de su contenedor..."
                    sh -c "$COMPOSE_CMD -f \"${WORKSPACE}/docker-compose.e2e.yml\" run --rm bdd-tests || true"
                '''
            }
            post {
                always {
                    sh '''
                        set +e
                        mkdir -p "${WORKSPACE}/e2e-tests/reports"
                        chmod -R 777 "${WORKSPACE}/e2e-tests/reports"

                        if docker compose version >/dev/null 2>&1; then
                            COMPOSE_CMD='docker compose'
                        else
                            COMPOSE_CMD='docker-compose'
                        fi

                        # Guardar logs antes del apagado
                        sh -c "$COMPOSE_CMD -f \"${WORKSPACE}/docker-compose.e2e.yml\" logs > \"${WORKSPACE}/docker-logs.txt\" || true"
                    '''

                    junit allowEmptyResults: true, testResults: 'e2e-tests/reports/junit.xml'

                    cucumber(
                        jsonReportDirectory: 'e2e-tests/reports',
                        fileIncludePattern: '**/cucumber.json',
                        buildStatus: 'UNSTABLE'
                    )

                    publishHTML(target: [
                        reportDir: 'e2e-tests/reports',
                        reportFiles: 'cucumber.html',
                        reportName: 'Reporte BDD',
                        keepAll: true,
                        alwaysLinkToLastBuild: true,
                        allowMissing: true
                    ])

                    archiveArtifacts artifacts: 'docker-logs.txt', allowEmptyArchive: true

                    // Limpieza agresiva post-ejecución
                    sh '''
                        set +e
                        if docker compose version >/dev/null 2>&1; then
                            COMPOSE_CMD='docker compose'
                        else
                            COMPOSE_CMD='docker-compose'
                        fi

                        echo "==> Apagando contenedores, borrando volúmenes e imágenes locales del compose..."
                        sh -c "$COMPOSE_CMD -f \"${WORKSPACE}/docker-compose.e2e.yml\" down -v --remove-orphans --rmi local || true"

                        echo "==> Eliminando imágenes colgadas (dangling) y liberando caché de Docker..."
                        docker image prune -f || true
                        docker volume prune -f || true
                    '''
                }
            }
        }
    }
}