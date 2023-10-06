package pcap

import (
	"log"
	"regexp"
)

var flagRegex *regexp.Regexp

func EnsureRegex(reg string) {
	if flagRegex == nil {
		reg, err := regexp.Compile(reg)
		if err != nil {
			log.Fatal("Failed to compile flag regex: ", err)
		} else {
			flagRegex = reg
		}
	}
}

func containsTag(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func ApplyFlagTags(flow *FlowEntry, reg string) {
	EnsureRegex(reg)
	if flagRegex == nil {
		return
	}
	for idx := 0; idx < len(flow.Flow); idx++ {
		flowItem := &flow.Flow[idx]
		if flagRegex.MatchString(flowItem.Data) {
			var tag string
			if flowItem.From == "c" {
				tag = "flag-in"
			} else {
				tag = "flag-out"
			}
			if !containsTag(flow.Tags, tag) {
				flow.Tags = append(flow.Tags, tag)
			}
		}
	}
}
