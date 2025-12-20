package main

import (
	"fmt"
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
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("startRemote server err: %v", err)
	}
	// return cmd.Wait()
	return nil
}
