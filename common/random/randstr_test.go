package random

import (
	"fmt"
)

func ExampleHex() {
	for i := 0; i < 5; i++ {
		token := Hex(16)
		fmt.Println(token)
	}
}

func ExampleString() {
	for i := 0; i < 5; i++ {
		token := String(16)
		fmt.Println(token)
	}
}
