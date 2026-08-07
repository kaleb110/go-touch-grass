package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// systemdUnitTemplate is the template for the generated user unit file.
// The binary path is filled in at install time.
const systemdUnitTemplate = `[Unit]
Description=Track daily machine usage time
After=default.target

[Service]
Type=simple
ExecStart=%s tracker -update
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
`

// InstallCMD implements the `install` subcommand.
// It copies the running binary to ~/.local/bin, writes a systemd user unit
// to ~/.config/systemd/user, and reloads+enables the service so tracking
// starts automatically on login.
type InstallCMD struct {
	name   string
	force  bool
	dryRun bool
}

// DefaultAlias is the binary name used when -as is not supplied.
const DefaultAlias = "go-touch-grass"

func NewInstall() *InstallCMD { return &InstallCMD{name: DefaultAlias} }

func (ic *InstallCMD) Name() string        { return "install" }
func (ic *InstallCMD) Description() string { return "install the binary and systemd user service" }

func (ic *InstallCMD) Init(args []string) error {
	fs := flag.NewFlagSet(ic.Name(), flag.ExitOnError)
	fs.StringVar(&ic.name, "as", DefaultAlias, "name to install the binary and service as (alias)")
	fs.BoolVar(&ic.force, "force", false, "overwrite existing install")
	fs.BoolVar(&ic.dryRun, "dry-run", false, "print actions without making changes")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go-touch-grass %s [flags]\n\nFlags:\n", ic.Name())
		fs.PrintDefaults()
	}
	return fs.Parse(args)
}

func (ic *InstallCMD) Run() error {
	binDir, binPath, unitPath, err := installPaths(ic.name)
	if err != nil {
		return err
	}

	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating running binary: %w", err)
	}

	fmt.Printf("Source binary: %s\n", src)
	fmt.Printf("Install target: %s\n", binPath)
	fmt.Printf("Systemd unit:   %s\n", unitPath)
	if ic.dryRun {
		fmt.Println("\n(dry-run: no changes made)")
		return nil
	}

	if !ic.force {
		if _, err := os.Stat(binPath); err == nil {
			return fmt.Errorf("binary already exists at %s (use -force to overwrite)", binPath)
		}
	}

	if err := mkdirAll(binDir); err != nil {
		return fmt.Errorf("creating bin dir: %w", err)
	}
	if err := copyFile(src, binPath); err != nil {
		return fmt.Errorf("copying binary: %w", err)
	}
	if err := os.Chmod(binPath, 0755); err != nil {
		return fmt.Errorf("chmod binary: %w", err)
	}
	fmt.Println("✓ Binary installed")

	if err := mkdirAll(filepath.Dir(unitPath)); err != nil {
		return fmt.Errorf("creating systemd dir: %w", err)
	}
	unit := fmt.Sprintf(systemdUnitTemplate, binPath)
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}
	fmt.Println("✓ Systemd unit written")

	if err := systemctlReload(); err != 0 {
		fmt.Printf("Warning: systemctl daemon-reload failed (exit %d). You may need to run it manually.\n", err)
	} else {
		fmt.Println("✓ systemd daemon reloaded")
	}

	if err := systemctlEnable(ic.name); err != 0 {
		fmt.Printf("Warning: systemctl enable failed (exit %d). You may need to run it manually.\n", err)
	} else {
		fmt.Println("✓ Service enabled (will start on login)")
	}

	fmt.Println("\nInstallation complete.")
	fmt.Printf("Start tracking now with:  systemctl --user start %s\n", ic.name)
	fmt.Printf("View live logs with:      journalctl --user -u %s -f\n", ic.name)
	if ic.name != DefaultAlias {
		fmt.Printf("\nInstalled as '%s'. Run '%s tracker' to use it.\n", ic.name, ic.name)
	}
	return nil
}

// --- helpers ---

func mkdirAll(path string) error { return os.MkdirAll(path, 0755) }

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func systemctlReload() int {
	cmd := exec.Command("systemctl", "--user", "daemon-reload")
	return runOrWarn(cmd)
}

func systemctlEnable(unitName string) int {
	cmd := exec.Command("systemctl", "--user", "enable", unitName+".service")
	return runOrWarn(cmd)
}

func runOrWarn(cmd *exec.Cmd) int {
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}


