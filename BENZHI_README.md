# BENZHI_README

基于 Go 实现的古树病原鉴定与保护放行 Web 项目，一款后端服务，用于管理古树病原取样、鉴定复核、风险处置和保护放行。

## 项目说明
- 项目：benzhi-project-b1ff9581-e322-4ac3-9c59-ca7a24c55e3d
- 项目用途：古树保护人员完成病原取样鉴定、风险复核与保护放行。
- Go 工具链：`golang:1.23`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run . -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-b1ff9581-e322-4ac3-9c59-ca7a24c55e3d-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-b1ff9581-e322-4ac3-9c59-ca7a24c55e3d-arm64 linux/arm64
docker run -it benzhi-project-b1ff9581-e322-4ac3-9c59-ca7a24c55e3d-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run . -selfcheck -addr=127.0.0.1:19081`
