APP_NAME    := dsh-desktop
BIN_DIR     := build/bin
DIST_DIR    := dist
VERSION     ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo 0.1.0)
MAINTAINER  := zhx <17760337690@163.com>

# 默认主架构; 可通过 make linux ARCH=arm64 等覆盖
ARCH ?= amd64

HOST_GOOS   := $(shell go env GOOS)
HOST_GOARCH := $(shell go env GOARCH)

# 各目标平台的 C 编译器: 优先交叉链; 同架构宿主直接复用本机 gcc。
# 交叉编译需工具链前缀相同(如 aarch64-linux-gnu-gcc, x86_64-w64-mingw32-gcc)。
CC_LINUX_AMD64   ?= $(shell command -v x86_64-linux-gnu-gcc 2>/dev/null || { [ "$(HOST_GOOS)/$(HOST_GOARCH)" = "linux/amd64" ] && command -v gcc 2>/dev/null; } || true)
CC_LINUX_ARM64   ?= $(shell command -v aarch64-linux-gnu-gcc 2>/dev/null || { [ "$(HOST_GOOS)/$(HOST_GOARCH)" = "linux/arm64" ] && command -v gcc 2>/dev/null; } || true)
CC_WINDOWS_AMD64 ?= $(shell command -v x86_64-w64-mingw32-gcc 2>/dev/null || { [ "$(HOST_GOOS)/$(HOST_GOARCH)" = "windows/amd64" ] && command -v gcc 2>/dev/null; } || true)
CC_WINDOWS_ARM64 ?= $(shell \
 command -v aarch64-w64-mingw32-gcc 2>/dev/null || \
 { [ -n "$(LLVM_MINGW)" ] && [ -x "$(LLVM_MINGW)/bin/aarch64-w64-mingw32-clang" ] && echo "$(LLVM_MINGW)/bin/aarch64-w64-mingw32-clang"; } || \
 { [ -x "$(HOME)/llvm-mingw/bin/aarch64-w64-mingw32-clang" ] && echo "$(HOME)/llvm-mingw/bin/aarch64-w64-mingw32-clang"; } || \
 { [ -x "/opt/llvm-mingw/bin/aarch64-w64-mingw32-clang" ] && echo "/opt/llvm-mingw/bin/aarch64-w64-mingw32-clang"; } || \
 command -v aarch64-w64-mingw32-clang 2>/dev/null || \
 { [ "$(HOST_GOOS)/(HOST_GOARCH)" = "windows/arm64" ] && command -v gcc 2>/dev/null; } || true)

.PHONY: all package package-all build test vet dev help clean \
	linux linux-all windows windows-all \
	linux-amd64 linux-arm64 windows-amd64 windows-arm64

help: ## 显示所有可用目标
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

all: package ## 默认: 构建 Windows 与 Linux 当前架构($(ARCH))安装包

package: linux windows ## 构建 Windows 与 Linux 当前架构($(ARCH))安装包

package-all: linux-all windows-all ## 构建全部四个架构组合(linux/windows × amd64/arm64)

# ---- Linux ----

linux: linux-$(ARCH) ## 构建 Linux deb 安装包(架构由 ARCH 指定, 默认 amd64)

linux-all: linux-amd64 linux-arm64 ## 构建 Linux amd64+arm64 安装包

linux-amd64:
	$(call build_deb,linux/amd64,$(CC_LINUX_AMD64),amd64)

linux-arm64:
	$(call build_deb,linux/arm64,$(CC_LINUX_ARM64),arm64)

