package main

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

type Log struct {
	timestamp string
	process   string
	message   string
}

const (
	cat_error = iota
	cat_warning
	cat_info
	cat_misc
)

func parseLog(s string) (log Log, ok bool) {
	r := regexp.MustCompile(`^(\w+? \d+? \d+?:\d+?:\d+?) .*? (.*?): (.*)$`)
	match := r.FindStringSubmatch(s)

	if len(match) == 4 {
		return Log{
			timestamp: match[1],
			process: match[2],
			message:   match[3],
		}, true
	}
	return Log{}, false
}

var error_flags = []string{
	"error",
	"fatal",
	"memory",
	"alert",
	"fail",
	"kill",
	"abnormally",
	"stack trace",
	"dumped core",
	"<0>",
	"<1>",
	"<2>",
	"<3>",
}

var warn_flags = []string{
	"warn",
	"caution",
	"deprecated",
	"unhandled",
	"unknown",
	"<4>",
}

var info_flags = []string{
	"info",
	"notice",
	"success",
	"completed",
	"finished",
	"notice",
	"started",
	"stopped",
	"reached",
	"starting",
	"connection",
	"supervising",
	"<5>",
	"<6>",
}

func categorise(log Log) uint {
	msg := strings.ToLower(log.message)
	for _, flag := range error_flags {
		if strings.Contains(msg, flag) {
			return cat_error
		}
	}
	for _, flag := range warn_flags {
		if strings.Contains(msg, flag) {
			return cat_warning
		}
	}
	for _, flag := range info_flags {
		if strings.Contains(msg, flag) {
			return cat_info
		}
	}
	return cat_misc
}

func GetCategorisedLogs(reader io.Reader) map[string][]Log{

	// logFile, err := os.Open(os.Args[1])
	// if err != nil {
	// 	print("could not open file", os.Args[1])
	// }
	// logFile, _ := os.Open("./Linux_2k.log")
	// logFile, _ := os.Open("./synthetic_system.log")
	// defer logFile.Close()
	scanner := bufio.NewScanner(reader)
	var errors []Log
	var warnings []Log
	var infos []Log
	var misc_logs []Log
	for scanner.Scan() {
		txt := scanner.Text()
		log, ok := parseLog(txt)
		if ok != true {
			continue
		}
		// println(log.process, log.message)
		category := categorise(log)
		switch category {
		case cat_error:
			errors = append(errors, log)
		case cat_warning:
			warnings = append(warnings, log)
		case cat_info:
			infos = append(infos, log)
		case cat_misc:
			misc_logs = append(misc_logs, log)
		}
	}
	mp := make(map[string][]Log)
	mp["errors"] = errors
	mp["warnings"] = warnings
	mp["infos"] = infos;
	mp["misc"] = misc_logs
	return mp
}
