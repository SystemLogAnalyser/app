package main

import (
	"bufio"
	"io"
	"regexp"
	"sort"
	"time"
)

func parseLog(s string) (log Log, ok bool) {
	r := regexp.MustCompile(`^(\w+? \d+? \d+?:\d+?:\d+?) .*? (.*?): (.*)$`)
	match := r.FindStringSubmatch(s)

	if len(match) == 4 {
		parsedTime, err := time.Parse("Jan 02 15:04:05", match[1])
		if err != nil {
			return Log{}, false
		}
		return Log{
			timestamp: parsedTime,
			process:   match[2],
			message:   match[3],
		}, true
	}
	return Log{}, false
}

type Freq struct {
	s     string
	count int
}

func getTopNProcess(logs []Log, n int) []Freq {
	mp := make(map[string]int)
	for _, log := range logs {
		mp[log.process] += 1
	}
	var freqList []Freq
	for process, freq := range mp {
		freqList = append(freqList, Freq{s: process, count: freq})
	}

	sort.Slice(freqList, func(i, j int) bool {
		return freqList[i].count > freqList[j].count
	})

	if n > len(freqList) {
		n = len(freqList) // Handle case where n > number of unique processes
	}
	return freqList[:n]
}

func getTrendValues(logs []Log, n int) []int {
	earliest := logs[0].timestamp
	latest := logs[0].timestamp
	for _, log := range logs {
		if log.timestamp.Before(earliest) {
			earliest = log.timestamp
		}
		if log.timestamp.After(latest) {
			latest = log.timestamp
		}
	}

	totalDuration := latest.Sub(earliest)
	sectionDuration := totalDuration / time.Duration(n)

	sections := make([]int, n)

	for _, log := range logs {
		offset := log.timestamp.Sub(earliest)
		sectionIndex := int(offset / sectionDuration)
		if sectionIndex >= n {
			sectionIndex = n - 1
		}
		sections[sectionIndex]++
	}

	return sections
}

func checkEmptyFilter(filters []Filter) bool {
	flag := true
	for _, filter := range filters {
		if len(filter.message) > 0 ||
			len(filter.process) > 0 ||
			filter.endTime != "" ||
			filter.startTime != "" ||
			filter.category != "" {
			return false
		}
	}
	return flag
}

func getFilteredLogs(logs []Log, filters []Filter) []Log {
	var res []Log
	if checkEmptyFilter(filters) {
		return logs
	}
	for _, log := range logs {
		flag := true
		for _, filter := range filters {
			for _, r := range filter.message {
				if !r.MatchString(log.message) {
					flag = false
					break
				}
			}
			if !flag {
				break
			}
			for _, r := range filter.process {
				if !r.MatchString(log.process) {
					flag = false
					break
				}
			}
			if !flag {
				break
			}
			if filter.startTime != "" && filter.startTime > log.timestamp.Format("Jan 02 15:04:05") {
				flag = false
			}
			if !flag {
				break
			}
			if filter.endTime != "" && filter.endTime < log.timestamp.Format("Jan 02 15:04:05") {
				flag = false
			}
			if !flag {
				break
			}
			if filter.category != "" && filter.category != log.category {
				flag = false
			}
			if !flag {
				break
			}
		}

		if flag {
			res = append(res, log)
		}
	}
	return res
}

var error_filter = Filter{
	name: "error",
	message: []*regexp.Regexp{
		regexp.MustCompile(`error`),
		regexp.MustCompile(`err`),
		regexp.MustCompile(`fatal`),
		regexp.MustCompile(`memory`),
		regexp.MustCompile(`alert`),
		regexp.MustCompile(`fail`),
		regexp.MustCompile(`kill`),
		regexp.MustCompile(`abnormally`),
		regexp.MustCompile(`stack trace`),
		regexp.MustCompile(`dumped core`),
		regexp.MustCompile(`<0>`),
		regexp.MustCompile(`<1>`),
		regexp.MustCompile(`<2>`),
		regexp.MustCompile(`<3>`),
	},
}

var warn_filter = Filter{
	name: "warn",
	message: []*regexp.Regexp{
		regexp.MustCompile(`warn`),
		regexp.MustCompile(`wrn`),
		regexp.MustCompile(`caution`),
		regexp.MustCompile(`deprecated`),
		regexp.MustCompile(`unhandled`),
		regexp.MustCompile(`unknown`),
		regexp.MustCompile(`<4>`),
	},
}

var info_filter = Filter{
	name: "info",
	message: []*regexp.Regexp{
		regexp.MustCompile(`info`),
		regexp.MustCompile(`inf`),
		regexp.MustCompile(`notice`),
		regexp.MustCompile(`success`),
		regexp.MustCompile(`completed`),
		regexp.MustCompile(`finished`),
		regexp.MustCompile(`notice`),
		regexp.MustCompile(`started`),
		regexp.MustCompile(`stopped`),
		regexp.MustCompile(`reached`),
		regexp.MustCompile(`starting`),
		regexp.MustCompile(`connection`),
		regexp.MustCompile(`supervising`),
		regexp.MustCompile(`<5>`),
		regexp.MustCompile(`<6>`),
	},
}

func GetParsedLogs(reader io.Reader) []Log {
	scanner := bufio.NewScanner(reader)
	var logs []Log
	for scanner.Scan() {
		txt := scanner.Text()
		log, ok := parseLog(txt)
		if ok != true {
			continue
		}
		logs = append(logs, log)
	}
	return logs
}

func GetCategorisedLogs(logs []Log) []Log {
	res := logs
	for i := range res {
		flag := false
		for _, filter := range []Filter{error_filter, warn_filter, info_filter} {
			for _, r := range filter.message {
				if r.MatchString(res[i].message) {
					res[i].category = filter.name
					flag = true
					break
				}
			}
			if flag {
				break
			}
		}
		if !flag {
			res[i].category = "misc"
		}
	}
	return res
}
