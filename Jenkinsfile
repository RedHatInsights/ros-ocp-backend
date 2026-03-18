def secrets = [
    [path: 'insights-cicd/ephemeral-bot-svc-account', secretValues: [
        [envVar: 'OC_LOGIN_TOKEN_DEV', vaultKey: 'oc-login-token-dev'],
        [envVar: 'OC_LOGIN_SERVER_DEV', vaultKey: 'oc-login-server-dev'],
        [envVar: 'OC_LOGIN_TOKEN', vaultKey: 'oc-login-token'],
        [envVar: 'OC_LOGIN_SERVER', vaultKey: 'oc-login-server']]],
    [path: 'app-sre/quay/cloudservices-push', secretValues: [
        [envVar: 'QUAY_USER', vaultKey: 'user'],
        [envVar: 'QUAY_TOKEN', vaultKey: 'token']]],
    [path: 'insights-cicd/insightsdroid-github', secretValues: [
        [envVar: 'GITHUB_TOKEN', vaultKey: 'token'],
        [envVar: 'GITHUB_API_URL', vaultKey: 'mirror_url']]],
    [path: 'insights-cicd/rh-registry-pull', secretValues: [
        [envVar: 'RH_REGISTRY_USER', vaultKey: 'user'],
        [envVar: 'RH_REGISTRY_TOKEN', vaultKey: 'token']]]
]

def configuration = [vaultUrl: params.VAULT_ADDRESS, vaultCredentialId: params.VAULT_CREDS_ID]

pipeline {
    agent {
        node {
            label 'rhel8-spot'
        }
    }
    
    options {
        timestamps()
        timeout(time: 150, unit: 'MINUTES')
    }
    
    environment {
        APP_NAME = "ros-ocp"
    }
    
    stages {
        stage('PR Check') {
            when {
                changeRequest()
            }
            steps {
                withVault([configuration: configuration, vaultSecrets: secrets]) {
                    sh 'bash pr_check.sh'
                }
            }
        }
        
        stage('Build and Deploy') {
            when {
                not { changeRequest() }
            }
            steps {
                withVault([configuration: configuration, vaultSecrets: secrets]) {
                    sh 'bash build_deploy.sh'
                }
            }
        }
    }
    
    post {
        always {
            archiveArtifacts artifacts: 'artifacts/**/*', fingerprint: true, allowEmptyArchive: true
            cleanWs()
        }
    }
}
