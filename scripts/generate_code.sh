#!/bin/bash

set -e

SERVER_NAME=$1
MODULE=$(head -1 ./go.mod | awk '{print $2}')
BACKUP_DIR="./_gen_backup_$$"

# 辅助函数：输出步骤标题
step() { echo ""; echo "===== $1 ====="; }

# 清理备份目录
cleanup() {
  if [ -d "${BACKUP_DIR}" ]; then
    rm -rf "${BACKUP_DIR}"
  fi
}
trap cleanup EXIT

# ============================================================
# 第一部分：gRPC 代码生成（保持原有逻辑不变）
# ============================================================

step "1/8 生成 gRPC 代码 (proto → pb + zrpc)"

rm -rf ./pb
goctl rpc protoc --go_out=./pb --go-grpc_out=./pb --zrpc_out=. ./proto/${SERVER_NAME}.proto >> /dev/null
protoc --go_out=./pb --go-grpc_out=./pb ./proto/*.proto
rm -rf ./scripts/internal
echo "  gRPC 代码生成完成"

step "2/8 迁移 gRPC logic → internal/logic/grpc/"

GRPC_LOGIC_DIR="./internal/logic/grpc"
mkdir -p "${GRPC_LOGIC_DIR}"

for f in ./internal/logic/*logic.go; do
  [ -f "$f" ] || continue
  filename=$(basename "$f")
  dest="${GRPC_LOGIC_DIR}/${filename}"
  if [ -f "${dest}" ]; then
    echo "  skip (already exists): ${dest}"
    rm -f "$f"
  else
    sed -i '' 's/^package logic$/package grpc/' "$f"
    mv "$f" "${dest}"
    echo "  moved: $f -> ${dest}"
  fi
done

step "3/8 修正 gRPC server import 路径"

OLD_IMPORT="${MODULE}/internal/logic"
NEW_IMPORT="${MODULE}/internal/logic/grpc"

SERVER_FILE="./internal/server/${SERVER_NAME}server.go"
if [ -f "${SERVER_FILE}" ]; then
  sed -i '' "s|\"${OLD_IMPORT}\"|\"${NEW_IMPORT}\"|g" "${SERVER_FILE}"
  sed -i '' 's/logic\.New/grpc.New/g' "${SERVER_FILE}"
  echo "  updated imports in: ${SERVER_FILE}"
fi

# ============================================================
# 第二部分：HTTP API 代码生成
# ============================================================

API_FILE="./api/${SERVER_NAME}.api"
if [ ! -f "${API_FILE}" ]; then
  echo "  WARNING: ${API_FILE} 不存在，跳过 HTTP API 代码生成"
  echo ""
  echo "===== 生成完成 ====="
  go fmt ./*.go >> /dev/null 2>&1 || true
  go fmt ./internal/server/* >> /dev/null 2>&1 || true
  exit 0
fi

# ------ 4. 备份需要保护的文件 ------

step "4/8 备份已有 HTTP 文件（保护策略）"

mkdir -p "${BACKUP_DIR}/handler" "${BACKUP_DIR}/types"

# 备份已有 handler 子目录
if [ -d "./internal/handler" ]; then
  for group_dir in ./internal/handler/*/; do
    [ -d "$group_dir" ] || continue
    group_name=$(basename "$group_dir")
    mkdir -p "${BACKUP_DIR}/handler/${group_name}"
    cp -p "$group_dir"*.go "${BACKUP_DIR}/handler/${group_name}/" 2>/dev/null || true
    echo "  备份 handler/${group_name}/"
  done
fi

# 备份 routes.go
if [ -f "./internal/handler/routes.go" ]; then
  cp -p "./internal/handler/routes.go" "${BACKUP_DIR}/handler/routes.go"
  echo "  备份 handler/routes.go"
fi

# 备份 types.go（goctl 会重新生成，直接恢复备份即可保留所有自定义代码）
if [ -f "./internal/types/types.go" ]; then
  cp -p "./internal/types/types.go" "${BACKUP_DIR}/types/types.go.bak"
  echo "  备份 types/types.go"
fi

# ------ 5. goctl api go 生成 HTTP API 代码 ------

step "5/8 生成 HTTP API 代码 (goctl api go)"

goctl api go -api "${API_FILE}" -dir . --style goZero
echo "  HTTP API 代码生成完成"

# 删除 goctl 自动生成的 yaml 配置文件（项目使用统一的 etc/chaininteractive.yaml）
GENERATED_YAML="./etc/${SERVER_NAME}-api.yaml"
if [ -f "${GENERATED_YAML}" ]; then
  rm -f "${GENERATED_YAML}"
  echo "  删除生成的配置文件: ${GENERATED_YAML}"
