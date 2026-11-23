pipeline {
    agent {
        docker {
            image 'casjaysdev/go@sha256:0db19a251249c5aca053d4c25a7b832cdc66422bdb10067a8b072f15a785330a'
            args '--user 0:0'
        }
    }

    options {
        disableConcurrentBuilds()
        timeout(time: 60, unit: 'MINUTES')
    }

    environment {
        CGO_ENABLED = '0'
        GOFLAGS = '-buildvcs=false'
    }

    triggers {
        cron('0 4 1 * *')
    }

    stages {
        stage('Build') {
            steps {
                sh 'go build -trimpath ./...'
            }
        }

        stage('Test') {
            steps {
                sh '''
                    mkdir -p /tmp/apimgr
                    COVDIR=$(mktemp -d /tmp/apimgr/anime-XXXXXX)
                    trap 'rm -rf "$COVDIR"' EXIT
                    go vet ./...
                    go test -timeout 5m -covermode=atomic -coverprofile="$COVDIR/coverage.out" ./...
                    coverage=$(go tool cover -func="$COVDIR/coverage.out" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')
                    awk -v coverage="$coverage" 'BEGIN { if (coverage+0 < 60) exit 1 }'
                '''
            }
        }

        stage('Security') {
            parallel {
                stage('Secret Scan') {
                    steps {
                        script {
                            docker.image('trufflesecurity/trufflehog@sha256:8fd07ea46a3f59583c4d50f006d8cf150530141b28969c3c34cc455cb9165a6b').inside {
                                sh 'trufflehog git file://"$PWD" --results=verified --exclude-paths=.github/trufflehog-exclude.txt --fail'
                            }
                        }
                    }
                }
                stage('Vulnerability Scan') {
                    steps {
                        sh 'govulncheck ./...'
                    }
                }
                stage('Workflow Policy') {
                    steps {
                        sh '''
                            if grep -RnE 'pull_request_target' .github/ .gitea/ .forgejo/ --exclude=ci.yml 2>/dev/null; then
                                echo 'ERROR: pull_request_target is forbidden for repository workflows'
                                exit 1
                            fi
                            bad=$(grep -RnE '^[[:space:]]*uses:' .github/ .gitea/ .forgejo/ 2>/dev/null | grep -vE '@[0-9a-f]{40}([[:space:]]|$)' || true)
                            if [ -n "$bad" ]; then
                                printf '%s\n' "$bad"
                                exit 1
                            fi
                        '''
                    }
                }
                stage('Image Scan') {
                    steps {
                        script {
                            docker.build('local/scan-target:ci', '-f docker/Dockerfile .')
                            docker.image('aquasec/trivy:0.70.0@sha256:be1190afcb28352bfddc4ddeb71470835d16462af68d310f9f4bca710961a41e').inside('-v /var/run/docker.sock:/var/run/docker.sock') {
                                sh 'trivy image --exit-code 1 --severity CRITICAL,HIGH local/scan-target:ci'
                            }
                        }
                    }
                }
            }
        }

        stage('Release') {
            when {
                tag 'v*'
            }
            steps {
                sh '''
                    VERSION=${TAG_NAME#v}
                    mkdir -p releases
                    for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64 freebsd/arm64; do
                        GOOS=${platform%/*}
                        GOARCH=${platform#*/}
                        extension=
                        if [ "$GOOS" = windows ]; then extension=.exe; fi
                        CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -trimpath -ldflags "-s -w -X main.Version=$VERSION" -o "releases/anime-$GOOS-$GOARCH$extension" ./src
                        CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -trimpath -ldflags "-s -w -X main.Version=$VERSION" -o "releases/anime-cli-$GOOS-$GOARCH$extension" ./src/client
                    done
                    printf '%s\n' "$VERSION" > releases/version.txt
                    tar --exclude=.git --exclude=.github --exclude=.gitea --exclude=.forgejo --exclude=releases --exclude='*.tar.gz' -czf "releases/anime-$VERSION-source.tar.gz" .
                    cyclonedx-gomod mod -json -output "releases/anime-$VERSION-sbom.cdx.json"
                    (cd releases && sha256sum * > sha256.txt && sha512sum * > sha512.txt)
                '''
                archiveArtifacts artifacts: 'releases/*', fingerprint: true
            }
        }
    }
}
