package docker

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"docker-build/internal/config"
)

// 无git，本地构建镜像
func (c *Client) BuildImage(ctx context.Context, contextDir, dockerfilePath, imageName string, platforms []string, buildArgs map[string]string, proxyConfig *config.ProxyConfig) error {
	log.Printf("[DOCKER] Building image from contextDir %s with Dockerfile %s. imageName:%s..\n", contextDir, dockerfilePath, imageName)
	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}
	platformsStr := "linux/amd64"
	if len(platforms) > 1 {
		platformsStr = strings.Join(platforms, ",")
	}
	fmt.Printf("[DOCKER] Building for platforms: %s\n", platformsStr)
	buildxCmd := exec.CommandContext(ctx, "docker", "buildx", "build",
		"--platform", platformsStr,
		"-f", dockerfilePath,
		"-t", imageName,
		"--push",
		"--progress=plain",
		contextDir,
	)

	for k, v := range buildArgs {
		buildxCmd.Args = append(buildxCmd.Args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	if proxyConfig != nil && proxyConfig.Enabled {
		fmt.Printf("[DOCKER] Using proxy: HTTP=%s, HTTPS=%s\n", proxyConfig.HTTP, proxyConfig.HTTPS)

		if proxyConfig.HTTP != "" {
			buildxCmd.Args = append(buildxCmd.Args, "--build-arg", fmt.Sprintf("HTTP_PROXY=%s", proxyConfig.HTTP))
			buildxCmd.Args = append(buildxCmd.Args, "--build-arg", fmt.Sprintf("http_proxy=%s", proxyConfig.HTTP))
		}
		if proxyConfig.HTTPS != "" {
			buildxCmd.Args = append(buildxCmd.Args, "--build-arg", fmt.Sprintf("HTTPS_PROXY=%s", proxyConfig.HTTPS))
			buildxCmd.Args = append(buildxCmd.Args, "--build-arg", fmt.Sprintf("https_proxy=%s", proxyConfig.HTTPS))
		}
		if proxyConfig.NoProxy != "" {
			buildxCmd.Args = append(buildxCmd.Args, "--build-arg", fmt.Sprintf("NO_PROXY=%s", proxyConfig.NoProxy))
			buildxCmd.Args = append(buildxCmd.Args, "--build-arg", fmt.Sprintf("no_proxy=%s", proxyConfig.NoProxy))
		}
	}
	buildxCmd.Stdout = os.Stdout
	buildxCmd.Stderr = os.Stderr
	if err := buildxCmd.Start(); err != nil {
		return fmt.Errorf("failed to start buildx: %v", err)
	}
	return buildxCmd.Wait()
}