fi

# ------ 恢复受保护的 handler 文件 ------

echo ""
echo "  恢复受保护的 handler 文件..."

# 恢复 handler 子目录：已存在的文件不覆盖（保留已有业务逻辑）
if [ -d "${BACKUP_DIR}/handler" ]; then
  for group_dir in "${BACKUP_DIR}/handler"/*/; do
    [ -d "$group_dir" ] || continue
    group_name=$(basename "$group_dir")
    target_dir="./internal/handler/${group_name}"
    for f in "$group_dir"*.go; do
      [ -f "$f" ] || continue
      filename=$(basename "$f")
      dest="${target_dir}/${filename}"
      if [ -f "${dest}" ]; then
        # 已存在的文件恢复备份（保护已有业务逻辑），goctl 新生成的不保留
        cp -p "$f" "${dest}"
        echo "  恢复(保护): handler/${group_name}/${filename}"
      else
        # 新增的文件（之前不存在的 handler），保留 goctl 生成的版本
        echo "  新增: handler/${group_name}/${filename}"
      fi
    done
  done
fi

# 恢复 routes.go（提醒开发者手动合并）
if [ -f "${BACKUP_DIR}/handler/routes.go" ]; then
  cp -p "${BACKUP_DIR}/handler/routes.go" "./internal/handler/routes.go"
  echo "  恢复 handler/routes.go（如 .api 有新路由，请手动合并）"
fi

# 恢复 types.go（直接恢复备份，保留所有自定义代码）
TYPES_BAK="${BACKUP_DIR}/types/types.go.bak"
if [ -f "${TYPES_BAK}" ]; then
  cp -p "${TYPES_BAK}" "./internal/types/types.go"
  echo "  恢复 types.go（保留所有自定义代码）"
else
  echo "  types.go 无备份，保留 goctl 生成版本"
fi

# ------ 6. 迁移 HTTP logic 到 internal/logic/http/{group}/ ------

step "6/8 迁移 HTTP logic → internal/logic/http/{group}/"

HTTP_LOGIC_DIR="./internal/logic/http"
mkdir -p "${HTTP_LOGIC_DIR}"

# 遍历 internal/logic/ 下由 goctl api 生成的 group 子目录（排除 grpc/ 和 http/ 目录）
for group_dir in ./internal/logic/*/; do
  [ -d "$group_dir" ] || continue
  group_name=$(basename "$group_dir")
  # 跳过 grpc 和 http 目录
  if [ "$group_name" = "grpc" ] || [ "$group_name" = "http" ]; then
    continue
  fi

  dest_dir="${HTTP_LOGIC_DIR}/${group_name}"
  mkdir -p "${dest_dir}"

  for f in "$group_dir"*.go; do
    [ -f "$f" ] || continue
    filename=$(basename "$f")
    dest_file="${dest_dir}/${filename}"
    if [ -f "${dest_file}" ]; then
      # 目标已存在同名文件（业务逻辑已填充），跳过覆盖并删除新生成的文件
      echo "  skip (already exists): logic/http/${group_name}/${filename}"
      rm -f "$f"
    else
      # 检查目标目录中是否已有包含相同 Logic 类型名的文件（文件名不同但类型名相同的情况）
      # 提取 goctl 生成文件中的 Logic 类型名（如 type GetUsageStatsLogic struct）
      LOGIC_TYPE=$(grep -m1 "^type.*Logic struct" "$f" 2>/dev/null | awk '{print $2}')
      if [ -n "${LOGIC_TYPE}" ] && grep -rl "^type ${LOGIC_TYPE} struct" "${dest_dir}" 2>/dev/null | grep -q .; then
        # 目标目录中已有相同 Logic 类型名的文件，跳过并删除新生成的骨架文件
        EXISTING_FILE=$(grep -rl "^type ${LOGIC_TYPE} struct" "${dest_dir}" 2>/dev/null | head -1)
        echo "  skip (type ${LOGIC_TYPE} already in $(basename "${EXISTING_FILE}")): logic/http/${group_name}/${filename}"
        rm -f "$f"
      else
        # 目标不存在，移动文件
        mv "$f" "${dest_file}"
        echo "  moved: logic/${group_name}/${filename} -> logic/http/${group_name}/${filename}"
      fi
    fi
  done

  # 删除空的 group 目录
  if [ -d "$group_dir" ] && [ -z "$(ls -A "$group_dir" 2>/dev/null)" ]; then
    rmdir "$group_dir"
  fi
