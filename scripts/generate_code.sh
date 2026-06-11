#!/bin/bash

SERVER_NAME=$1

# 1. 生成 pb 和 grpc/zrpc 代码
rm -rf ./pb
goctl rpc protoc --go_out=./pb --go-grpc_out=./pb --zrpc_out=. ./proto/${SERVER_NAME}.proto >> /dev/null
protoc --go_out=./pb --go-grpc_out=./pb ./proto/*.proto
rm -rf ./scripts/internal

# 2. 将 goctl 生成到 internal/logic/ 根目录的 grpc logic 文件移到 internal/logic/grpc/
#    若目标文件已存在（说明业务逻辑已填充），则跳过，避免覆盖
GRPC_LOGIC_DIR="./internal/logic/grpc"
mkdir -p "${GRPC_LOGIC_DIR}"

for f in ./internal/logic/*logic.go; do
  [ -f "$f" ] || continue
  filename=$(basename "$f")
  dest="${GRPC_LOGIC_DIR}/${filename}"
  if [ -f "${dest}" ]; then
    echo "skip (already exists): ${dest}"
    rm -f "$f"
  else
    # 修改 package 声明：package logic -> package grpc
    sed -i '' 's/^package logic$/package grpc/' "$f"
    mv "$f" "${dest}"
    echo "moved: $f -> ${dest}"
  fi
done

# 3. 修正 internal/server 中对 logic 包的 import 路径，指向 internal/logic/grpc
MODULE=$(head -1 ./go.mod | awk '{print $2}')
OLD_IMPORT="${MODULE}/internal/logic"
NEW_IMPORT="${MODULE}/internal/logic/grpc"

SERVER_FILE="./internal/server/${SERVER_NAME}server.go"
if [ -f "${SERVER_FILE}" ]; then
  sed -i '' "s|\"${OLD_IMPORT}\"|\"${NEW_IMPORT}\"|g" "${SERVER_FILE}"
  # 将 server 文件中 logic.NewXxx 调用改为 grpc.NewXxx
  sed -i '' 's/logic\.New/grpc.New/g' "${SERVER_FILE}"
  echo "updated imports in: ${SERVER_FILE}"
fi

# 4. 格式化
go fmt ./*.go >> /dev/null
go fmt ./internal/server/* >> /dev/null
