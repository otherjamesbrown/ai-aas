package main

import (
	"fmt"
	"os"

	"github.com/otherjamesbrown/ai-aas/services/hello-service/pkg/hello"
)

func main() {
	name := ""
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	fmt.Println(hello.Greeting(name))
}
