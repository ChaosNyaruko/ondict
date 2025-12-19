package main

import (
	"fmt"
	"os"
	"os/exec"
)

var (
	startRemote        = startRemoteDefault
	autoNetworkAddress = autoNetworkAddressDefault
)

func autoNetworkAddressDefault(goplsPath, id string) (network string, address string) {
	if id != "" {
		panic("identified remotes are not supported on windows")
	}
	return "tcp", "localhost:37374"
}

func startRemoteDefault(dp string, env []string, args ...string) error {
	cmd := exec.Command(dp, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("startRemote server err: %v", err)
	}
	// return cmd.Wait()
	return nil
}
