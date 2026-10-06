// Command bridgehost serves the mobile bridge over standard input and output,
// one JSON request per line in and one JSON response per line out, through the
// same two functions the XCFramework exports. It is how a desktop tool or a
// verification script drives the exact protocol the iPhone app speaks without
// a simulator.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/sglavoie/tt/mobile/ttbridge"
)

// maxRequestBytes bounds one request line.
const maxRequestBytes = 16 << 20

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(ttbridge.BridgeVersion())
		return
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: bridgehost [--version] < requests.jsonl")
		os.Exit(2)
	}
	if err := serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "bridgehost:", err)
		os.Exit(1)
	}
}

// serve answers each nonblank request line with one response line, flushing
// after each so a caller can converse with it.
func serve(in io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64<<10), maxRequestBytes)
	out := bufio.NewWriter(w)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		if _, err := fmt.Fprintln(out, ttbridge.Call(scanner.Text())); err != nil {
			return err
		}
		if err := out.Flush(); err != nil {
			return err
		}
	}
	return scanner.Err()
}
