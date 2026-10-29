// Pipeline orquestador (Reto 7, Punto 4): construye y versiona las imágenes de los siete
// microservicios, despliega el sistema completo en un proyecto de Compose aislado, corre
// la suite BDD del Reto 6 contra él, publica los reportes y limpia solo lo que creó.
//
// Checkout → Build & Package → Publish → Deploy → E2E Tests → (post) reportes, logs, limpieza
//
// Aislamiento: cada build usa el proyecto `talentflow-ci-<build>` y el override
// docker-compose.ci-e2e.yml (sin container_name ni puertos en el host), así que no choca
// con el sistema de desarrollo de la misma máquina ni con otro build.
// El workspace está en la misma ruta en el host y en el contenedor (D16): los bind mounts
// de Compose (scripts SQL, e2e-tests/reports) resuelven bien con `agent any`.

// Servicio → [contexto de build, Dockerfile]. Empleados usa la raíz como contexto
// (necesita el package-lock.json de los workspaces), igual que docker-compose.yml.
servicios = [
    'api-gateway'          : ['microservicios/api-gateway',          'microservicios/api-gateway/Dockerfile'],
    'auth-service'         : ['microservicios/auth-service',         'microservicios/auth-service/Dockerfile'],
    'gestion-departamentos': ['microservicios/gestion-departamentos', 'microservicios/gestion-departamentos/Dockerfile'],
    'gestion-empleados'    : ['.',                                   'microservicios/gestion-empleados/Dockerfile'],
    'gestion-perfiles'     : ['microservicios/gestion-perfiles',     'microservicios/gestion-perfiles/Dockerfile'],
    'gestion-vacaciones'   : ['microservicios/gestion-vacaciones',   'microservicios/gestion-vacaciones/Dockerfile'],
    'notificaciones'       : ['microservicios/notificaciones',       'microservicios/notificaciones/Dockerfile'],
]

pipeline {
    agent any

    options {
        skipDefaultCheckout()
        timeout(time: 60, unit: 'MINUTES')
    }

    environment {
        // El push lo hace el daemon del host: para él, el registro es localhost:5000.
        REGISTRY_URL = "${env.REGISTRY_URL ?: 'localhost:5000'}"
        // docker-compose.ci-e2e.yml despliega ${REGISTRY_URL}/<servicio>:${IMAGE_TAG}.
        IMAGE_TAG    = "${env.BUILD_NUMBER}"
        PROYECTO     = "talentflow-ci-${env.BUILD_NUMBER}"
        COMPOSE      = "docker compose -p talentflow-ci-${env.BUILD_NUMBER} -f docker-compose.yml -f docker-compose.e2e.yml -f docker-compose.ci-e2e.yml"
        // La copia aislada no levanta Jaeger (perfil "observabilidad"): sin esto, cada servicio
        // intentaría enviarle trazas y llenaría los logs de errores de conexión (Reto 8).
        OTEL_SDK_DISABLED = 'true'
        REPORTES     = 'e2e-tests/reports'
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Build & Package') {
            steps {
                script {
                    parallel servicios.collectEntries { nombre, build ->
                        [(nombre): {
                            sh "docker build -t ${REGISTRY_URL}/${nombre}:${IMAGE_TAG} -t ${REGISTRY_URL}/${nombre}:latest -f ${build[1]} ${build[0]}"
                        }]
                    }
                }
            }
        }

        stage('Publish') {
            steps {
                script {
                    parallel servicios.collectEntries { nombre, build ->
                        [(nombre): {
                            sh "docker push ${REGISTRY_URL}/${nombre}:${IMAGE_TAG}"
                            sh "docker push ${REGISTRY_URL}/${nombre}:latest"
                        }]
                    }
                }
            }
        }

        stage('Deploy') {
            steps {
                // Reportes limpios por build. La carpeta se crea antes del montaje; la suite
                // corre con el uid/gid de Jenkins (root en su contenedor) para poder escribirla.
                sh '''
                    rm -rf "$REPORTES" && mkdir -p "$REPORTES"
                    export E2E_UID=$(id -u) E2E_GID=$(id -g)
                    $COMPOSE up -d
                    # Polling con tope (scripts/esperar-sistema.js) hasta que el Gateway informe
                    # sistema=OK; `timeout` acota además la espera de los healthchecks.
                    timeout 600 $COMPOSE run --rm bdd-tests npm run --silent esperar-sistema
                '''
            }
        }

        stage('E2E Tests') {
            steps {
                // Sin `|| true`: un escenario que falla hace fallar el pipeline (D12).
                sh '''
                    export E2E_UID=$(id -u) E2E_GID=$(id -g)
                    $COMPOSE run --rm bdd-tests
                '''
            }
            post {
                always {
                    // Cada publicación por separado: si una falla, las demás y la limpieza siguen.
                    catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                        junit "${REPORTES}/junit.xml"
                    }
                    catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                        cucumber jsonReportDirectory: "${REPORTES}", fileIncludePattern: 'cucumber.json'
                    }
                    catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                        publishHTML(target: [
                            reportDir            : "${REPORTES}",
                            reportFiles          : 'cucumber.html',
                            reportName           : 'Reporte BDD',
                            keepAll              : true,
                            alwaysLinkToLastBuild: true,
                            allowMissing         : false,
                        ])
                    }
                }
            }
        }
    }

    post {
        // Corre aunque falle cualquier etapa. Los errores de limpieza se registran sin
        // tocar el resultado del build; solo se borra lo del proyecto de este build.
        always {
            sh '$COMPOSE logs --no-color --timestamps > docker-logs.txt 2>&1 || echo "AVISO: no se pudieron leer los logs de $PROYECTO"'
            archiveArtifacts artifacts: 'docker-logs.txt', allowEmptyArchive: true
            sh '''
                $COMPOSE --profile e2e down -v --remove-orphans --rmi local \
                    || echo "AVISO: falló el down de $PROYECTO"
            '''
            script {
                // Las imágenes versionadas ya están en el registro; localmente se quita solo
                // la etiqueta de este build (:latest queda como caché del siguiente).
                def etiquetas = servicios.keySet().collect { "${REGISTRY_URL}/${it}:${IMAGE_TAG}" }.join(' ')
                sh "docker image rm ${etiquetas} || echo 'AVISO: no se pudieron quitar las etiquetas :${IMAGE_TAG}'"
            }
        }
    }
}
