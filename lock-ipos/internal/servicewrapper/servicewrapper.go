package servicewrapper

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultServiceName       = "NusaTunnelClient"
	DefaultConfigName        = "client.toml"
	DefaultRatholeBinaryName = "nusatunnel.exe"

	defaultStartupGracePeriod  = 3 * time.Second
	defaultShutdownGracePeriod = 10 * time.Second
)

// Config controls how the Windows service wrapper launches rathole.
type Config struct {
	BundleDir           string
	ConfigPath          string
	ServiceName         string
	RatholePath         string
	StartupGracePeriod  time.Duration
	ShutdownGracePeriod time.Duration
}

// ParseArgs parses CLI flags for the service wrapper.
func ParseArgs(args []string, executableDir string) (Config, error) {
	defaultBundleDir := strings.TrimSpace(executableDir)
	if defaultBundleDir == "" {
		defaultBundleDir = "."
	}

	var (
		bundleDir   = defaultBundleDir
		configPath  string
		ratholePath string
		serviceName = DefaultServiceName
	)

	fs := flag.NewFlagSet("nusatunnel-service", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&bundleDir, "bundle-dir", bundleDir, "Directory containing nusatunnel.exe and client.toml")
	fs.StringVar(&configPath, "config", "", "Path to client.toml")
	fs.StringVar(&serviceName, "service-name", serviceName, "Windows service name for log context")
	fs.StringVar(&ratholePath, "rathole-bin", "", "Path to nusatunnel.exe")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	bundleDir = strings.TrimSpace(bundleDir)
	if bundleDir == "" {
		bundleDir = defaultBundleDir
	}
	if strings.TrimSpace(configPath) == "" {
		configPath = filepath.Join(bundleDir, DefaultConfigName)
	}
	if strings.TrimSpace(ratholePath) == "" {
		ratholePath = filepath.Join(bundleDir, DefaultRatholeBinaryName)
	}

	cfg := Config{
		BundleDir:           bundleDir,
		ConfigPath:          configPath,
		ServiceName:         serviceName,
		RatholePath:         ratholePath,
		StartupGracePeriod:  defaultStartupGracePeriod,
		ShutdownGracePeriod: defaultShutdownGracePeriod,
	}
	return cfg.normalized(), nil
}

func (c Config) normalized() Config {
	out := c
	out.BundleDir = trimWindowsArgQuotes(strings.TrimSpace(out.BundleDir))
	if out.BundleDir == "" {
		out.BundleDir = "."
	}
	out.ConfigPath = trimWindowsArgQuotes(strings.TrimSpace(out.ConfigPath))
	if out.ConfigPath == "" {
		out.ConfigPath = filepath.Join(out.BundleDir, DefaultConfigName)
	}
	out.RatholePath = trimWindowsArgQuotes(strings.TrimSpace(out.RatholePath))
	if out.RatholePath == "" {
		out.RatholePath = filepath.Join(out.BundleDir, DefaultRatholeBinaryName)
	}
	out.ServiceName = strings.TrimSpace(out.ServiceName)
	if out.ServiceName == "" {
		out.ServiceName = DefaultServiceName
	}
	if out.StartupGracePeriod <= 0 {
		out.StartupGracePeriod = defaultStartupGracePeriod
	}
	if out.ShutdownGracePeriod <= 0 {
		out.ShutdownGracePeriod = defaultShutdownGracePeriod
	}
	return out
}

// Validate checks that required sidecar files exist.
func (c Config) Validate() error {
	cfg := c.normalized()
	if !fileExists(cfg.RatholePath) {
		return fmt.Errorf("binary rathole tidak ditemukan: %s", cfg.RatholePath)
	}
	if !fileExists(cfg.ConfigPath) {
		return fmt.Errorf("file config client.toml tidak ditemukan: %s", cfg.ConfigPath)
	}
	return nil
}

// Run starts the bundled rathole process and keeps the wrapper alive for NSSM.
func Run(ctx context.Context, cfg Config, stdout, stderr io.Writer) error {
	cfg = cfg.normalized()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	cmd := exec.Command(cfg.RatholePath, cfg.ConfigPath)
	cmd.Dir = cfg.BundleDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	fmt.Fprintf(stdout, "[service-wrapper] starting %s for %s with config %s\n", filepath.Base(cfg.RatholePath), cfg.ServiceName, cfg.ConfigPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("gagal menjalankan rathole: %w", err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	startupTimer := time.NewTimer(cfg.StartupGracePeriod)
	defer startupTimer.Stop()

	select {
	case err := <-waitCh:
		if err != nil {
			return fmt.Errorf("rathole berhenti terlalu cepat: %w", err)
		}
		return errors.New("rathole berhenti terlalu cepat tanpa error")
	case <-startupTimer.C:
		fmt.Fprintf(stdout, "[service-wrapper] rathole running with pid %d\n", cmd.Process.Pid)
	case <-ctx.Done():
		fmt.Fprintln(stdout, "[service-wrapper] shutdown diminta sebelum startup selesai")
		return shutdownChild(cmd, waitCh, cfg.ShutdownGracePeriod, stdout)
	}

	select {
	case err := <-waitCh:
		if err != nil {
			return fmt.Errorf("rathole berhenti: %w", err)
		}
		return errors.New("rathole berhenti tanpa error")
	case <-ctx.Done():
		fmt.Fprintln(stdout, "[service-wrapper] shutdown diminta, menghentikan rathole")
		return shutdownChild(cmd, waitCh, cfg.ShutdownGracePeriod, stdout)
	}
}

func shutdownChild(cmd *exec.Cmd, waitCh <-chan error, timeout time.Duration, stdout io.Writer) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	_ = cmd.Process.Signal(os.Interrupt)

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-waitCh:
		fmt.Fprintln(stdout, "[service-wrapper] rathole berhenti")
		return nil
	case <-timer.C:
		fmt.Fprintln(stdout, "[service-wrapper] force kill rathole setelah timeout shutdown")
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("gagal force kill rathole: %w", err)
		}
		<-waitCh
		return nil
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func trimWindowsArgQuotes(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return strings.TrimSuffix(strings.TrimPrefix(value, `"`), `"`)
	}
	return value
}
