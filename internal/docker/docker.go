package docker

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"docker-build/internal/config"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
)

type Client struct {
	cli      *client.Client
	proxyCfg *config.ProxyConfig
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

func (c *Client) SetProxyConfig(proxyCfg *config.ProxyConfig) {
	c.proxyCfg = proxyCfg
}

// 有git，从远程构建镜像
func (c *Client) BuildImagea(ctx context.Context, contextDir, imageName string, platforms []string, buildArgs map[string]string, dockerfilePath string, proxyConfig *config.ProxyConfig) error {
	log.Printf("[DOCKER] Building image %s from context %s with Dockerfile %s...\n", imageName, contextDir, dockerfilePath)
	if proxyConfig != nil && proxyConfig.Enabled {
		fmt.Printf("[DOCKER] Using proxy: HTTP=%s, HTTPS=%s\n", proxyConfig.HTTP, proxyConfig.HTTPS)
	}

	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}

	if len(platforms) > 0 {
		fmt.Printf("[DOCKER] Building for platforms: %s\n", strings.Join(platforms, ", "))

		tarCmd := exec.CommandContext(ctx, "tar", "cf", "-", "-C", contextDir, ".")
		buildxCmd := exec.CommandContext(ctx, "docker", "buildx", "build",
			"--platform", strings.Join(platforms, ","),
			"-f", dockerfilePath,
			"-t", imageName,
			"--push",
			"--progress=plain",
			"-",
		)

		for k, v := range buildArgs {
			buildxCmd.Args = append(buildxCmd.Args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}

		if proxyConfig != nil && proxyConfig.Enabled {
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

		tarStdout, err := tarCmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create tar pipe: %v", err)
		}
		buildxCmd.Stdin = tarStdout

		buildxCmd.Stdout = os.Stdout
		buildxCmd.Stderr = os.Stderr

		if err := tarCmd.Start(); err != nil {
			return fmt.Errorf("failed to start tar: %v", err)
		}

		if err := buildxCmd.Start(); err != nil {
			tarCmd.Wait()
			return fmt.Errorf("failed to start buildx: %v", err)
		}

		tarCmd.Wait()
		return buildxCmd.Wait()
	} else {
		args := []string{"build", "-t", imageName, contextDir}

		for k, v := range buildArgs {
			args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}

		if dockerfilePath != "Dockerfile" {
			args = append(args, "-f", dockerfilePath)
		}

		if proxyConfig != nil && proxyConfig.Enabled {
			if proxyConfig.HTTP != "" {
				args = append(args, "--build-arg", fmt.Sprintf("HTTP_PROXY=%s", proxyConfig.HTTP))
				args = append(args, "--build-arg", fmt.Sprintf("http_proxy=%s", proxyConfig.HTTP))
			}
			if proxyConfig.HTTPS != "" {
				args = append(args, "--build-arg", fmt.Sprintf("HTTPS_PROXY=%s", proxyConfig.HTTPS))
				args = append(args, "--build-arg", fmt.Sprintf("https_proxy=%s", proxyConfig.HTTPS))
			}
			if proxyConfig.NoProxy != "" {
				args = append(args, "--build-arg", fmt.Sprintf("NO_PROXY=%s", proxyConfig.NoProxy))
				args = append(args, "--build-arg", fmt.Sprintf("no_proxy=%s", proxyConfig.NoProxy))
			}
		}

		buildCmd := exec.CommandContext(ctx, "docker", args...)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr

		return buildCmd.Run()
	}
}

func (c *Client) EnsureBuildxBuilder(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "buildx", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("buildx not available: %v", err)
	}

	cmd = exec.CommandContext(ctx, "docker", "buildx", "inspect", "multiarch-builder")
	if err := cmd.Run(); err != nil {
		fmt.Println("[DOCKER] Creating multiarch-builder...")
		cmd = exec.CommandContext(ctx, "docker", "buildx", "create", "--name", "multiarch-builder", "--use", "--bootstrap")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create buildx builder: %v", err)
		}
		fmt.Println("[DOCKER] multiarch-builder created and ready")
	} else {
		fmt.Println("[DOCKER] Using existing multiarch-builder")
	}
	return nil
}

func (c *Client) PushImage(ctx context.Context, imageName, username, password string) error {
	authConfig := registry.AuthConfig{
		Username: username,
		Password: password,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)

	out, err := c.cli.ImagePush(ctx, imageName, image.PushOptions{
		RegistryAuth: authStr,
	})
	if err != nil {
		return err
	}
	defer out.Close()

	decoder := json.NewDecoder(out)
	for {
		var msg map[string]interface{}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if msg["error"] != nil {
			return fmt.Errorf("push failed: %v", msg["error"])
		}
		if msg["status"] != nil {
			fmt.Printf("[DOCKER] %s\n", msg["status"])
		}
	}

	return nil
}

func (c *Client) Login(ctx context.Context, username, password string) error {
	authConfig := registry.AuthConfig{
		Username: username,
		Password: password,
	}

	// 如果启用了代理，设置环境变量
	if c.proxyCfg != nil && c.proxyCfg.Enabled {
		if c.proxyCfg.HTTP != "" {
			err := os.Setenv("HTTP_PROXY", c.proxyCfg.HTTP)
			if err != nil {
				return err
			}
		}
		if c.proxyCfg.HTTPS != "" {
			err := os.Setenv("HTTPS_PROXY", c.proxyCfg.HTTPS)
			if err != nil {
				return err
			}
		}
		if c.proxyCfg.NoProxy != "" {
			err := os.Setenv("NO_PROXY", c.proxyCfg.NoProxy)
			if err != nil {
				return err
			}
		}
		fmt.Printf("[DOCKER] Proxy enabled: HTTP_PROXY=%s, HTTPS_PROXY=%s, NO_PROXY=%s\n", c.proxyCfg.HTTP, c.proxyCfg.HTTPS, c.proxyCfg.NoProxy)
	} else {
		fmt.Printf("[DOCKER] Proxy disabled\n")
	}

	_, err := c.cli.RegistryLogin(ctx, authConfig)

	// 清理环境变量
	if c.proxyCfg != nil && c.proxyCfg.Enabled {
		os.Unsetenv("HTTP_PROXY")
		os.Unsetenv("HTTPS_PROXY")
		os.Unsetenv("NO_PROXY")
		fmt.Printf("[DOCKER] Proxy environment variables cleaned up\n")
	}

	return err
}

func createBuildContext(sourceDir string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "docker-build-")
	if err != nil {
		return "", err
	}

	tarFile := filepath.Join(tmpDir, "context.tar.gz")
	file, err := os.Create(tarFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == tarFile {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, relPath)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tarWriter, f); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return tarFile, nil
}
