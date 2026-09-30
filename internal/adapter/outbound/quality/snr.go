package quality

import (
	"bufio"
	"errors"
	contract "issueops/internal/contract/quality"
	policy "issueops/internal/domain/quality"
	"math"
	"os"
	"strings"
)

func ComputeCodeSNR(root string) (contract.SNRResult, error) {
	var signal, noise int
	paths, scanErrors := productionGoFiles(root)
	if err := errors.Join(scanErrors...); err != nil {
		return contract.SNRResult{}, err
	}
	for _, path := range paths {
		s, n, err := snrCountFile(path)
		if err != nil {
			return contract.SNRResult{}, err
		}
		signal += s
		noise += n
	}
	total := signal + noise
	ratio := 0.0
	if total > 0 {
		ratio = math.Round(float64(signal)/float64(total)*10000) / 10000
	}
	return contract.SNRResult{SignalLines: signal, NoiseLines: noise, TotalLines: total, Ratio: ratio}, nil
}

func snrCountFile(path string) (signal, noise int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	inBlockComment := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case inBlockComment:
			noise++
			if strings.Contains(line, "*/") {
				inBlockComment = false
			}
		case line == "":
			noise++
		case strings.HasPrefix(line, "//"):
			noise++
		case strings.HasPrefix(line, "/*"):
			noise++
			if !strings.Contains(line, "*/") {
				inBlockComment = true
			}
		case policy.SNRStructuralOnly(line):
			noise++
		default:
			signal++
		}
	}
	if err := sc.Err(); err != nil {
		return 0, 0, err
	}
	return signal, noise, nil
}
