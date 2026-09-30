package gates

import "sort"

func SortIssueFolders(names []string) {
	sort.SliceStable(names, func(i, j int) bool {
		ni, oki := gateIssueFolderNumber(names[i])
		nj, okj := gateIssueFolderNumber(names[j])
		switch {
		case oki && okj:
			return ni < nj
		case oki != okj:
			return oki
		default:
			return names[i] < names[j]
		}
	})
}
func gateIssueFolderNumber(name string) (int, bool) {
	if name == "" {
		return 0, false
	}
	n := 0
	for _, r := range name {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}
