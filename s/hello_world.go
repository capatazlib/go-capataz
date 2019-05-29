package s

import (
	"fmt"
)

// Hello is a function used to test the coverage pipeline
func Hello(name string) string {
	return fmt.Sprintf("Hello %s\n", name)
}