define build_deb
	@echo "构建 Linux/$(3) ..."
	@if [ -z "$(2)" ]; then echo "错误: 缺少 Linux/$(3) 交叉编译器与工具链, 请先安装"; \
		echo "  例如: apt install gcc-aarch64-linux-gnu libgtk-3-dev:arm64 libglib2.0-dev:arm64"; exit 1; fi
	@if [ "$(3)" = "arm64" ]; then \
		if [ ! -d /usr/lib/aarch64-linux-gnu/pkgconfig ] || [ ! -f /usr/lib/aarch64-linux-gnu/pkgconfig/gtk+-3.0.pc ]; then \
			echo "错误: 未安装 arm64 交叉开发库"; \
			echo "  请执行: sudo dpkg --add-architecture arm64 && sudo apt update && sudo apt install libgtk-3-dev:arm64 libwebkit2gtk-4.0-dev:arm64"; exit 1; fi; \
		PKG_CONFIG_PATH=/usr/lib/aarch64-linux-gnu/pkgconfig:/usr/share/pkgconfig \
		PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig:/usr/share/pkgconfig \
		CC="$(2)" CGO_ENABLED=1 wails build -clean -platform $(1); \
	else \
		CC="$(2)" CGO_ENABLED=1 wails build -clean -platform $(1); \
	fi
	@rm -rf $(DIST_DIR)/staging
	@mkdir -p $(DIST_DIR)/staging/DEBIAN
	@mkdir -p $(DIST_DIR)/staging/usr/bin
	@mkdir -p $(DIST_DIR)/staging/usr/share/applications
	@mkdir -p $(DIST_DIR)/staging/usr/share/pixmaps
	@mkdir -p $(DIST_DIR)/staging/usr/share/icons/hicolor
	@cp $(BIN_DIR)/$(APP_NAME) $(DIST_DIR)/staging/usr/bin/$(APP_NAME)
	@cp build/$(APP_NAME).png $(DIST_DIR)/staging/usr/share/pixmaps/$(APP_NAME).png
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
		'Architecture: $(3)' \
		'Maintainer: $(MAINTAINER)' \
		'Description: DeepSeek Harness Desktop' \
		' Desktop client for DeepSeek Harness. Double-click to run, no CLI required.' \
		> $(DIST_DIR)/staging/DEBIAN/control
	@dpkg-deb --build --root-owner-group $(DIST_DIR)/staging $(DIST_DIR)/$(APP_NAME)_$(VERSION)_$(3).deb
	@rm -rf $(DIST_DIR)/staging
	@echo "Linux 安装包: $(DIST_DIR)/$(APP_NAME)_$(VERSION)_$(3).deb"
endef

# ---- Windows ----

windows: windows-$(ARCH) ## 构建 Windows NSIS 安装包(架构由 ARCH 指定, 默认 amd64)

windows-all: windows-amd64 windows-arm64 ## 构建 Windows amd64+arm64 安装包

windows-amd64:
	$(call build_nsis,windows/amd64,$(CC_WINDOWS_AMD64),amd64)

windows-arm64:
	$(call build_nsis,windows/arm64,$(CC_WINDOWS_ARM64),arm64)

define build_nsis
	@echo "构建 Windows/$(3) (NSIS) ..."
	@if [ -z "$(2)" ]; then echo "错误: 缺少 Windows/$(3) 交叉编译器, 请先安装 MinGW 工具链"; \
		echo "  例如: apt install gcc-mingw-w64-$(3)"; exit 1; fi
	@CC="$(2)" CGO_ENABLED=1 wails build -clean -platform $(1) -nsis
	@mkdir -p $(DIST_DIR)
	@cp $(BIN_DIR)/$(APP_NAME)-*-installer.exe $(DIST_DIR)/$(APP_NAME)-$(3)-installer.exe
	@echo "Windows 安装包: $(DIST_DIR)/$(APP_NAME)-$(3)-installer.exe"
endef

# ---- 通用 ----

build: ## 仅构建当前平台二进制
	wails build -clean

test: vet ## 运行测试
	go test ./...

vet: ## 静态检查
	go vet ./...

dev: ## 开发模式(热更新 + 绑定自动生成)
	wails dev

clean: ## 清理构建产物与安装包
	rm -rf $(BIN_DIR) $(DIST_DIR)
