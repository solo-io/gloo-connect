package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Simple static webserver:
	fmt.Println("Example server running!")
	log.Fatal(http.ListenAndServe(":8080", http.FileServer(http.Dir("/usr/share/doc"))))
}
