package gates

import "sort"

// SortIssueFolders 함수는 이슈 폴더 이름을 제자리에서 정렬한다.
// 숫자로만 된 이름은 숫자 크기순으로 앞에 두고, 나머지는 그 뒤에 사전순으로 둔다.
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
