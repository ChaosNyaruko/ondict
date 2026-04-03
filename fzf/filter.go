package fzf

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
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

	openCmd := browserOpenCommand()
	// Open the server URL directly in the browser, so static resources (images etc.) are served properly.
	// Use sed to URL-encode spaces in the fzf placeholder {}.
	bind := fmt.Sprintf(`--bind="enter:execute-silent(%s 'http://localhost:1345/dict?query='$(echo {} | sed 's/ /%%20/g')'&engine=mdx&format=html')"`, openCmd)
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

func browserOpenCommand() string {
	switch runtime.GOOS {
	case "darwin":
		return "open"
	case "windows":
		return "start"
	default:
		return "xdg-open"
	}
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
