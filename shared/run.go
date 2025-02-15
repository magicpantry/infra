package shared

import (
	"log"
	"os/exec"
)

func Run(cmd string) string {
	log.Println(cmd)
	x := exec.Command("bash", "-c", cmd)
	output, err := x.CombinedOutput()
	if string(output) != "" {
		log.Println(string(output))
	}
	if err != nil {
		log.Fatal(err)
	}
	return string(output)
}