done

# ------ 7. 修正 handler import 路径 ------

step "7/8 修正新增 handler 的 import 路径 (logic/{group} → logic/http/{group})"

# 只对新增的 handler 文件（备份中不存在的）做 import 修正
# 已有文件已在步骤5中通过备份恢复，import 路径已正确，无需处理
LOGIC_BASE="${MODULE}/internal/logic/"
LOGIC_HTTP="${MODULE}/internal/logic/http/"

find ./internal/handler -name "*.go" -type f | while read -r f; do
  group_name=$(basename "$(dirname "$f")")
  filename=$(basename "$f")
  backup_file="${BACKUP_DIR}/handler/${group_name}/${filename}"

  # 只处理备份中不存在的新增文件
  if [ ! -f "${backup_file}" ]; then
    # 修正 import 路径：logic/{group} → logic/http/{group}
    # 新增文件一定是 goctl 生成的，import 路径为 logic/{group}，需要改为 logic/http/{group}
    # 用 sed 替换：将 "xxx/internal/logic/{group}" 改为 "xxx/internal/logic/http/{group}"
    # 注意：不替换已含 http/ 或 grpc/ 的路径
    if grep -qF "${LOGIC_BASE}" "$f" 2>/dev/null; then
      # 临时将 logic/grpc/ 和 logic/http/ 替换为占位符
      sed -i '' "s|${MODULE}/internal/logic/grpc/|__LOGIC_GRPC__/|g" "$f"
      sed -i '' "s|${MODULE}/internal/logic/http/|__LOGIC_HTTP__/|g" "$f"
      # 替换剩余的 logic/{group} → logic/http/{group}
      sed -i '' "s|${LOGIC_BASE}|${LOGIC_HTTP}|g" "$f"
      # 恢复占位符
      sed -i '' "s|__LOGIC_GRPC__/|${MODULE}/internal/logic/grpc/|g" "$f"
      sed -i '' "s|__LOGIC_HTTP__/|${MODULE}/internal/logic/http/|g" "$f"
      echo "  修正新增文件: $(echo "$f" | sed 's|^\./||')"
    fi
  fi
done

# ------ 8. GatewayConf 配置追加 ------

step "8/8 检查 GatewayConf 配置"

YAML_FILE="./etc/${SERVER_NAME}.yaml"
if [ -f "${YAML_FILE}" ]; then
  if grep -q "GatewayConf:" "${YAML_FILE}"; then
    echo "  GatewayConf 配置已存在，跳过追加"
  else
    # 追加 GatewayConf 配置块（使用 config.go 中定义的默认值）
    cat >> "${YAML_FILE}" << 'EOF'

# HTTP Gateway 配置
GatewayConf:
  # Enable 是否启用 HTTP Gateway
  Enable: true
  # Host HTTP 监听地址
  Host: 0.0.0.0
  # Port HTTP 监听端口
  Port: 8080
  # RateLimit 默认 QPS 限制（每租户）
  RateLimit: 10
EOF
    echo "  追加 GatewayConf 配置到 ${YAML_FILE}"
  fi
else
  echo "  WARNING: ${YAML_FILE} 不存在，跳过 GatewayConf 配置追加"
fi

# ============================================================
# 格式化
# ============================================================

echo ""
step "格式化代码"

go fmt ./*.go >> /dev/null 2>&1 || true
go fmt ./internal/server/... >> /dev/null 2>&1 || true
go fmt ./internal/handler/... >> /dev/null 2>&1 || true
go fmt ./internal/logic/... >> /dev/null 2>&1 || true
go fmt ./internal/types/... >> /dev/null 2>&1 || true
echo "  代码格式化完成"

# ============================================================
# 执行结果摘要
# ============================================================

echo ""
echo "========================================="
echo "  代码生成完成！"
echo "========================================="
echo ""
echo "  gRPC:"
echo "    - pb 代码:          ./pb/"
echo "    - gRPC server:      ./internal/server/"
echo "    - gRPC logic:       ./internal/logic/grpc/"
echo ""
echo "  HTTP API:"
echo "    - handler:          ./internal/handler/"
echo "    - HTTP logic:       ./internal/logic/http/"
echo "    - types:            ./internal/types/"
echo ""
echo "  配置:"
echo "    - 统一配置文件:     ./etc/${SERVER_NAME}.yaml"
echo ""
echo "  注意事项:"
echo "    - routes.go 已恢复备份版本，如 .api 有新路由请手动合并"
echo "    - 已有业务逻辑文件已保护，不会被覆盖"
echo "========================================="