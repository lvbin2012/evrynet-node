pipeline {
    agent { label "slave" }
    environment{
        branchName = sh(
            script: "echo ${env.GIT_BRANCH} | sed -e 's|/|-|g'",
            returnStdout: true
        ).trim()
        dockerTag="${env.branchName}-${env.BUILD_NUMBER}"
        dockerImage="${env.CONTAINER_IMAGE}:${env.dockerTag}"
        dockerOnenodeImage="${env.CONTAINER_IMAGE}:${env.dockerTag}-onenode"
        dockerOnenodeContainer="evrynet-onenode"
        appName="evrynet-node"
        githubUsername="Evrynetlabs"

        CONTAINER_IMAGE="registry.gitlab.com/evry/${appName}"
        status_failure="{\"state\": \"failure\",\"context\": \"continuous-integration/jenkins\", \"description\": \"Jenkins\", \"target_url\": \"${BUILD_URL}\"}"
        status_success="{\"state\": \"success\",\"context\": \"continuous-integration/jenkins\", \"description\": \"Jenkins\", \"target_url\": \"${BUILD_URL}\"}"
    }
    stages {
        stage ('Cleanup') {
            steps {
                sh '''
                    echo "memory usage"
                    free -m
                '''
                sh '''
                    docker images
                    free -m
                '''
                dir('directoryToDelete') {
                    deleteDir()
                }
            }
        }

        stage('Build Image Test') {
            steps {
                withCredentials([usernamePassword(credentialsId: 'devopsautomate', passwordVariable: 'gitlabPassword', usernameVariable: 'gitlabUsername')]) {
                    sh '''
                        echo "Build Image"
                        docker login -u ${gitlabUsername} -p ${gitlabPassword} registry.gitlab.com
                        docker build --pull --target builder -t ${dockerImage} -f Dockerfile .
                    '''
                }
            }
        }

        stage('Lint') {
            steps {
                sh '''
                    echo "Run lint -> ${dockerImage}"
                    docker run --rm ${dockerImage} sh -c "go run build/ci.go lint"
                '''
            }
        }

        stage('Unit Test') {
            steps {
                sh '''
                    echo "Run unit test -> ${dockerImage}"
                    docker run --rm ${dockerImage} sh -c "go run build/ci.go test"
                '''
            }
        }

        stage('Integration Test') {
            steps {
                sh '''
                    docker build . -f ./tests/onenode/Dockerfile -t ${dockerOnenodeImage}
                    docker run -d -p 22001:8545 -it --name ${dockerOnenodeContainer} ${dockerOnenodeImage}
                    docker run --network host --rm ${dockerImage} sh -c "go run build/ci.go test -integration -coverage"
                '''
            }
        }

        stage('Build and Push to Registry') {
            when {
                anyOf {
                    branch 'develop';
                    branch 'release/*';
                    branch 'master'
                }
            }
            steps {
                withCredentials([usernamePassword(credentialsId: 'devopsautomate', passwordVariable: 'gitlabPassword', usernameVariable: 'gitlabUsername')]) {
                    sh '''
                        echo "Push to Registry"
                        docker login -u ${gitlabUsername} -p ${gitlabPassword} registry.gitlab.com
                        docker build --pull -t ${dockerImage} -f Dockerfile .
                        docker push ${dockerImage}
                        docker tag ${dockerImage} ${CONTAINER_IMAGE}:${branchName}
                        docker push ${CONTAINER_IMAGE}:${branchName}
                    '''
                }
            }
        }
    }
    post {
        failure {
            withCredentials([string(credentialsId: 'evry-github-token-pipeline-status', variable: 'githubToken')]) {
                sh '''
                    curl \"https://api.github.com/repos/${githubUsername}/${appName}/statuses/${GIT_COMMIT}?access_token=${githubToken}\" \
                    -H \"Content-Type: application/json\" \
                    -X POST \
                    -d "${status_failure}"
                '''
                }
        }
        success {
            withCredentials([string(credentialsId: 'evry-github-token-pipeline-status', variable: 'githubToken')]) {
                sh '''
                    curl \"https://api.github.com/repos/${githubUsername}/${appName}/statuses/${GIT_COMMIT}?access_token=${githubToken}\" \
                    -H \"Content-Type: application/json\" \
                    -X POST \
                    -d "${status_success}"
                '''
                }
        }
        always {
            sh '''
               docker image rm -f ${CONTAINER_IMAGE}:${branchName}
               docker image rm -f ${dockerImage}
               docker image rm -f ${dockerOnenodeImage}
            '''
            // remove intermediate images when multi-stage build
            sh '''
                docker image prune -f
                docker image ls
            '''
            // stop and remove one node container
            sh '''
                docker stop ${dockerOnenodeContainer}
                docker rm ${dockerOnenodeContainer}
            '''
            deleteDir()
        }
    }
}
