package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := "9999"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	dir := "."
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}

	http.Handle("/", http.FileServer(http.Dir(dir)))
	fmt.Printf("[+] Sharing %s on http://192.168.133.1:%s\n", dir, port)
	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		fmt.Printf("[-] Error: %v\n", err)
	}
}
