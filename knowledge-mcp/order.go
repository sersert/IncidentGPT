package main

import "sort"

func sortSearch(result *SearchOutput) {
	sort.Slice(result.Sources, func(i, j int) bool { return result.Sources[i].SourceID < result.Sources[j].SourceID })
	sort.SliceStable(result.Documents, func(i, j int) bool { return result.Documents[i].SourceID < result.Documents[j].SourceID })
}
