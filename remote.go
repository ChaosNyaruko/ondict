package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// autoNetworkAddressPosix resolves an id on the 'auto' pseduo-network to a
// real network and address. On unix, this uses unix domain sockets.
// copied from x/gopls
func autoNetworkAddressPosix(goplsPath, id string) (network string, address string) {
	// Especially when doing local development or testing, it's important that
	// the remote gopls instance we connect to is running the same binary as our
	// forwarder. So we encode a short hash of the binary path into the daemon
	// socket name. If possible, we also include the buildid in this hash, to
	// account for long-running processes where the binary has been subsequently
	// rebuilt.
	h := sha256.New()
	cmd := exec.Command("go", "tool", "buildid", goplsPath)
	cmd.Stdout = h
	var pathHash []byte
	if err := cmd.Run(); err == nil {
		pathHash = h.Sum(nil)
	} else {
		log.Debugf("error getting current buildid: %v", err)
		sum := sha256.Sum256([]byte(goplsPath))
		pathHash = sum[:]
	}
	shortHash := fmt.Sprintf("%x", pathHash)[:6]
	user := os.Getenv("USER")
	if user == "" {
		user = "shared"
	}
	basename := filepath.Base(goplsPath)
	idComponent := ""
	if id != "" {
		idComponent = "-" + id
	}
	runtimeDir := os.TempDir()
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		runtimeDir = xdg
	}
	return "unix", filepath.Join(runtimeDir, fmt.Sprintf("%s-%s-daemon.%s%s", basename, shortHash, user, idComponent))
}

func startRemote(dp string, args ...string) error {
	cmd := exec.Command(dp, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("startRemote server err: %v", err)
	}
	// return cmd.Wait()
	return nil
}

func request(netConn net.Conn, e, f string) error {
	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return netConn, nil
			},
		},
	}
	res, err := httpc.Get(fmt.Sprintf("http://fakedomain/dict?query=%s&engine=%s&format=%s", url.QueryEscape(*word), e, f))
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Fatalf("new request error %v", err)
	}
	defer res.Body.Close()
	if res, err := io.ReadAll(res.Body); err != nil {
		return fmt.Errorf("read body error %v", err)
	} else {
		fmt.Println(string(res))

	}

	return nil
}
