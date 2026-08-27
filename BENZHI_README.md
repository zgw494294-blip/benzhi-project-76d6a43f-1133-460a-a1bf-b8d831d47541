# BENZHI_README

基于 Go 实现的馆藏微环境调控启用台 HTTP API 项目，一款后端服务，文物保护团队对库房微环境调控方案进行试运行、复核与启用。

## 项目说明
- 项目：benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541
- 项目用途：文物保护团队对库房微环境调控方案进行试运行、复核与启用。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/chamberd -selfcheck -selfcheck-timeout=8s -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541-arm64 linux/arm64
docker run -it benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/chamberd -selfcheck -selfcheck-timeout=8s -addr=127.0.0.1:19081`
