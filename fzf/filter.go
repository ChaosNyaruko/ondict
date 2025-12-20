package fzf

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/sources"
)

func withFilter(command string, input func(in io.WriteCloser)) []string {
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}
	cat := "cat"
	if isCommandAvailable("mdcat") {
		cat = "mdcat"
	} else if isCommandAvailable("bat") {
		cat = "bat --file-name=tmpondicttmp12345.md"
	}

	tmpFileName := "/tmp/ondictfzfoutput12345.html"
	// TODO: "open" according to sysytem, now just for my macOS.
	// TODO: performance, and maybe "auto" mode for easy usage.
	// TODO: the temp html file opened by browser will not automatically find the static resources, e.g. pictures, but I don't know how to replace the whitespaces in fzf placeholder {}, which will cause failure when constructing the HTTP GET request.
	bind := fmt.Sprintf(`--bind="enter:execute(ondict -q {} -r 1 -remote localhost:1345 -e mdx -f html> %s && open %s)"`, tmpFileName, tmpFileName)
	preview := fmt.Sprintf(`--preview="ondict -q {} -remote auto -f md -e mdx" | %s`, cat)
	header := "--header='[press ENTER to display the result in your browser, which might have a fancier display]'"

	command = strings.Join([]string{command, header, bind, preview}, " ")
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
	if !isCommandAvailable("fzf") {
		log.Fatal("Cannot find fzf executable, check https://github.com/junegunn/fzf?tab=readme-ov-file#installation to install")
	}
	// log.SetOutput(io.Discard)
	_ = withFilter("fzf ", func(in io.WriteCloser) {
		for _, g := range *sources.G {
			for _, k := range g.MdxDict.Keys() {
				// TODO: tell where the word is from
				// fmt.Fprintln(in, fmt.Sprintf("%s in[%s]", k, g.MdxFile))
				fmt.Fprintln(in, fmt.Sprintf("%s", k))
			}
		}
	})
	// fmt.Println(filtered)
}

func isCommandAvailable(name string) bool {
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}
	cmd := exec.Command(shell, "-c", "command -v "+name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
