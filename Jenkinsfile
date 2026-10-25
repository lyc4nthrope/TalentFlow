pipeline {
    agent any

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
                    ls -la "${WORKSPACE}"
                    test -f "${WORKSPACE}/docker-compose.e2e.yml"
                    mkdir -p "${WORKSPACE}/e2e-tests/reports"

                    if docker compose version >/dev/null 2>&1; then
                        COMPOSE_CMD='docker compose'
                    elif command -v docker-compose >/dev/null 2>&1; then
                        COMPOSE_CMD='docker-compose'
                    else
                        echo "Docker Compose no está disponible dentro de Jenkins. Se omite la ejecución E2E."
                        exit 0
                    fi

                    echo "==> Levantando entorno E2E con: ${COMPOSE_CMD}"
                    sh -c "$COMPOSE_CMD -f \"${WORKSPACE}/docker-compose.e2e.yml\" up -d --build --remove-orphans"
                '''

                sh './scripts/wait-for-healthy.sh || true'

                sh '''
                    cd "${WORKSPACE}/e2e-tests"
                    npx cucumber-js \
                    --format junit:reports/junit.xml \
                    --format json:reports/cucumber.json \
                    --format html:reports/cucumber.html || true
                '''
            }
            post {
                always {
                    sh '''
                        set +e
                        mkdir -p "${WORKSPACE}/e2e-tests/reports"

                        if docker compose version >/dev/null 2>&1; then
                            COMPOSE_CMD='docker compose'
                        elif command -v docker-compose >/dev/null 2>&1; then
                            COMPOSE_CMD='docker-compose'
                        else
                            echo "No hay Docker Compose disponible en Jenkins." > "${WORKSPACE}/docker-logs.txt"
                            exit 0
                        fi

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

                    sh '''
                        set +e
                        if docker compose version >/dev/null 2>&1; then
                            COMPOSE_CMD='docker compose'
                        elif command -v docker-compose >/dev/null 2>&1; then
                            COMPOSE_CMD='docker-compose'
                        else
                            exit 0
                        fi

                        sh -c "$COMPOSE_CMD -f \"${WORKSPACE}/docker-compose.e2e.yml\" down --remove-orphans || true"
                    '''
                }
            }
        }
    }
}