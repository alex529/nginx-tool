package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run main.go <nginx_conf_path> <service> <port>")
		return
	}
	filePath := os.Args[1]
	service := os.Args[2]
	port := os.Args[3]

	if err := exec(filePath, service, port); err != nil {
		log.Fatal(err)
	}
}

func exec(confPath, service, portStr string) error {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(confPath, os.O_RDWR, 0755)
	if err != nil {
		return err
	}

	services := make(map[string]int)
	scanner := bufio.NewScanner(file)
	isRouteRegion := false
	var buf bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "#routes_start") {
			if err := writeLine(&buf, line); err != nil {
				return err
			}
			isRouteRegion = true
			continue
		}
		if strings.Contains(line, "#routes_end") {
			isRouteRegion = false
			services[service] = port

			if err := insertRoutes(services, &buf); err != nil {
				return err
			}
		}
		if !isRouteRegion {
			if err := writeLine(&buf, line); err != nil {
				return err
			}
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line[0] == '#' {
			continue
		}

		serv, p, err := getService(line)
		if err != nil {
			return err
		}
		services[serv] = p
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if err := file.Truncate(0); err != nil {
		return err
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = file.Write(buf.Bytes())
	return err
}

func writeLine(buf *bytes.Buffer, line string) error {
	if _, err := buf.WriteString(line); err != nil {
		return err
	}
	_, err := buf.WriteRune('\n')
	return err
}

func insertRoutes(services map[string]int, buf *bytes.Buffer) error {
	for s, p := range services {
		if err := writeLine(buf, genRoute(s, p)); err != nil {
			return err
		}
	}
	return nil
}

func genRoute(container string, port int) string {
	return fmt.Sprintf("    location /%s/ {rewrite /%s/(.*) /$1 break; proxy_pass http://%s:%d;} #%s_route",
		container,
		container,
		container,
		port,
		container,
	)
}

var re = regexp.MustCompile(`.*http://(\w+):(\d+);.*`)

func getService(line string) (string, int, error) {
	match := re.FindStringSubmatch(line)
	if len(match) != 3 {
		return "", 0, fmt.Errorf("route didn't match regex\nline: %s", line)
	}

	port, err := strconv.Atoi(match[2])
	return match[1], port, err
}
