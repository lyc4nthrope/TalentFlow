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
                                echo "==> Build auth-service"
                                go mod download
                                go build ./...

                                echo "==> Publish auth-service"
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
                                echo "==> Build gestion-departamentos"
                                find . -name "*.php" -exec php -l {} +

                                echo "==> Publish gestion-departamentos"
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
                                echo "==> Build gestion-empleados"
                                npm ci
                                npm test || true

                                echo "==> Publish gestion-empleados"
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
                                echo "==> Build gestion-perfiles"
                                python -m pip install --upgrade pip
                                pip install -r requirements.txt
                                pytest || true

                                echo "==> Publish gestion-perfiles"
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
                                echo "==> Build gestion-vacaciones"
                                mvn clean compile -DskipTests

                                echo "==> Publish gestion-vacaciones"
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
                // Levantar el sistema completo
                sh 'docker compose -f docker-compose.yml up -d --build'
                
                // Esperar health checks de los servicios
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
                    // 1. Resultados máquina-legibles JUnit
                    junit 'e2e-tests/reports/junit.xml'
                    
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
                        alwaysLinkToLastBuild: true
                    ])
                    
                    // 4. Archivar logs para diagnóstico
                    sh 'docker compose logs --no-color > docker-logs.txt || true'
                    archiveArtifacts artifacts: 'docker-logs.txt', allowEmptyArchive: true
                    
                    // 5. Apagar y limpiar el entorno
                    sh 'docker compose -f docker-compose.yml down -v'
                }
            }
        }
    }
}