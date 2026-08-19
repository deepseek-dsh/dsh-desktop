APP_NAME    := dsh-desktop
BIN_DIR     := build/bin
DIST_DIR    := dist
VERSION     ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo 0.1.0)
ARCH        := amd64
MAINTAINER  := zhx <17760337690@163.com>

.PHONY: all package build windows linux deb clean test vet dev help

help: ## 显示所有可用目标
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

all: package ## 默认: 构建 Windows 与 Linux 安装包

package: windows linux ## 构建 Windows 与 Linux 安装包

build: ## 仅构建当前平台二进制
	wails build -clean

windows: ## 构建 Windows 安装包(NSIS)
	wails build -platform windows/amd64 -nsis -clean
	@mkdir -p $(DIST_DIR)
	@cp $(BIN_DIR)/dsh-desktop-*-installer.exe $(DIST_DIR)/
	@echo "Windows 安装包: $(DIST_DIR)/dsh-desktop-$(ARCH)-installer.exe"

linux: $(BIN_DIR)/dsh-desktop deb ## 构建 Linux 安装包(deb)

$(BIN_DIR)/dsh-desktop: ## 构建 Linux 二进制
	wails build -platform linux/amd64 -clean

deb: $(BIN_DIR)/dsh-desktop ## 制作 deb 安装包
	@rm -rf $(DIST_DIR)/staging
	@mkdir -p $(DIST_DIR)/staging/DEBIAN
	@mkdir -p $(DIST_DIR)/staging/usr/bin
	@mkdir -p $(DIST_DIR)/staging/usr/share/applications
	@mkdir -p $(DIST_DIR)/staging/usr/share/pixmaps
	@mkdir -p $(DIST_DIR)/staging/usr/share/icons/hicolor
	@cp $(BIN_DIR)/$(APP_NAME) $(DIST_DIR)/staging/usr/bin/$(APP_NAME)
	@cp build/dsh-desktop.png $(DIST_DIR)/staging/usr/share/pixmaps/$(APP_NAME).png
	@python3 -c 'import os; from PIL import Image; \
		src = "$(DIST_DIR)/staging/usr/share/icons/hicolor"; \
		s = Image.open("build/$(APP_NAME).png").convert("RGBA"); \
		[(os.makedirs(f"{src}/{n}x{n}/apps", exist_ok=True), s.resize((n, n), Image.LANCZOS).save(f"{src}/{n}x{n}/apps/$(APP_NAME).png")) for n in (48, 64, 128, 256, 512)]'
	@printf '%s\n' \
		'[Desktop Entry]' \
		'Type=Application' \
		'Name=DSH Desktop' \
		'Comment=DeepSeek Harness Desktop' \
		'Exec=/usr/bin/$(APP_NAME)' \
		'Icon=$(APP_NAME)' \
		'Terminal=false' \
		'Categories=Utility;Development;' \
		'StartupWMClass=$(APP_NAME)' \
		> $(DIST_DIR)/staging/usr/share/applications/$(APP_NAME).desktop
	@printf '%s\n' \
		'Package: $(APP_NAME)' \
		'Version: $(VERSION)' \
		'Section: utils' \
		'Priority: optional' \
		'Architecture: $(ARCH)' \
		'Maintainer: $(MAINTAINER)' \
		'Description: DeepSeek Harness Desktop' \
		' Desktop client for DeepSeek Harness. Double-click to run, no CLI required.' \
		> $(DIST_DIR)/staging/DEBIAN/control
	@dpkg-deb --build --root-owner-group $(DIST_DIR)/staging $(DIST_DIR)/$(APP_NAME)_$(VERSION)_$(ARCH).deb
	@rm -rf $(DIST_DIR)/staging
	@echo "Linux 安装包: $(DIST_DIR)/$(APP_NAME)_$(VERSION)_$(ARCH).deb"

test: vet ## 运行测试
	go test ./...

vet: ## 静态检查
	go vet ./...

dev: ## 开发模式(热更新 + 绑定自动生成)
	wails dev

clean: ## 清理构建产物与安装包
	rm -rf $(BIN_DIR) $(DIST_DIR)