package utils

import (
	"log"
	"os"

	"github.com/eiannone/keyboard"
)

func PauseExit() {
	log.Println("Press any to exit")
	keyboard.GetSingleKey()
	os.Exit(0)
}
