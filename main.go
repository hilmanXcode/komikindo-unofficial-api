package main

import (
	"komikindo-scraper/bootstrap"
)

func main() {

	// stop := make(chan struct{})

	// go func() {
	// 	ticker := time.NewTicker(2 * time.Second) // Set your interval here
	// 	defer ticker.Stop()

	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			log.Println("Hello World")
	// 		case <-stop:
	// 			fmt.Println("Stopping worker...")
	// 			return
	// 		}
	// 	}
	// }()

	bootstrap.BootstrapApp()

}
