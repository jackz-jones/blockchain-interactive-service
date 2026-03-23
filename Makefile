BUILD_TIME := $(shell date "+%F %T")
COMMIT_SHA1 := $(shell git rev-parse HEAD )

# 版本手动指定
VERSION=v1.1.0
LDFLAGS := -ldflags "-X 'github.com/jackz-jones/blockchain-interactive-service/internal.BuildTime=${BUILD_TIME}'  -X 'github.com/jackz-jones/blockchain-interactive-service/internal.CommitID=${COMMIT_SHA1}'  -X 'github.com/jackz-jones/blockchain-interactive-service/internal.Version=${VERSION}'"
SOURCE := ./chaininteractive.go
BUILD_NAME := chain-interactive-service

IMAGE=192.168.1.2:5000/chain-interactive-service
REV=$(shell git rev-parse --short HEAD)
REPO=${IMAGE}-${REV}:${VERSION}
TAG=192.168.1.2:5000/chain-interactive-service:${VERSION}

.PHONY:gen-code start-service build

gen-code:
	@sh ./scripts/generate_code.sh chaininteractive

start-service:
	go run ${SOURCE}

build:
	go build ${LDFLAGS} -o ${BUILD_NAME} ${SOURCE}

build-docker:

	# ida 定制版本的 sdk
	go get chainmaker.org/chainmaker/sdk-go/v2@v2.3.8
	go mod tidy

	# 1. 安装下载modvendor程序
	go install github.com/goware/modvendor@v0.5.0

	# 2. 将依赖包导入到自己的项目根目录下
	GO111MODULE=on go mod vendor

	# 3. 使用modvendor将vendor下第三方依赖的.c/.h/.a文件copy到vendor对应目录下
	modvendor -copy="**/*.c **/*.h **/*.a" -v

	docker build -t ${REPO} -f ./docker/Dockerfile .
	docker tag ${REPO} ${TAG}
	docker push ${REPO}
	docker push ${TAG}

push-docker:
	docker push ${REPO}

gen-cert:
	bash ./scripts/generate_cert.sh ./cert ./scripts

gen-mock:
	mockgen -source=chaininteractive/chaininteractive.go -destination=mock/mock_chain_interactive.go -package=mock

ut:
	#cd scripts && ./ut_cover.sh
	go test -coverprofile cover.out ./...
	@echo "\n"
	@echo "综合UT覆盖率：" `go tool cover -func=cover.out | tail -1  | grep -P '\d+\.\d+(?=\%)' -o`
	@echo "\n"

lint:
	golangci-lint run ./...

comment:
	gocloc --include-lang=Go --output-type=json --not-match=".*_test.go|types.go" --not-match-d="cert|chain|mock|pb|internal/code|internal/errors|internal/server|internal/model" . | jq '(.total.comment-.total.files*5)/(.total.code+.total.comment)*100'

pre-commit: lint ut comment

update-mod:
	go get github.com/ethereum/go-ethereum@v1.14.11
	go get chainmaker.org/chainmaker/sdk-go/v2@v2.3.8
	go get github.com/jackz-jones/common@dev
	go mod tidy