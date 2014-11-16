// /proc/$PID/
//     → stat
//          pname, PID
//     → net/tcp | net/tcp6
//          0A == LISTEN
//          port, inode
//     → fd/3
//          symlink ?
//          socket:[inode]
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

var (
	progs     []string
	ipver4    bool
	ipver6    bool
	ipver     string
	tolerance bool
	help      bool
)

const LISTEN = "0A"

const (
	sl = iota
	local_address
	remote_address
	st
	txqueue_rxqueue // tx_queue tx_queue
	tr_tm_when      // tr tm->when
	retrnsmt
	uid
	timeout
	inode
	// and seven unnamed fields...
)

type PIDPORT struct {
	PID  string
	Port string
}

func isPosInt(a string) bool {
	for _, i := range a {
		switch i {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		default:
			return false
		}
	}
	return true
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s -[46ht] progname1 progname2 .. prognameN\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Output format: Progname PID port")
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	os.Exit(1)
}

func errOut(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a)
	if !tolerance {
		os.Exit(1)
	}
}

func init() {
	flag.BoolVar(&ipver4, "4", false, "use IPv4")
	flag.BoolVar(&ipver6, "6", true, "use IPv6")
	flag.BoolVar(&tolerance, "t", false, "if set, the program will continue till the end despite errors")
	flag.BoolVar(&help, "h", false, "display help text")
	flag.BoolVar(&help, "help", false, "display help text")
	flag.BoolVar(&help, "-help", false, "display help text")
	flag.Parse()

	if len(flag.Args()) == 0 || help {
		usage()
	}

	progs = flag.Args()
	switch {
	case ipver4:
		ipver = ""
	case ipver6:
		ipver = "6"
	}
}

// Gather all file descriptors present and associate program name with its fd.
func readAllProc() map[string]PIDPORT {
	proc, err := os.Open("/proc/")
	if err != nil {
		errOut("Opening /proc failed with error:", err)
	}
	defer proc.Close()

	names, err := proc.Readdirnames(0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	searches := map[string]PIDPORT{}

	for _, n := range names {
		if isPosInt(n) {
			cnt, err := ioutil.ReadFile("/proc/" + n + "/stat")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			spl := strings.Split(string(cnt), " ")
			pname := string(spl[1][1 : len(spl[1])-1])
			searches[pname] = PIDPORT{PID: n}
		}
	}

	return searches
}

func main() {
	searches := readAllProc()

	for _, prog := range progs {
		cnt, err := ioutil.ReadFile("/proc/" + searches[prog].PID + "/net/tcp" + ipver)
		if err != nil {
			errOut("Failed to read net/tcp with error:", err)
		}

		// Imagine how simple and easy this part is with Awk.
		lines := strings.Split(string(cnt), "\n")
		var records [][]string
		for _, l := range lines {
			rds := strings.Fields(l)
			records = append(records, rds)
		}

		var ports []string
		var inodes []string
		for i := 1; i < len(records)-1; i++ {
			r := records[i]
			if r[st] == LISTEN {
				rl := r[local_address]
				p := rl[strings.Index(rl, ":")+1:]
				ports = append(ports, p)
				inodes = append(inodes, r[inode])
			}
		}

		fd3 := "/proc/" + searches[prog].PID + "/fd/3"
		sym, err := os.Readlink(fd3)
		if err != nil {
			errOut("Failed to read symlink of fd/3 with error:", err)
		} else {
			for i, in := range inodes {
				si := strings.Index(sym, "[")
				sin := (sym[:len(sym)-1])[si+1:] // Some dark slicing magick.
				if in == sin {
					// It pretty much is a positive integer at this point...
					d, _ := strconv.ParseInt(ports[i], 16, 32)
					ps := searches[prog]
					ps.Port = strconv.Itoa(int(d))
					searches[prog] = ps
				}
			}
		}

		fmt.Println(prog, searches[prog].PID, searches[prog].Port)
	}
}
