package composer

import (
	"fmt"
	"os"
	"os/exec"
)

func FindBinary() (string, error) {
	if path, err := exec.LookPath("composer"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("composer.phar"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("composer not found — install from https://getcomposer.org")
}

func Run(dir string, args ...string) error {
	bin, err := FindBinary()
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func Install(dir string) error {
	return Run(dir, "install")
}

func Require(dir string, pkg string) error {
	return Run(dir, "require", pkg)
}

func Remove(dir string, pkg string) error {
	return Run(dir, "remove", pkg)
}
