package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/pockethost/pocketone/ext/extkit"
)

// ImageRepo is where release build images are published, tagged v<version>.
const ImageRepo = "ghcr.io/pockethost/pocketone-build"

// DevImage is the build image a development pocketone uses: built locally from its checkout on first use.
const DevImage = "pocketone-build:dev"

// cacheVolume keeps Go's build cache between builds, so a rebuild takes seconds.
const cacheVolume = "pocketone-build-cache"

// Dockerfile is the build image's Dockerfile, relative to a PocketOne checkout (the build context).
const Dockerfile = "ext/desktop/image/Dockerfile"

// DefaultImage is the build image matching this pocketone: the release's own, or DevImage for a development build.
func DefaultImage() string {
	v := extkit.Version
	if v == "" || strings.Contains(v, "dev") {
		return DevImage
	}
	return ImageRepo + ":v" + strings.TrimPrefix(v, "v")
}

// buildInDocker runs the build in the build image: the app folder and the output folder mounted, the same command
// inside, then the macOS bundles signed ad hoc here (codesign only runs on macOS).
func buildInDocker(ctx context.Context, o BuildOptions) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return errors.New("desktop build runs in Docker: install Docker (https://docs.docker.com/get-docker/), or build with your own Go: --local")
	}
	image := orStr(o.Image, DefaultImage())
	if err := ensureImage(ctx, docker, image, o); err != nil {
		return err
	}
	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return err
	}
	args := []string{"run", "--rm",
		"-v", o.Dir + ":/app",
		"-v", o.Out + ":/out",
		"-v", cacheVolume + ":/cache",
	}
	if goruntime.GOOS == "linux" { // files in the output folder belong to you, not root
		args = append(args, "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()))
	}
	args = append(args, image, "desktop", "build", "--out", "/out", "--target", strings.Join(o.Targets, ","))
	fmt.Fprintf(o.Log, "%s running the build in %s\n", stamp(), image)
	cmd := exec.CommandContext(ctx, docker, args...)
	cmd.Stdout, cmd.Stderr = o.Log, o.Log
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("the build in %s failed: %w", image, err)
	}
	// the bundles were built in Linux: sign them here, so macOS runs them on this machine
	for _, t := range o.Targets {
		if goos, goarch, _ := strings.Cut(t, "/"); goos == "darwin" {
			apps, _ := filepath.Glob(filepath.Join(o.Out, goos+"-"+goarch, "*.app"))
			for _, app := range apps {
				adhocSign(ctx, app)
			}
		}
	}
	return nil
}

// ensureImage makes sure image is here: pulled, or for the development image built from the checkout o.Dir is in.
func ensureImage(ctx context.Context, docker, image string, o BuildOptions) error {
	if exec.CommandContext(ctx, docker, "image", "inspect", image).Run() == nil {
		return nil
	}
	if image != DevImage {
		fmt.Fprintf(o.Log, "%s pulling %s\n", stamp(), image)
		return run(ctx, o.Log, docker, "pull", image)
	}
	source, err := findSource(o.Source, o.Dir)
	if err != nil {
		return fmt.Errorf("this development pocketone builds in %s, which isn't here yet and is built from a PocketOne "+
			"checkout: run desktop build inside one, or pass --source <checkout>", DevImage)
	}
	return BuildImage(ctx, source, image, o.Log)
}

// BuildImage builds the build image from a PocketOne checkout (about a minute the first time).
func BuildImage(ctx context.Context, source, image string, log io.Writer) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return errors.New("building the build image needs Docker")
	}
	if _, err := os.Stat(filepath.Join(source, Dockerfile)); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s has no %s", source, Dockerfile)
	}
	start := time.Now()
	fmt.Fprintf(log, "%s building %s from %s (the first time takes a minute or two)\n", stamp(), image, source)
	if err := run(ctx, log, docker, "build", "--progress=plain", "-f", filepath.Join(source, Dockerfile), "-t", image, source); err != nil {
		return fmt.Errorf("building %s: %w", image, err)
	}
	fmt.Fprintf(log, "%s built %s (%s)\n", stamp(), image, time.Since(start).Round(time.Second))
	return nil
}

func run(ctx context.Context, log io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = log, log
	return cmd.Run()
}
