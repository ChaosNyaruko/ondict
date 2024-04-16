package fzf

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/ChaosNyaruko/ondict/sources"
)

func withFilter(command string, input func(in io.WriteCloser)) []string {
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}
	command = strings.Join([]string{command, "--bind=\"enter:execute(ondict -q {} -remote localhost:1345 -e mdx -f html > /tmp/ondictfzfoutput1345.html && open /tmp/ondictfzfoutput1345.html)\"", "--preview=\"ondict -q {} -remote localhost:1345 -f=md -e=mdx\" | bat --file-name=tmpondicttmp12345.md"}, " ")
	cmd := exec.Command(shell, "-c", command)
	cmd.Stderr = os.Stderr
	in, _ := cmd.StdinPipe()
	go func() {
		input(in)
		in.Close()
	}()
	result, _ := cmd.Output()
	return strings.Split(string(result), "\n")
}

func ListAllWord() {
	log.SetOutput(io.Discard)
	_ = withFilter("fzf ", func(in io.WriteCloser) {
		for _, g := range *sources.G {
			for _, k := range g.MdxDict.Keys() {
				fmt.Fprintln(in, k)
			}
		}
	})
	// fmt.Println(filtered)
}
