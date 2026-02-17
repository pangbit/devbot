#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/opt/devbot"
SERVICE_NAME="devbot"

echo "=== DevBot 部署脚本 ==="

# 检查 root 权限
if [ "$(id -u)" -ne 0 ]; then
    echo "请使用 root 权限运行: sudo bash $0"
    exit 1
fi

# 编译
echo "编译..."
make build

# 创建目录
echo "安装到 ${INSTALL_DIR}..."
mkdir -p "${INSTALL_DIR}"
cp -f devbot "${INSTALL_DIR}/devbot"
chmod 755 "${INSTALL_DIR}/devbot"

# 安装配置文件（不覆盖已有的）
if [ ! -f "${INSTALL_DIR}/config.yaml" ]; then
    cp deploy/config.example.yaml "${INSTALL_DIR}/config.yaml"
    chmod 600 "${INSTALL_DIR}/config.yaml"
    echo "已创建配置文件 ${INSTALL_DIR}/config.yaml，请编辑后启动"
else
    echo "配置文件已存在，跳过"
fi

# 安装 systemd service
echo "安装 systemd service..."
cp deploy/devbot.service /etc/systemd/system/${SERVICE_NAME}.service
systemctl daemon-reload

# 提示
echo ""
echo "=== 部署完成 ==="
echo ""
echo "1. 编辑配置文件:"
echo "   vim ${INSTALL_DIR}/config.yaml"
echo ""
echo "2. 启动服务:"
echo "   systemctl start ${SERVICE_NAME}"
echo ""
echo "3. 设置开机自启:"
echo "   systemctl enable ${SERVICE_NAME}"
echo ""
echo "4. 查看日志:"
echo "   tail -f ${INSTALL_DIR}/devbot.log"
echo ""
echo "5. 查看状态:"
echo "   systemctl status ${SERVICE_NAME}"
