# Docker Builder - 自动化 Docker 镜像构建工具

一个基于 Go 开发的自动化 Docker 镜像构建工具，支持从 GitHub 仓库自动拉取代码、构建多平台镜像并推送到 Docker Hub。

## 📋 目录

- [功能特性](#-功能特性)
- [系统架构](#-系统架构)
- [快速开始](#-快速开始)
- [配置说明](#-配置说明)
- [目录结构](#-目录结构)
- [使用示例](#-使用示例)
- [高级功能](#-高级功能)
- [环境要求](#-环境要求)

## ✨ 功能特性

- ✅ **自动化构建**：自动检测 GitHub 仓库中的 Dockerfile 并构建镜像
- ✅ **多平台支持**：支持构建多架构镜像（amd64、arm64 等）
- ✅ **分支与 Tag**：支持指定分支或 Tag 进行构建
- ✅ **Dockerfile 优先级**：支持 `dockerfile_user` 和 `dockerfile_project` 两种自定义方式，`dockerfile_user` 优先级最高
- ✅ **本地上下文构建**：支持使用本地目录作为构建上下文，跳过 git clone
- ✅ **代理支持**：内置网络代理配置，解决网络限制问题
- ✅ **构建参数**：支持传递 ARG 参数到 Docker 构建过程
- ✅ **并发处理**：支持同时构建多个仓库
- ✅ **灵活配置**：YAML 格式配置，易于理解和维护

## 🏗️ 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                    Docker Builder                           │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │  Config      │  │  GitHub      │  │  Docker      │       │
│  │  Parser      │──│  Client      │──│  Client      │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
│                        │                  │                   │
│                        ▼                  ▼                   │
│              ┌──────────────────┐  ┌──────────────┐          │
│              │  Repository      │  │  Buildx      │          │
│              │  Operations      │  │  Multi-      │          │
│              │                  │  │  Platform    │          │
│              └──────────────────┘  └──────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

## 🚀 快速开始

### 1. 编译项目

```bash
go build -o docker-builder ./main.go
```

### 2. 配置 config.yaml

参考 [配置说明](#配置说明) 创建配置文件。

### 3. 运行构建

```bash
./docker-builder -c config.yaml
```

## 📝 配置说明

### 完整配置示例

```yaml
docker_hub:
  username: "your-dockerhub-username"
  password: "your-dockerhub-password-or-token"

github:
  username: "your-github-username"
  token: "your-github-personal-access-token"  # 可选

proxy:
  enabled: false
  http: "http://proxy.example.com:8080"
  https: "http://proxy.example.com:8080"
  no_proxy: "localhost,127.0.0.1"

repositories:
  - url: "https://github.com/owner/repo1"
    enabled: true
    branch: "main"
    tag: "latest"
    dockerfile_project: "Dockerfile"
    dockerfile_user: ""  # 用户自定义 Dockerfile 路径
    context_dir: ""  # 本地上下文目录（与 url 互斥）
    platforms:
      - "linux/amd64"
      - "linux/arm64"
    build_args:
      VERSION: "1.0.0"
      BUILD_DATE: "2026-03-12"
```

### 配置项详细说明

#### docker_hub

Docker Hub 认证配置。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | Docker Hub 用户名 |
| password | string | 是 | Docker Hub 密码或 Access Token |

#### github

GitHub API 访问配置。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | GitHub 用户名 |
| token | string | 否 | GitHub Personal Access Token（建议配置以避免速率限制） |

#### proxy

网络代理配置（可选）。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| enabled | boolean | 是 | 是否启用代理 |
| http | string | 否 | HTTP 代理地址 |
| https | string | 否 | HTTPS 代理地址 |
| no_proxy | string | 否 | 不使用代理的主机列表，逗号分隔 |

#### repositories

仓库配置列表。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| url | string | 否 | GitHub 仓库 URL（与 context_dir 互斥） |
| context_dir | string | 否 | 本地构建上下文目录（与 url 互斥） |
| enabled | boolean | 否 | 是否启用该仓库（默认 true） |
| branch | string | 是 | 要构建的分支（默认 "main"） |
| tag | string | 是 | 镜像标签（默认 "latest"） |
| dockerfile_project | string | 否 | 仓库中的 Dockerfile 路径（默认 "Dockerfile"） |
| dockerfile_user | string | 否 | 用户自定义 Dockerfile 的本地路径（优先级高于 dockerfile_project） |
| platforms | array | 否 | 目标平台列表（空则使用单平台构建） |
| build_args | map | 否 | Docker 构建参数（ARG） |

**Dockerfile 优先级说明：**
- `dockerfile_user` 优先级最高，会覆盖仓库中的任何 Dockerfile
- `dockerfile_project` 用于指定仓库中非默认路径的 Dockerfile
- 两者都未配置时，默认使用仓库根目录的 `Dockerfile`

**URL 与 ContextDir 说明：**
- `url` 和 `context_dir` 互斥，只能配置其中一个
- 配置 `url` 时：会克隆 GitHub 仓库到临时目录进行构建
- 配置 `context_dir` 时：直接使用本地目录作为构建上下文，跳过 git clone

### 镜像命名规则

镜像名称格式：`{username}/{repo}[-{branch}]:{tag}`

- 当 `branch` 为 `main` 或 `master` 时，省略分支名
- 当 `branch` 为其他分支时，包含分支名

**示例：**

| 配置 | 镜像名称 |
|------|---------|
| `branch: "main", tag: "latest"` | `qida/picoclaw:latest` |
| `branch: "dev", tag: "v1.0"` | `qida/picoclaw-dev:v1.0` |
| `context_dir: "/path/to/project"` | `qida/project:latest` |

## 📂 目录结构

```
docker-build/
├── config.yaml              # 配置文件
├── main.go                  # 主程序入口
├── go.mod                   # Go 模块定义
├── go.sum                   # Go 依赖校验
├── internal/                # 内部包
│   ├── config/              # 配置解析
│   │   └── config.go
│   ├── github/              # GitHub 客户端
│   │   └── github.go
│   └── docker/              # Docker 客户端
│       └── docker.go
└── dockerfile/              # 用户自定义 Dockerfile
    ├── n2n/
    │   └── Dockerfile
    └── go-thrift/
        └── Dockerfile
```

## 💡 使用示例

### 示例 1：基本构建

从 GitHub 仓库构建单平台镜像：

```yaml
repositories:
  - url: "https://github.com/example/project"
    branch: "main"
    tag: "latest"
    platforms:
      - "linux/amd64"
```

构建镜像：`qida/project:latest`

### 示例 2：多平台构建

构建多架构镜像：

```yaml
repositories:
  - url: "https://github.com/example/project"
    branch: "dev"
    tag: "v2.3.0"
    platforms:
      - "linux/amd64"
      - "linux/arm64"
```

构建镜像：`qida/project-dev:v2.3.0`

### 示例 3：自定义 Dockerfile

使用仓库中非默认路径的 Dockerfile（使用 `dockerfile_project`）：

```yaml
repositories:
  - url: "https://github.com/example/project"
    branch: "main"
    tag: "latest"
    dockerfile_project: "docker/Dockerfile.prod"
    platforms:
      - "linux/amd64"
```

构建镜像：`qida/project:latest`

### 示例 4：用户自定义 Dockerfile

当仓库没有 Dockerfile 时，使用用户提供的 Dockerfile（优先级最高）：

```yaml
repositories:
  - url: "https://github.com/ntop/n2n"
    branch: "dev"
    tag: "latest"
    dockerfile_user: "./dockerfile/Dockerfile.n2n"
    platforms:
      - "linux/amd64"
```

构建镜像：`qida/n2n-dev:latest`

**注意：** `dockerfile_user` 优先级高于 `dockerfile_project`，会覆盖仓库中的任何 Dockerfile。

### 示例 5：本地上下文构建

当 `url` 为空时，使用本地目录作为构建上下文，跳过 git clone：

```yaml
repositories:
  - context_dir: "/path/to/local/project"
    branch: "main"
    tag: "latest"
    dockerfile_user: "./dockerfile/Dockerfile"
    platforms:
      - "linux/amd64"
```

构建镜像：`qida/project:latest`

**说明：**
- `url` 配置为空字符串 `""`
- `context_dir` 指定本地目录路径
- 适用于本地开发测试或无法访问 GitHub 的场景

### 示例 6：传递构建参数

使用 ARG 参数构建：

```yaml
repositories:
  - url: "https://github.com/example/project"
    branch: "main"
    tag: "latest"
    platforms:
      - "linux/amd64"
    build_args:
      VERSION: "1.0.0"
      BUILD_DATE: "2026-03-12"
```

### 示例 7：组合使用

结合所有功能的完整示例：

```yaml
proxy:
  enabled: true
  http: "http://proxy.example.com:8080"
  https: "http://proxy.example.com:8080"

repositories:
  - url: "https://github.com/example/project1"
    enabled: true
    branch: "main"
    tag: "latest"
    dockerfile_project: "docker/Dockerfile"
    platforms:
      - "linux/amd64"
      - "linux/arm64"
    build_args:
      VERSION: "1.0.0"
      BUILD_DATE: "2026-03-12"

  - context_dir: "/local/path/to/project2"
    enabled: true
    branch: "dev"
    tag: "v1.0"
    dockerfile_user: "./dockerfile/Dockerfile"
    platforms:
      - "linux/amd64"
    build_args:
      ENV: "production"
```

## 🛠️ 高级功能

### 本地上下文构建

当需要在没有网络或本地开发测试时，可以使用 `context_dir` 配置：

```yaml
repositories:
  - context_dir: "/path/to/local/project"
    branch: "main"
    tag: "latest"
    dockerfile_user: "./dockerfile/Dockerfile"
    platforms:
      - "linux/amd64"
```

**使用场景：**
- 本地开发测试
- 离线环境构建
- 性能优化（避免重复克隆）

### 网络代理

在受限网络环境中，可以配置代理：

```yaml
proxy:
  enabled: true
  http: "http://proxy.example.com:8080"
  https: "http://proxy.example.com:8080"
  no_proxy: "localhost,127.0.0.1"
```

### 禁用仓库

临时禁用某个仓库的构建：

```yaml
repositories:
  - url: "https://github.com/example/project"
    enabled: false
    branch: "main"
    tag: "latest"
```

## 📋 环境要求

### 系统要求

- **操作系统**：Linux、macOS、Windows
- **Go 版本**：1.21 或更高
- **Docker**：19.03 或更高（支持 buildx）
- **Git**：2.20 或更高

### 权限要求

- **Docker**：需要访问 Docker Daemon 的权限
- **GitHub**：需要读取仓库的权限（公开仓库无需 token）
- **Docker Hub**：需要推送镜像的权限

### 网络要求

- 访问 GitHub API（如配置了 token，可提高速率限制）
- 访问 Docker Hub（用于推送镜像）
- 可选：网络代理（在受限环境中）

## 🔧 故障排查

### 常见问题

1. **Docker buildx 未安装**
   ```bash
   docker buildx version
   # 如果未安装，请安装 Docker Desktop 或 docker-buildx-plugin
   ```

2. **GitHub API 速率限制**
   - 配置 GitHub token
   - 使用 `ghp_` 开头的 Personal Access Token

3. **网络连接问题**
   - 配置代理
   - 检查防火墙设置

4. **Docker Hub 登录失败**
   - 检查用户名和密码/Token
   - 确保 Docker Hub 账户未被锁定

## 📄 许可证

MIT License
