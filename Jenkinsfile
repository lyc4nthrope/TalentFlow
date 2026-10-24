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
                // Crear directorio de reportes
                sh 'mkdir -p e2e-tests/reports'

                // Usar contenedor oficial de Docker (compatible con ARM64 / Mac)
                sh '''
                    docker run --rm \
                      -v /var/run/docker.sock:/var/run/docker.sock \
                      -v "$PWD":/app \
                      -w /app \
                      docker:latest compose -f docker-compose.yml up -d --build
                '''
                
                // Esperar health checks
                sh './scripts/wait-for-healthy.sh'
                
                // Ejecutar la suite BDD
                sh '''
                    cd e2e-tests
                    npx cucumber-js \
                    --format junit:reports/junit.xml \
                    --format json:reports/cucumber.json \
                    --format html:reports/cucumber.html || true
                '''
            }
            post {
                always {
                    // Garantizar directorio de reportes
                    sh 'mkdir -p e2e-tests/reports'

                    // 1. Resultados máquina-legibles JUnit
                    junit allowEmptyResults: true, testResults: 'e2e-tests/reports/junit.xml'
                    
                    // 2. Reporte navegable BDD Cucumber
                    cucumber(
                        jsonReportDirectory: 'e2e-tests/reports',
                        fileIncludePattern: '**/cucumber.json',
                        buildStatus: 'UNSTABLE'
                    )
                    
                    // 3. Reporte HTML genérico
                    publishHTML(target: [
                        reportDir: 'e2e-tests/reports',
                        reportFiles: 'cucumber.html',
                        reportName: 'Reporte BDD',
                        keepAll: true,
                        alwaysLinkToLastBuild: true,
                        allowMissing: true
                    ])
                    
                    // 4. Archivar logs para diagnóstico
                    sh '''
                        docker run --rm \
                          -v /var/run/docker.sock:/var/run/docker.sock \
                          -v "$PWD":/app \
                          -w /app \
                          docker:latest compose -f docker-compose.yml logs > docker-logs.txt || true
                    '''
                    archiveArtifacts artifacts: 'docker-logs.txt', allowEmptyArchive: true
                    
                    // 5. Apagar y limpiar el entorno
                    sh '''
                        docker run --rm \
                          -v /var/run/docker.sock:/var/run/docker.sock \
                          -v "$PWD":/app \
                          -w /app \
                          docker:latest compose -f docker-compose.yml down -v || true
                    '''
                }
            }
        }
    }
}