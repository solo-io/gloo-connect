package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Simple static webserver:
	fmt.Println("Example server running!")
	log.Fatal(http.ListenAndServe(":8080", wrap(http.FileServer(http.Dir("/usr/share/doc")))))
}

func wrap(h http.Handler) http.Handler {
	i := 0
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		i++
		if i%2 == 1 {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		h.ServeHTTP(rw, r)
	})
}
