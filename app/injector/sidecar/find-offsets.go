// Helper tool to find http.Request struct field offsets
// This helps us know where to read Method and RequestURI in memory
package main

import (
	"fmt"
	"net/http"
	"runtime"
	"unsafe"
)

func main() {
	req := &http.Request{}
	
	fmt.Println("=== http.Request struct offsets for Go", runtime.Version(), "===")
	fmt.Printf("Method offset:     %d bytes\n", unsafe.Offsetof(req.Method))
	fmt.Printf("URL offset:        %d bytes\n", unsafe.Offsetof(req.URL))
	fmt.Printf("Proto offset:      %d bytes\n", unsafe.Offsetof(req.Proto))
	fmt.Printf("Header offset:     %d bytes\n", unsafe.Offsetof(req.Header))
	fmt.Printf("Body offset:       %d bytes\n", unsafe.Offsetof(req.Body))
	fmt.Printf("RequestURI offset: %d bytes\n", unsafe.Offsetof(req.RequestURI))
	fmt.Printf("Host offset:       %d bytes\n", unsafe.Offsetof(req.Host))
	
	fmt.Println("\n=== Go string representation ===")
	fmt.Printf("String size: %d bytes (pointer + length)\n", unsafe.Sizeof(""))
	fmt.Printf("Pointer size: %d bytes\n", unsafe.Sizeof(uintptr(0)))
}
